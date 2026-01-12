#!/usr/bin/env python3
"""
Codebase Indexer - Multi-Index Architecture
Fast symbol lookup with isolated local and knowledge repo indexes.

Architecture:
  - LOCAL index: Your project code (always active, default)
  - REPO indexes: Knowledge repos (only queried explicitly)

Usage:
    CODEBASE_ROOT=/path/to/code REPOS_ROOT=/path/to/repos python indexer.py
"""

import os
import re
import json
import time
import hashlib
import threading
import subprocess
from pathlib import Path
from typing import Dict, List, Set, Optional, Tuple
from dataclasses import dataclass, asdict
from collections import defaultdict

from flask import Flask, request, jsonify
from watchdog.observers import Observer
from watchdog.events import FileSystemEventHandler

# Tree-sitter for code outlines (165+ languages via language-pack)
try:
    from tree_sitter import Parser, Query, QueryCursor
    from tree_sitter_language_pack import get_language, get_parser
    TREE_SITTER_AVAILABLE = True

    def get_ts_language(lang_name: str):
        """Get a tree-sitter language by name. Supports 165+ languages."""
        # Map common aliases to tree-sitter-language-pack names
        LANG_ALIASES = {
            'typescript': 'typescript',
            'javascript': 'javascript',
            'python': 'python',
            'go': 'go',
            'rust': 'rust',
            'java': 'java',
            'c': 'c',
            'cpp': 'cpp',
            'ruby': 'ruby',
            'php': 'php',
            'c_sharp': 'c_sharp',
            'csharp': 'c_sharp',
            'swift': 'swift',
            'kotlin': 'kotlin',
            'scala': 'scala',
            'bash': 'bash',
            'shell': 'bash',
            'html': 'html',
            'css': 'css',
            'json': 'json',
            'yaml': 'yaml',
            'toml': 'toml',
            'sql': 'sql',
            'lua': 'lua',
            'elixir': 'elixir',
            'haskell': 'haskell',
            'ocaml': 'ocaml',
            'r': 'r',
            'julia': 'julia',
            'dart': 'dart',
            'zig': 'zig',
            'nim': 'nim',
            'perl': 'perl',
            'markdown': 'markdown',
            'vue': 'vue',
            'svelte': 'svelte',
        }
        try:
            name = LANG_ALIASES.get(lang_name, lang_name)
            return get_language(name)
        except Exception:
            return None

except ImportError:
    TREE_SITTER_AVAILABLE = False
    get_ts_language = lambda x: None

# Ranking module for predictive file scoring
import sys
sys.path.insert(0, '/app')  # For Docker
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))  # For local
try:
    from ranking import Scorer, WeightTuner
    RANKING_AVAILABLE = True
except ImportError:
    RANKING_AVAILABLE = False
    Scorer = None
    WeightTuner = None

app = Flask(__name__)

# ============================================================================
# Data Structures
# ============================================================================

@dataclass
class Location:
    file: str
    line: int
    col: int
    symbol_type: str
    mtime: int
    # Symbol-level metadata for semantic compression tags
    symbol: Optional[str] = None      # Symbol/function name (e.g., "handleAuth")
    symbol_kind: Optional[str] = None # Kind (e.g., "function", "class")
    end_line: Optional[int] = None    # Where the symbol ends

@dataclass
class FileMeta:
    path: str
    mtime: int
    size: int
    language: str
    content_hash: str

@dataclass
class ChangeRecord:
    file: str
    timestamp: int
    change_type: str  # added, modified, deleted
    lines_changed: Optional[List[int]] = None


@dataclass
class IntentRecord:
    """Record of an intent capture from tool usage."""
    timestamp: int
    session_id: str
    tool: str
    files: List[str]
    tags: List[str]
    tool_use_id: Optional[str] = None  # Claude's toolu_xxx correlation key
    project_id: Optional[str] = None  # UUID for per-project isolation
    file_sizes: Optional[Dict[str, int]] = None  # File path -> size in bytes (for baseline calc)
    output_size: Optional[int] = None  # Actual output size in bytes (for real savings calc)


@dataclass
class OutlineSymbol:
    """A symbol extracted from code outline."""
    name: str
    kind: str  # function, class, method
    start_line: int
    end_line: int
    signature: Optional[str] = None
    children: Optional[List['OutlineSymbol']] = None
    tags: Optional[List[str]] = None  # AI-generated intent tags (via enrichment)


class OutlineParser:
    """Extract code structure using tree-sitter."""

    # Map file extensions to tree-sitter language names (165+ supported)
    LANG_MAP = {
        # Tier 1: Core languages with full symbol extraction
        'python': 'python',
        'typescript': 'typescript',
        'javascript': 'javascript',
        'go': 'go',
        'rust': 'rust',
        'java': 'java',
        'c': 'c',
        'cpp': 'cpp',
        'ruby': 'ruby',
        'php': 'php',
        'swift': 'swift',
        'kotlin': 'kotlin',
        'scala': 'scala',
        'csharp': 'c_sharp',
        # Tier 2: Additional languages with outline support
        'bash': 'bash',
        'shell': 'bash',
        'lua': 'lua',
        'elixir': 'elixir',
        'haskell': 'haskell',
        'ocaml': 'ocaml',
        'r': 'r',
        'julia': 'julia',
        'dart': 'dart',
        'zig': 'zig',
        'nim': 'nim',
        'perl': 'perl',
        'clojure': 'clojure',
        'erlang': 'erlang',
        # Tier 3: Markup/config (basic outline)
        'html': 'html',
        'css': 'css',
        'json': 'json',
        'yaml': 'yaml',
        'toml': 'toml',
        'sql': 'sql',
        'markdown': 'markdown',
        'vue': 'vue',
        'svelte': 'svelte',
    }

    # Node types that represent symbols we want to extract (by language)
    SYMBOL_NODES = {
        'python': {
            'function_definition': 'function',
            'class_definition': 'class',
        },
        'typescript': {
            'function_declaration': 'function',
            'class_declaration': 'class',
            'method_definition': 'method',
            'arrow_function': 'function',
            'interface_declaration': 'interface',
        },
        'javascript': {
            'function_declaration': 'function',
            'class_declaration': 'class',
            'method_definition': 'method',
            'arrow_function': 'function',
        },
        'go': {
            'function_declaration': 'function',
            'method_declaration': 'method',
            'type_declaration': 'type',
        },
        'rust': {
            'function_item': 'function',
            'impl_item': 'impl',
            'struct_item': 'struct',
            'enum_item': 'enum',
            'trait_item': 'trait',
        },
        'java': {
            'method_declaration': 'method',
            'class_declaration': 'class',
            'interface_declaration': 'interface',
        },
        'c': {
            'function_definition': 'function',
            'struct_specifier': 'struct',
        },
        'cpp': {
            'function_definition': 'function',
            'class_specifier': 'class',
            'struct_specifier': 'struct',
        },
        'ruby': {
            'method': 'method',
            'class': 'class',
            'module': 'module',
        },
        'php': {
            'function_definition': 'function',
            'method_declaration': 'method',
            'class_declaration': 'class',
            'interface_declaration': 'interface',
            'trait_declaration': 'trait',
        },
        'swift': {
            'function_declaration': 'function',
            'class_declaration': 'class',
            'struct_declaration': 'struct',
            'protocol_declaration': 'protocol',
        },
        'kotlin': {
            'function_declaration': 'function',
            'class_declaration': 'class',
            'object_declaration': 'object',
            'interface_declaration': 'interface',
        },
        'scala': {
            'function_definition': 'function',
            'class_definition': 'class',
            'object_definition': 'object',
            'trait_definition': 'trait',
        },
        'bash': {
            'function_definition': 'function',
        },
        'lua': {
            'function_declaration': 'function',
            'function_definition': 'function',
        },
        'elixir': {
            'call': 'function',  # def/defp are calls in Elixir AST
        },
        'haskell': {
            'function': 'function',
            'data': 'data',
            'type_synonym': 'type',
        },
        'dart': {
            'function_signature': 'function',
            'class_definition': 'class',
            'method_signature': 'method',
        },
        'zig': {
            'fn_decl': 'function',
            'struct_decl': 'struct',
        },
        'c_sharp': {
            'method_declaration': 'method',
            'class_declaration': 'class',
            'interface_declaration': 'interface',
            'struct_declaration': 'struct',
        },
    }

    # Pattern queries for framework-specific symbols (routes, tests, handlers)
    # These capture patterns that node-type extraction misses
    # Note: predicates must be inside the pattern parentheses
    PATTERN_QUERIES = {
        'javascript': """
            ; Express routes: app.get('/path', handler), router.post('/path', handler)
            (call_expression
              function: (member_expression
                object: (identifier) @_router
                property: (property_identifier) @_method
                (#match? @_method "^(get|post|put|delete|patch|options|head|all|use)$"))
              arguments: (arguments
                (string) @path)) @route

            ; Jest/Mocha tests: describe('...', fn), it('...', fn), test('...', fn)
            (call_expression
              function: (identifier) @_test_fn
              (#match? @_test_fn "^(describe|it|test|beforeEach|afterEach|beforeAll|afterAll)$")
              arguments: (arguments
                (string) @test_name)) @test

            ; Event handlers: emitter.on('event', handler)
            (call_expression
              function: (member_expression
                property: (property_identifier) @_on_method
                (#match? @_on_method "^(on|once|addEventListener|addListener)$"))
              arguments: (arguments
                (string) @event_name)) @event_handler
        """,

        'typescript': """
            ; Express routes: app.get('/path', handler), router.post('/path', handler)
            (call_expression
              function: (member_expression
                object: (identifier) @_router
                property: (property_identifier) @_method
                (#match? @_method "^(get|post|put|delete|patch|options|head|all|use)$"))
              arguments: (arguments
                (string) @path)) @route

            ; Jest/Mocha tests: describe('...', fn), it('...', fn), test('...', fn)
            (call_expression
              function: (identifier) @_test_fn
              (#match? @_test_fn "^(describe|it|test|beforeEach|afterEach|beforeAll|afterAll)$")
              arguments: (arguments
                (string) @test_name)) @test

            ; Event handlers: emitter.on('event', handler)
            (call_expression
              function: (member_expression
                property: (property_identifier) @_on_method
                (#match? @_on_method "^(on|once|addEventListener|addListener)$"))
              arguments: (arguments
                (string) @event_name)) @event_handler
        """,

        'python': """
            ; Flask/FastAPI decorators: @app.route('/path'), @router.get('/path')
            (decorated_definition
              (decorator
                (call
                  function: (attribute
                    attribute: (identifier) @_method
                    (#match? @_method "^(route|get|post|put|delete|patch|options|head)$"))
                  arguments: (argument_list
                    (string) @path)))
              definition: (_) @_func) @route

            ; pytest tests: def test_something()
            (function_definition
              name: (identifier) @test_name
              (#match? @test_name "^test_")) @test
        """,
    }

    def __init__(self):
        self._parsers = {}
        self._queries = {}  # Cache compiled queries

    def get_parser(self, language: str):
        """Get or create a parser for the given language."""
        if not TREE_SITTER_AVAILABLE:
            return None

        ts_lang = self.LANG_MAP.get(language)
        if not ts_lang:
            return None

        if ts_lang not in self._parsers:
            lang_obj = get_ts_language(ts_lang)
            if not lang_obj:
                return None
            try:
                parser = Parser(lang_obj)
                self._parsers[ts_lang] = parser
            except Exception:
                return None

        return self._parsers.get(ts_lang)

    def get_query(self, language: str):
        """Get or create a compiled query for pattern matching."""
        if not TREE_SITTER_AVAILABLE:
            return None

        ts_lang = self.LANG_MAP.get(language)
        if not ts_lang:
            return None

        if ts_lang not in self._queries:
            query_str = self.PATTERN_QUERIES.get(ts_lang)
            if not query_str:
                return None
            lang_obj = get_ts_language(ts_lang)
            if not lang_obj:
                return None
            try:
                # Use Query() constructor (new API)
                query = Query(lang_obj, query_str)
                self._queries[ts_lang] = query
            except Exception:
                return None

        return self._queries.get(ts_lang)

    def _run_pattern_queries(self, tree, source: bytes, language: str) -> List['OutlineSymbol']:
        """Run pattern queries to extract framework-specific symbols."""
        query = self.get_query(language)
        if not query:
            return []

        symbols = []
        try:
            # Use QueryCursor for executing queries (new API)
            cursor = QueryCursor(query)
            captures = cursor.captures(tree.root_node)
        except Exception:
            return []

        # Group captures by their pattern type (@route, @test, @event_handler)
        # The captures dict has capture name as key and list of nodes as value
        for capture_name, nodes in captures.items():
            if capture_name.startswith('_'):
                # Skip internal captures (prefixed with _)
                continue

            for node in nodes:
                # Determine symbol kind and name based on capture type
                if capture_name == 'route':
                    # Find the path capture within this match
                    path_node = None
                    method_node = None
                    for child in node.children:
                        if child.type == 'member_expression':
                            for subchild in child.children:
                                if subchild.type == 'property_identifier':
                                    method_node = subchild
                        elif child.type == 'arguments':
                            for subchild in child.children:
                                if subchild.type == 'string':
                                    path_node = subchild
                                    break

                    if path_node and method_node:
                        method = source[method_node.start_byte:method_node.end_byte].decode('utf-8', errors='replace').upper()
                        path = source[path_node.start_byte:path_node.end_byte].decode('utf-8', errors='replace').strip('"\'')
                        name = f"{method} {path}"
                        symbols.append(OutlineSymbol(
                            name=name,
                            kind='route',
                            start_line=node.start_point[0] + 1,
                            end_line=node.end_point[0] + 1,
                            signature=source[node.start_byte:min(node.end_byte, node.start_byte + 80)].decode('utf-8', errors='replace').strip(),
                            children=[]
                        ))

                elif capture_name == 'test':
                    # Extract test name from the string argument
                    test_name_node = None
                    test_fn = None
                    for child in node.children:
                        if child.type == 'identifier':
                            test_fn = source[child.start_byte:child.end_byte].decode('utf-8', errors='replace')
                        elif child.type == 'arguments':
                            for subchild in child.children:
                                if subchild.type == 'string':
                                    test_name_node = subchild
                                    break

                    if test_name_node:
                        test_name = source[test_name_node.start_byte:test_name_node.end_byte].decode('utf-8', errors='replace').strip('"\'')
                        kind = 'test' if test_fn in ('it', 'test') else 'test_suite' if test_fn == 'describe' else 'test_hook'
                        name = f"{test_fn}: {test_name}"
                        symbols.append(OutlineSymbol(
                            name=name,
                            kind=kind,
                            start_line=node.start_point[0] + 1,
                            end_line=node.end_point[0] + 1,
                            signature=source[node.start_byte:min(node.end_byte, node.start_byte + 80)].decode('utf-8', errors='replace').strip(),
                            children=[]
                        ))

                elif capture_name == 'event_handler':
                    # Extract event name
                    event_name_node = None
                    for child in node.children:
                        if child.type == 'arguments':
                            for subchild in child.children:
                                if subchild.type == 'string':
                                    event_name_node = subchild
                                    break

                    if event_name_node:
                        event_name = source[event_name_node.start_byte:event_name_node.end_byte].decode('utf-8', errors='replace').strip('"\'')
                        name = f"on: {event_name}"
                        symbols.append(OutlineSymbol(
                            name=name,
                            kind='event',
                            start_line=node.start_point[0] + 1,
                            end_line=node.end_point[0] + 1,
                            signature=source[node.start_byte:min(node.end_byte, node.start_byte + 80)].decode('utf-8', errors='replace').strip(),
                            children=[]
                        ))

                elif capture_name == 'test_name':
                    # Python pytest: def test_something()
                    name = source[node.start_byte:node.end_byte].decode('utf-8', errors='replace')
                    # Get parent function for line info
                    parent = node.parent
                    if parent and parent.type == 'function_definition':
                        symbols.append(OutlineSymbol(
                            name=name,
                            kind='test',
                            start_line=parent.start_point[0] + 1,
                            end_line=parent.end_point[0] + 1,
                            signature=source[parent.start_byte:min(parent.end_byte, parent.start_byte + 80)].decode('utf-8', errors='replace').strip(),
                            children=[]
                        ))

                elif capture_name == 'path':
                    # Python Flask/FastAPI route - need to get parent decorated_definition
                    parent = node
                    while parent and parent.type != 'decorated_definition':
                        parent = parent.parent
                    if parent:
                        path = source[node.start_byte:node.end_byte].decode('utf-8', errors='replace').strip('"\'')
                        # Try to find method from decorator
                        method = 'ROUTE'
                        for child in parent.children:
                            if child.type == 'decorator':
                                dec_text = source[child.start_byte:child.end_byte].decode('utf-8', errors='replace')
                                for m in ['get', 'post', 'put', 'delete', 'patch']:
                                    if f'.{m}(' in dec_text.lower():
                                        method = m.upper()
                                        break
                        name = f"{method} {path}"
                        symbols.append(OutlineSymbol(
                            name=name,
                            kind='route',
                            start_line=parent.start_point[0] + 1,
                            end_line=parent.end_point[0] + 1,
                            signature=source[parent.start_byte:min(parent.end_byte, parent.start_byte + 80)].decode('utf-8', errors='replace').strip(),
                            children=[]
                        ))

        return symbols

    def _get_node_name(self, node, source_bytes: bytes, language: str) -> Optional[str]:
        """Extract the name of a symbol node."""
        # Different languages have different name child node types
        name_types = {
            'python': ['identifier', 'name'],
            'typescript': ['identifier', 'property_identifier'],
            'javascript': ['identifier', 'property_identifier'],
            'go': ['identifier', 'field_identifier'],
            'rust': ['identifier', 'type_identifier'],
            'java': ['identifier'],
            'c': ['identifier'],
            'cpp': ['identifier'],
            'ruby': ['identifier', 'constant'],
            'php': ['name'],
            'swift': ['identifier', 'simple_identifier'],
            'kotlin': ['simple_identifier', 'identifier'],
            'scala': ['identifier'],
            'c_sharp': ['identifier'],
            'bash': ['word'],
            'lua': ['identifier', 'name'],
            'elixir': ['identifier'],
            'haskell': ['identifier', 'variable'],
            'dart': ['identifier'],
            'zig': ['identifier'],
        }

        types_to_check = name_types.get(language, ['identifier', 'name'])

        for child in node.children:
            if child.type in types_to_check:
                return source_bytes[child.start_byte:child.end_byte].decode('utf-8', errors='replace')

        return None

    def _get_signature(self, node, source_bytes: bytes, max_len: int = 100) -> str:
        """Extract signature (first line of the node)."""
        start = node.start_byte
        # Find end of first line
        end = start
        while end < len(source_bytes) and end < start + max_len:
            if source_bytes[end:end+1] == b'\n':
                break
            end += 1
        return source_bytes[start:end].decode('utf-8', errors='replace').strip()

    def parse_file(self, file_path: str, language: str) -> List[OutlineSymbol]:
        """Parse a file and return its outline."""
        parser = self.get_parser(language)
        if not parser:
            return []

        try:
            with open(file_path, 'rb') as f:
                source = f.read()
        except (IOError, OSError):
            return []

        try:
            tree = parser.parse(source)
        except Exception:
            return []

        symbols = []
        symbol_types = self.SYMBOL_NODES.get(language, {})

        def walk(node, depth=0):
            if node.type in symbol_types:
                name = self._get_node_name(node, source, language)
                if name:
                    symbol = OutlineSymbol(
                        name=name,
                        kind=symbol_types[node.type],
                        start_line=node.start_point[0] + 1,  # 1-indexed
                        end_line=node.end_point[0] + 1,
                        signature=self._get_signature(node, source),
                        children=[]
                    )
                    symbols.append(symbol)

            for child in node.children:
                walk(child, depth + 1)

        walk(tree.root_node)

        # Run pattern queries for framework-specific symbols (routes, tests, handlers)
        pattern_symbols = self._run_pattern_queries(tree, source, language)
        symbols.extend(pattern_symbols)

        return symbols


# Global outline parser instance
outline_parser = OutlineParser()


class CodebaseIndex:
    """Single codebase index with inverted index, file metadata, and change log."""

    # Aggressive extension mapping - index everything, outline where tree-sitter available
    # Tier 1: Full tree-sitter support (rich outlines)
    # Tier 2: Basic indexing only (tokenization works, no structural outline)
    EXTENSIONS = {
        # === TIER 1: Tree-sitter supported (rich structural outline) ===
        # Core systems
        '.py': 'python',
        '.js': 'javascript', '.jsx': 'javascript', '.mjs': 'javascript', '.cjs': 'javascript',
        '.ts': 'typescript', '.tsx': 'typescript', '.mts': 'typescript',
        '.go': 'go',
        '.rs': 'rust',
        '.c': 'c', '.h': 'c',
        '.cpp': 'cpp', '.hpp': 'cpp', '.cc': 'cpp', '.cxx': 'cpp', '.hxx': 'cpp',
        '.java': 'java',
        '.cs': 'csharp',
        '.rb': 'ruby',
        '.php': 'php',
        '.swift': 'swift',
        '.kt': 'kotlin', '.kts': 'kotlin',
        '.scala': 'scala', '.sc': 'scala',
        '.lua': 'lua',
        '.ex': 'elixir', '.exs': 'elixir',
        '.hs': 'haskell', '.lhs': 'haskell',
        '.sh': 'bash', '.bash': 'bash', '.zsh': 'bash',
        '.sql': 'sql',
        '.html': 'html', '.htm': 'html',
        '.css': 'css', '.scss': 'scss', '.sass': 'scss', '.less': 'css',
        '.json': 'json', '.jsonc': 'json',
        '.yaml': 'yaml', '.yml': 'yaml',
        '.toml': 'toml',
        '.md': 'markdown', '.mdx': 'markdown',
        '.xml': 'xml', '.xsl': 'xml', '.xslt': 'xml',
        '.vue': 'vue',
        '.svelte': 'svelte',

        # === TIER 2: Indexing only (no tree-sitter, but still searchable) ===
        # JVM ecosystem
        '.groovy': 'groovy', '.gradle': 'groovy',
        '.clj': 'clojure', '.cljs': 'clojure', '.cljc': 'clojure', '.edn': 'clojure',
        # .NET ecosystem
        '.fs': 'fsharp', '.fsx': 'fsharp', '.fsi': 'fsharp',
        '.vb': 'vb',
        # Systems
        '.zig': 'zig',
        '.nim': 'nim',
        '.d': 'd',
        '.ada': 'ada', '.adb': 'ada', '.ads': 'ada',
        '.f90': 'fortran', '.f95': 'fortran', '.f03': 'fortran', '.f': 'fortran',
        '.cob': 'cobol', '.cbl': 'cobol',
        # Scripting
        '.pl': 'perl', '.pm': 'perl',
        '.r': 'r', '.R': 'r',
        '.jl': 'julia',
        '.tcl': 'tcl',
        '.awk': 'awk',
        '.sed': 'sed',
        # Functional
        '.ml': 'ocaml', '.mli': 'ocaml',
        '.erl': 'erlang', '.hrl': 'erlang',
        '.elm': 'elm',
        '.purs': 'purescript',
        '.rkt': 'racket',
        '.scm': 'scheme', '.ss': 'scheme',
        '.lisp': 'lisp', '.cl': 'lisp', '.el': 'elisp',
        # Web/mobile
        '.dart': 'dart',
        '.coffee': 'coffeescript',
        '.slim': 'slim',
        '.haml': 'haml',
        '.pug': 'pug', '.jade': 'pug',
        '.ejs': 'ejs',
        '.hbs': 'handlebars', '.handlebars': 'handlebars',
        '.mustache': 'mustache',
        '.twig': 'twig',
        '.liquid': 'liquid',
        # Data/config
        '.graphql': 'graphql', '.gql': 'graphql',
        '.proto': 'protobuf',
        '.thrift': 'thrift',
        '.avsc': 'avro',
        '.tf': 'terraform', '.tfvars': 'terraform',
        '.hcl': 'hcl',
        '.nix': 'nix',
        '.dhall': 'dhall',
        '.ini': 'ini', '.cfg': 'ini', '.conf': 'ini',
        '.env': 'dotenv',
        '.properties': 'properties',
        # DevOps/CI
        '.dockerfile': 'dockerfile',
        '.containerfile': 'dockerfile',
        '.jenkinsfile': 'groovy',
        '.makefile': 'make', '.mk': 'make',
        '.cmake': 'cmake',
        # Documentation
        '.rst': 'rst',
        '.adoc': 'asciidoc', '.asciidoc': 'asciidoc',
        '.tex': 'latex', '.latex': 'latex',
        '.org': 'org',
        # Misc
        '.diff': 'diff', '.patch': 'diff',
        '.log': 'log',
        '.csv': 'csv',
        '.tsv': 'tsv',
    }

    IGNORE_DIRS = {
        'node_modules', '.git', '__pycache__', 'target', 'dist',
        'build', '.next', '.nuxt', 'vendor', 'venv', '.venv',
        '.idea', '.vscode', 'coverage', '.cache', 'repos'
    }

    def __init__(self, root: str, name: str = 'local'):
        self.root = Path(root).resolve()
        self.name = name
        self.session_start = int(time.time())
        self.last_indexed = int(time.time())

        # Core data structures
        self.inverted_index: Dict[str, List[Location]] = defaultdict(list)
        self.files: Dict[str, FileMeta] = {}
        self.changes: List[ChangeRecord] = []

        # Dependency graph
        self.deps_outgoing: Dict[str, List[str]] = defaultdict(list)
        self.deps_incoming: Dict[str, List[str]] = defaultdict(list)

        # Thread safety
        self.lock = threading.RLock()

    def get_language(self, path: Path) -> str:
        return self.EXTENSIONS.get(path.suffix.lower(), 'unknown')

    def should_index(self, path: Path) -> bool:
        """Check if file should be indexed."""
        # Use relative path from index root for ignore checks
        try:
            rel_path = path.relative_to(self.root)
            parts = rel_path.parts
        except ValueError:
            parts = path.parts

        if any(part.startswith('.') for part in parts):
            return False
        if any(ignored in parts for ignored in self.IGNORE_DIRS):
            return False
        return path.suffix.lower() in self.EXTENSIONS

    def tokenize(self, content: str) -> List[Tuple[str, int, int]]:
        """Extract tokens with their positions."""
        tokens = []
        for line_num, line in enumerate(content.split('\n'), 1):
            for match in re.finditer(r'[a-zA-Z_][a-zA-Z0-9_]*', line):
                token = match.group()
                if len(token) >= 2:
                    tokens.append((token, line_num, match.start()))
        return tokens

    def index_file(self, path: Path) -> bool:
        """Index a single file."""
        try:
            content = path.read_text(encoding='utf-8', errors='ignore')
            stat = path.stat()
            mtime = int(stat.st_mtime)

            rel_path = str(path.relative_to(self.root))
            language = self.get_language(path)
            content_hash = hashlib.md5(content.encode()).hexdigest()[:16]

            with self.lock:
                if rel_path in self.files:
                    if self.files[rel_path].content_hash == content_hash:
                        return False
                    self._remove_file_from_index(rel_path)

                self.files[rel_path] = FileMeta(
                    path=rel_path,
                    mtime=mtime,
                    size=stat.st_size,
                    language=language,
                    content_hash=content_hash
                )

                for token, line, col in self.tokenize(content):
                    loc = Location(
                        file=rel_path,
                        line=line,
                        col=col,
                        symbol_type='token',
                        mtime=mtime
                    )
                    self.inverted_index[token].append(loc)
                    lower = token.lower()
                    if lower != token:
                        self.inverted_index[lower].append(loc)

                self._extract_deps(path, content, language)
                self.last_indexed = int(time.time())

            return True

        except Exception as e:
            print(f"Error indexing {path}: {e}")
            return False

    def _remove_file_from_index(self, rel_path: str):
        """Remove all entries for a file from the index."""
        for token, locations in list(self.inverted_index.items()):
            self.inverted_index[token] = [
                loc for loc in locations if loc.file != rel_path
            ]
            if not self.inverted_index[token]:
                del self.inverted_index[token]

        if rel_path in self.deps_outgoing:
            del self.deps_outgoing[rel_path]
        if rel_path in self.deps_incoming:
            del self.deps_incoming[rel_path]

    def _extract_deps(self, path: Path, content: str, language: str):
        """Extract import/dependency information."""
        rel_path = str(path.relative_to(self.root))
        imports = []

        if language in ('typescript', 'javascript'):
            for match in re.finditer(r'''(?:import\s+.*?from\s+|require\()['"]([^'"]+)['"]''', content):
                imports.append(match.group(1))
        elif language == 'python':
            for match in re.finditer(r'^(?:from\s+(\S+)|import\s+(\S+))', content, re.MULTILINE):
                imports.append(match.group(1) or match.group(2))
        elif language == 'rust':
            for match in re.finditer(r'^(?:use|mod)\s+([a-zA-Z_][a-zA-Z0-9_:]*)', content, re.MULTILINE):
                imports.append(match.group(1))

        if imports:
            self.deps_outgoing[rel_path] = imports
            for imp in imports:
                self.deps_incoming[imp].append(rel_path)

    def full_scan(self):
        """Scan entire codebase."""
        start = time.time()
        count = 0

        for path in self.root.rglob('*'):
            if path.is_file() and self.should_index(path):
                if self.index_file(path):
                    count += 1

        elapsed = time.time() - start
        print(f"[{self.name}] Indexed {count} files in {elapsed:.2f}s ({len(self.inverted_index)} symbols)")

    def record_change(self, path: Path, change_type: str):
        """Record a file change."""
        try:
            rel_path = str(path.relative_to(self.root))
        except ValueError:
            rel_path = str(path)

        with self.lock:
            self.changes.append(ChangeRecord(
                file=rel_path,
                timestamp=int(time.time()),
                change_type=change_type
            ))

    def search(self, query: str, mode: str = 'recent', limit: int = 20,
               since: int = None, before: int = None) -> List[dict]:
        """Search for a term with filename boosting and optional time filtering."""
        results = []

        with self.lock:
            if query in self.inverted_index:
                results.extend(self.inverted_index[query])
            lower = query.lower()
            if lower != query and lower in self.inverted_index:
                results.extend(self.inverted_index[lower])

        # Time filtering
        if since is not None or before is not None:
            filtered = []
            for loc in results:
                if since is not None and loc.mtime < since:
                    continue
                if before is not None and loc.mtime > before:
                    continue
                filtered.append(loc)
            results = filtered

        # Deduplicate by (file, line)
        seen = set()
        unique = []
        for loc in results:
            key = (loc.file, loc.line)
            if key not in seen:
                seen.add(key)
                unique.append(loc)

        # Score each result with filename boosting
        query_lower = query.lower()
        def score(loc):
            filename = loc.file.lower().split('/')[-1]  # Just the filename
            filepath = loc.file.lower()

            # Filename boost: files named after the query rank highest
            if query_lower in filename.replace('-', '').replace('_', ''):
                filename_boost = 1000
            elif query_lower in filename:
                filename_boost = 500
            elif query_lower in filepath:
                filename_boost = 100
            else:
                filename_boost = 0

            # Recency as secondary factor
            recency = loc.mtime

            return (filename_boost, recency)

        if mode == 'recent':
            unique.sort(key=score, reverse=True)
        else:
            unique.sort(key=lambda x: x.file)

        return [asdict(loc) for loc in unique[:limit]]

    def search_multi(self, terms: List[str], mode: str = 'recent', limit: int = 20,
                     since: int = None, before: int = None) -> List[dict]:
        """Search for multiple terms, rank by density."""
        all_results = []
        for term in terms:
            all_results.extend(self.search(term, mode, limit * 2, since=since, before=before))

        file_scores: Dict[str, Tuple[int, int]] = {}
        for loc in all_results:
            if loc['file'] not in file_scores:
                file_scores[loc['file']] = (0, loc['mtime'])
            count, mtime = file_scores[loc['file']]
            file_scores[loc['file']] = (count + 1, max(mtime, loc['mtime']))

        sorted_files = sorted(
            file_scores.items(),
            key=lambda x: (x[1][0], x[1][1]),
            reverse=True
        )

        top_files = set(f for f, _ in sorted_files[:limit])
        return [loc for loc in all_results if loc['file'] in top_files][:limit]

    def changes_since(self, since: int) -> List[dict]:
        """Get changes since timestamp."""
        with self.lock:
            return [asdict(c) for c in self.changes if c.timestamp >= since]

    def list_files(self, pattern: Optional[str] = None, mode: str = 'recent', limit: int = 50) -> List[dict]:
        """List files matching pattern."""
        with self.lock:
            results = list(self.files.values())

        if pattern:
            if '*' in pattern:
                regex = pattern.replace('.', r'\.').replace('*', '.*')
                results = [f for f in results if re.search(regex, f.path)]
            else:
                results = [f for f in results if pattern in f.path]

        if mode == 'recent':
            results.sort(key=lambda x: x.mtime, reverse=True)
        else:
            results.sort(key=lambda x: x.path)

        return [asdict(f) for f in results[:limit]]

    def get_stats(self) -> dict:
        """Get index statistics."""
        return {
            'name': self.name,
            'root': str(self.root),
            'files': len(self.files),
            'symbols': len(self.inverted_index),
            'last_indexed': self.last_indexed
        }

    def clear(self):
        """Clear the index."""
        with self.lock:
            self.inverted_index.clear()
            self.files.clear()
            self.changes.clear()
            self.deps_outgoing.clear()
            self.deps_incoming.clear()


# ============================================================================
# Index Manager - Manages local + repo indexes
# ============================================================================

class IndexManager:
    """Manages multiple isolated indexes: local project + knowledge repos.

    Supports two modes:
    - Legacy mode: Single local index from CODEBASE_ROOT
    - Global mode: Multiple project indexes from /config/projects.json
    """

    def __init__(self, local_root: str, repos_root: str, config_dir: str = None, indexes_dir: str = None):
        self.local_root = Path(local_root).resolve() if local_root else None
        self.repos_root = Path(repos_root).resolve()
        self.config_dir = Path(config_dir) if config_dir else None
        self.indexes_dir = Path(indexes_dir) if indexes_dir else None
        self.user_home = os.environ.get('USER_HOME', '/home')

        # Create repos directory if needed
        self.repos_root.mkdir(parents=True, exist_ok=True)

        # Determine mode
        self.global_mode = self.config_dir is not None and (self.config_dir / 'projects.json').exists()

        # Local index (legacy mode - your project)
        self.local: Optional[CodebaseIndex] = None
        if self.local_root and self.local_root.exists():
            self.local = CodebaseIndex(str(self.local_root), name='local')

        # Project indexes (global mode - multiple projects)
        self.projects: Dict[str, CodebaseIndex] = {}

        # Repo indexes (knowledge repos)
        self.repos: Dict[str, CodebaseIndex] = {}

        # File watchers
        self.observers: Dict[str, Observer] = {}

        self.lock = threading.RLock()

    def get_local(self, project_id: str = None) -> Optional[CodebaseIndex]:
        """Get the appropriate index for a query.

        In global mode, returns the project index if project_id is provided.
        In legacy mode, returns the single local index.

        IMPORTANT: In global mode, if project_id is provided but not found,
        returns None to prevent cross-project data leakage.
        In legacy mode, always falls back to the single local index.
        """
        if project_id:
            # Project ID provided - check registered projects first
            if project_id in self.projects:
                return self.projects[project_id]
            # In legacy mode (single index), fall back to local
            # This is safe because there's only one index anyway
            if self.local:
                return self.local
            # In global mode, don't fall back to wrong index
            return None

        # No project ID - legacy mode fallback
        if self.local:
            return self.local
        # Return first project if available (legacy compatibility)
        if self.projects:
            return next(iter(self.projects.values()))
        return None

    def init_local(self):
        """Initialize and scan local index (legacy mode)."""
        if self.local:
            print(f"Initializing local index: {self.local_root}")
            self.local.full_scan()
            self._start_watcher('local', self.local)
        elif self.global_mode:
            print("Global mode: No single local index, using project indexes")
            self._load_projects()

    def _load_projects(self):
        """Load all registered projects from config."""
        if not self.config_dir:
            return

        projects_file = self.config_dir / 'projects.json'
        if not projects_file.exists():
            print("No projects.json found")
            return

        try:
            projects = json.loads(projects_file.read_text())
            print(f"Loading {len(projects)} registered projects...")

            for proj in projects:
                self._load_project(proj['id'], proj['name'], proj['path'])
        except Exception as e:
            print(f"Error loading projects: {e}")

    def _load_project(self, project_id: str, name: str, path: str) -> Optional[CodebaseIndex]:
        """Load or create index for a project."""
        # Convert path to container path (user's home is mounted at /userhome)
        container_path = path.replace(self.user_home, '/userhome')

        if not Path(container_path).exists():
            print(f"  Project path not accessible: {path}")
            return None

        with self.lock:
            if project_id in self.projects:
                return self.projects[project_id]

            print(f"  Loading project: {name} ({project_id})")
            idx = CodebaseIndex(container_path, name=name)
            idx.full_scan()
            self.projects[project_id] = idx
            self._start_watcher(f"project:{project_id}", idx)
            print(f"    -> {len(idx.files)} files indexed")
            return idx

    def register_project(self, project_id: str, name: str, path: str) -> Tuple[bool, str, int]:
        """Register and index a new project."""
        try:
            idx = self._load_project(project_id, name, path)
            if idx:
                return True, f"Project '{name}' registered", len(idx.files)
            else:
                return False, f"Could not access project path: {path}", 0
        except Exception as e:
            return False, f"Error registering project: {e}", 0

    def unregister_project(self, project_id: str) -> Tuple[bool, str]:
        """Unregister a project and remove its index."""
        with self.lock:
            # Stop watcher
            self._stop_watcher(f"project:{project_id}")

            # Remove from index
            if project_id in self.projects:
                del self.projects[project_id]
                return True, f"Project unregistered"
            else:
                return False, f"Project not found"

    def init_repos(self):
        """Initialize indexes for existing repos."""
        if not self.repos_root.exists():
            return

        for repo_dir in self.repos_root.iterdir():
            if repo_dir.is_dir() and not repo_dir.name.startswith('.'):
                self._load_repo(repo_dir.name)

    def _load_repo(self, name: str) -> Optional[CodebaseIndex]:
        """Load an existing repo into the index."""
        repo_path = self.repos_root / name
        if not repo_path.exists():
            return None

        with self.lock:
            if name in self.repos:
                return self.repos[name]

            print(f"Loading repo index: {name}")
            idx = CodebaseIndex(str(repo_path), name=name)
            idx.full_scan()
            self.repos[name] = idx
            self._start_watcher(name, idx)
            return idx

    def _start_watcher(self, name: str, idx: CodebaseIndex):
        """Start file watcher for an index."""
        handler = IndexerHandler(idx)
        observer = Observer()
        observer.schedule(handler, str(idx.root), recursive=True)
        observer.start()
        self.observers[name] = observer
        print(f"File watcher started for: {name}")

    def _stop_watcher(self, name: str):
        """Stop file watcher for an index."""
        if name in self.observers:
            self.observers[name].stop()
            self.observers[name].join()
            del self.observers[name]

    def add_repo(self, name: str, git_url: str) -> Tuple[bool, str]:
        """Clone a git repo and index it."""
        repo_path = self.repos_root / name

        if repo_path.exists():
            return False, f"Repo '{name}' already exists"

        # Clone the repo
        try:
            print(f"Cloning {git_url} to {repo_path}...")
            result = subprocess.run(
                ['git', 'clone', '--depth', '1', git_url, str(repo_path)],
                capture_output=True,
                text=True,
                timeout=300
            )
            if result.returncode != 0:
                return False, f"Git clone failed: {result.stderr}"
        except subprocess.TimeoutExpired:
            return False, "Git clone timed out"
        except Exception as e:
            return False, f"Git clone error: {e}"

        # Index the repo
        idx = self._load_repo(name)
        if idx:
            return True, f"Repo '{name}' added with {len(idx.files)} files"
        else:
            return False, "Failed to index repo"

    def remove_repo(self, name: str) -> Tuple[bool, str]:
        """Remove a repo and its index."""
        repo_path = self.repos_root / name

        with self.lock:
            # Stop watcher
            self._stop_watcher(name)

            # Remove from index
            if name in self.repos:
                del self.repos[name]

            # Remove files
            if repo_path.exists():
                import shutil
                shutil.rmtree(repo_path)
                return True, f"Repo '{name}' removed"
            else:
                return False, f"Repo '{name}' not found"

    def list_repos(self) -> List[dict]:
        """List all knowledge repos."""
        repos = []
        with self.lock:
            for name, idx in self.repos.items():
                repos.append(idx.get_stats())
        return repos

    def get_repo(self, name: str) -> Optional[CodebaseIndex]:
        """Get a repo index by name."""
        with self.lock:
            return self.repos.get(name)

    def shutdown(self):
        """Stop all watchers."""
        for name in list(self.observers.keys()):
            self._stop_watcher(name)


# ============================================================================
# File Watcher
# ============================================================================

class IndexerHandler(FileSystemEventHandler):
    def __init__(self, index: CodebaseIndex):
        self.index = index

    def on_modified(self, event):
        if event.is_directory:
            return
        path = Path(event.src_path)
        if self.index.should_index(path):
            if self.index.index_file(path):
                self.index.record_change(path, 'modified')

    def on_created(self, event):
        if event.is_directory:
            return
        path = Path(event.src_path)
        if self.index.should_index(path):
            if self.index.index_file(path):
                self.index.record_change(path, 'added')

    def on_deleted(self, event):
        if event.is_directory:
            return
        path = Path(event.src_path)
        try:
            rel_path = str(path.relative_to(self.index.root))
            with self.index.lock:
                if rel_path in self.index.files:
                    self.index._remove_file_from_index(rel_path)
                    del self.index.files[rel_path]
                    self.index.record_change(path, 'deleted')
        except Exception:
            pass


# ============================================================================
# Intent Index - Semantic layer over tool usage
# ============================================================================

class IntentIndex:
    """
    Bidirectional index for intent tracking with per-project isolation.

    Stores (per project):
    - tag -> files: Which files are associated with each intent tag
    - file -> tags: Which intent tags are associated with each file
    - timeline: Chronological list of all intent records (in-memory, session-scoped)

    Persists to Redis (survives restarts):
    - Cumulative metrics (total_intents, total_tokens_saved)
    - Tag-file associations
    - First seen timestamp
    """

    DEFAULT_PROJECT = "_global"  # Fallback for requests without project_id

    def __init__(self, redis_client=None):
        # All data structures are nested by project_id
        self.tag_to_files: Dict[str, Dict[str, Set[str]]] = defaultdict(lambda: defaultdict(set))
        self.file_to_tags: Dict[str, Dict[str, Set[str]]] = defaultdict(lambda: defaultdict(set))
        self.timeline: Dict[str, List[IntentRecord]] = defaultdict(list)
        self.session_intents: Dict[str, Dict[str, List[IntentRecord]]] = defaultdict(lambda: defaultdict(list))
        self.lock = threading.RLock()
        self.redis = redis_client  # Optional Redis for persistence

    def _project_key(self, project_id: str = None) -> str:
        """Get project key, using default for empty/None."""
        return project_id if project_id else self.DEFAULT_PROJECT

    def record(self, tool: str, files: List[str], tags: List[str], session_id: str,
               tool_use_id: str = None, project_id: str = None, file_sizes: Dict[str, int] = None,
               output_size: int = None):
        """Record an intent from a tool use."""
        proj = self._project_key(project_id)
        record = IntentRecord(
            timestamp=int(time.time()),
            session_id=session_id,
            tool=tool,
            files=files,
            tags=tags,
            tool_use_id=tool_use_id,
            project_id=project_id,
            file_sizes=file_sizes or {},
            output_size=output_size
        )

        # Calculate token savings for this record
        tokens_saved = 0
        if output_size and output_size > 0 and file_sizes:
            for f, size in file_sizes.items():
                if size > 0:
                    baseline_tokens = size // 4  # bytes to ~tokens
                    actual_tokens = output_size // 4
                    tokens_saved = max(0, baseline_tokens - actual_tokens)
                    break  # Only count first file

        with self.lock:
            # Add to project-specific timeline
            self.timeline[proj].append(record)
            self.session_intents[proj][session_id].append(record)

            # Update project-specific bidirectional indexes
            for tag in tags:
                for f in files:
                    self.tag_to_files[proj][tag].add(f)
                    self.file_to_tags[proj][f].add(tag)

        # Persist to Redis (cumulative, survives restarts)
        # Note: self.redis is RedisClient wrapper, use .client for raw redis-py operations
        if self.redis:
            try:
                r = self.redis.client  # Get raw redis-py client
                redis_key = f"aoa:{proj}:metrics"
                # Increment cumulative counters
                r.hincrby(redis_key, 'total_intents', 1)
                if tokens_saved > 0:
                    r.hincrby(redis_key, 'total_tokens_saved', tokens_saved)
                # Set first_seen if not exists
                r.hsetnx(redis_key, 'first_seen', int(time.time()))
                # Update last_active
                r.hset(redis_key, 'last_active', int(time.time()))

                # Persist tag-file associations
                for tag in tags:
                    for f in files:
                        r.sadd(f"aoa:{proj}:tags:{tag}", f)
                        r.sadd(f"aoa:{proj}:file_tags:{f}", tag)
            except Exception as e:
                print(f"[IntentIndex] Redis persistence error: {e}", flush=True)

    def files_for_tag(self, tag: str, project_id: str = None) -> List[str]:
        """Get files associated with a tag."""
        proj = self._project_key(project_id)
        with self.lock:
            return list(self.tag_to_files[proj].get(tag, set()))

    def tags_for_file(self, file: str, project_id: str = None) -> List[str]:
        """Get tags associated with a file."""
        proj = self._project_key(project_id)
        with self.lock:
            proj_file_to_tags = self.file_to_tags[proj]
            # Try exact match first, then partial
            if file in proj_file_to_tags:
                return list(proj_file_to_tags[file])
            # Partial match (filename only)
            for f, tags in proj_file_to_tags.items():
                if f.endswith(file) or file in f:
                    return list(tags)
            return []

    def recent(self, since: int = None, limit: int = 50, project_id: str = None) -> List[dict]:
        """Get recent intent records."""
        proj = self._project_key(project_id)
        with self.lock:
            records = self.timeline[proj]
            if since:
                records = [r for r in records if r.timestamp >= since]
            records = records[-limit:]
            return [asdict(r) for r in reversed(records)]

    def session(self, session_id: str, project_id: str = None) -> List[dict]:
        """Get intent records for a session."""
        proj = self._project_key(project_id)
        with self.lock:
            return [asdict(r) for r in self.session_intents[proj].get(session_id, [])]

    def all_tags(self, project_id: str = None) -> List[Tuple[str, int]]:
        """Get all tags with file counts, sorted by count."""
        proj = self._project_key(project_id)
        with self.lock:
            return sorted(
                [(tag, len(files)) for tag, files in self.tag_to_files[proj].items()],
                key=lambda x: x[1],
                reverse=True
            )

    def get_stats(self, project_id: str = None) -> dict:
        """Get intent index statistics including token savings.

        Combines:
        - Cumulative metrics from Redis (persisted across restarts)
        - Session metrics from memory (current session only)
        """
        proj = self._project_key(project_id)

        # Get cumulative metrics from Redis (persisted)
        cumulative_intents = 0
        cumulative_tokens_saved = 0
        first_seen = None
        redis_tags_count = 0

        if self.redis:
            try:
                r = self.redis.client  # Get raw redis-py client
                redis_key = f"aoa:{proj}:metrics"
                metrics = r.hgetall(redis_key)
                if metrics:
                    cumulative_intents = int(metrics.get('total_intents', 0))
                    cumulative_tokens_saved = int(metrics.get('total_tokens_saved', 0))
                    first_seen = int(metrics.get('first_seen', 0)) if metrics.get('first_seen') else None

                # Count persisted tags
                tag_keys = self.redis.keys(f"aoa:{proj}:tags:*")
                redis_tags_count = len(tag_keys) if tag_keys else 0
            except Exception:
                pass

        # Get session metrics from memory
        with self.lock:
            session_records = len(self.timeline[proj])
            session_tags = len(self.tag_to_files[proj])
            session_files = len(self.file_to_tags[proj])
            session_count = len(self.session_intents[proj])

            # Calculate session token savings
            session_baseline = 0
            session_actual = 0
            measured_count = 0

            for record in self.timeline[proj]:
                if record.output_size and record.output_size > 0 and record.file_sizes:
                    for f, size in record.file_sizes.items():
                        if size > 0:
                            session_baseline += size // 4
                            session_actual += record.output_size // 4
                            measured_count += 1
                            break

            session_tokens_saved = max(0, session_baseline - session_actual)

        # Combine: cumulative (Redis) includes session (memory) via hincrby
        # So we use Redis values directly for totals
        total_tokens_saved = cumulative_tokens_saved if cumulative_tokens_saved > 0 else session_tokens_saved

        # Estimate time savings
        time_sec = total_tokens_saved * 0.0075

        return {
            'total_records': cumulative_intents if cumulative_intents > 0 else session_records,
            'unique_tags': max(redis_tags_count, session_tags),
            'unique_files': session_files,  # File count from memory is accurate for session
            'sessions': session_count,
            'project_id': project_id,
            'first_seen': first_seen,
            'savings': {
                'tokens': total_tokens_saved,
                'time_sec': round(time_sec, 1),
                'baseline': session_baseline,  # Session baseline for debugging
                'actual': session_actual,
                'measured_records': measured_count
            }
        }


# ============================================================================
# Global Index Manager
# ============================================================================

manager: Optional[IndexManager] = None
intent_index: Optional[IntentIndex] = None


# ============================================================================
# API Endpoints - Local Index (default)
# ============================================================================

@app.route('/health')
def health():
    local = manager.get_local()

    response = {
        'status': 'ok',
        'mode': 'global' if manager.global_mode else 'legacy',
        'repos': [r.get_stats() for r in manager.repos.values()]
    }

    if local:
        response['local'] = local.get_stats()

    if manager.projects:
        response['projects'] = [
            {
                'id': pid,
                'name': idx.name,
                'files': len(idx.files),
                'symbols': len(idx.inverted_index)
            }
            for pid, idx in manager.projects.items()
        ]

    return jsonify(response)

@app.route('/symbol')
def symbol_search():
    start = time.time()
    try:
        q = request.args.get('q', '')
        mode = request.args.get('mode', 'recent')
        limit = int(request.args.get('limit', 20))
        project = request.args.get('project')  # Optional project ID
        since = request.args.get('since')  # Unix timestamp or seconds ago
        before = request.args.get('before')  # Unix timestamp or seconds ago

        # Convert time params to absolute timestamps
        now = int(time.time())
        since_ts = None
        before_ts = None
        if since:
            since_val = int(since)
            since_ts = since_val if since_val > 1000000000 else now - since_val
        if before:
            before_val = int(before)
            before_ts = before_val if before_val > 1000000000 else now - before_val

        idx = manager.get_local(project)
        if not idx:
            return jsonify({
                'error': 'No index available',
                'message': 'Run "aoa init" in a project to register it',
                'results': [],
                'ms': 0
            }), 404

        results = idx.search(q, mode, limit, since=since_ts, before=before_ts)

        return jsonify({
            'results': results,
            'index': idx.name,
            'project': project,
            'ms': (time.time() - start) * 1000
        })
    except Exception as e:
        return jsonify({
            'error': 'Search failed',
            'message': str(e),
            'results': [],
            'ms': (time.time() - start) * 1000
        }), 500

@app.route('/multi', methods=['GET', 'POST'])
def multi_search():
    start = time.time()

    # Support both GET (query params) and POST (JSON body)
    if request.method == 'GET':
        q = request.args.get('q', '')
        terms = q.split() if q else []
        mode = request.args.get('mode', 'recent')
        limit = int(request.args.get('limit', 20))
        project = request.args.get('project')
        since = request.args.get('since')
        before = request.args.get('before')
    else:
        data = request.json
        terms = data.get('terms', [])
        mode = data.get('mode', 'recent')
        limit = int(data.get('limit', 20))
        project = data.get('project')
        since = data.get('since')
        before = data.get('before')

    # Convert time params to absolute timestamps
    now = int(time.time())
    since_ts = None
    before_ts = None
    if since:
        since_val = int(since)
        since_ts = since_val if since_val > 1000000000 else now - since_val
    if before:
        before_val = int(before)
        before_ts = before_val if before_val > 1000000000 else now - before_val

    idx = manager.get_local(project)
    if not idx:
        return jsonify({'error': 'No index available', 'results': [], 'ms': 0}), 404

    results = idx.search_multi(terms, mode, limit, since=since_ts, before=before_ts)

    return jsonify({
        'results': results,
        'index': idx.name,
        'project': project,
        'ms': (time.time() - start) * 1000
    })


@app.route('/outline')
def get_outline():
    """Get code outline (functions, classes) for a file using tree-sitter."""
    start = time.time()

    file_path = request.args.get('file')
    project = request.args.get('project')

    if not file_path:
        return jsonify({'error': 'Missing file parameter', 'symbols': [], 'ms': 0}), 400

    if not TREE_SITTER_AVAILABLE:
        return jsonify({
            'error': 'tree-sitter not available',
            'message': 'Install tree-sitter and tree-sitter-languages',
            'symbols': [],
            'ms': 0
        }), 503

    idx = manager.get_local(project)
    if not idx:
        return jsonify({'error': 'No index available', 'symbols': [], 'ms': 0}), 404

    # Resolve file path
    full_path = Path(idx.root) / file_path if not Path(file_path).is_absolute() else Path(file_path)

    if not full_path.exists():
        return jsonify({'error': f'File not found: {file_path}', 'symbols': [], 'ms': 0}), 404

    # Get language from file extension
    language = idx.get_language(full_path)
    if language == 'unknown':
        return jsonify({
            'error': f'Unsupported language for: {file_path}',
            'symbols': [],
            'ms': (time.time() - start) * 1000
        }), 400

    # Parse and get outline
    symbols = outline_parser.parse_file(str(full_path), language)

    return jsonify({
        'file': str(file_path),
        'language': language,
        'symbols': [asdict(s) for s in symbols],
        'count': len(symbols),
        'ms': (time.time() - start) * 1000
    })


@app.route('/outline/enriched', methods=['POST'])
def mark_enriched():
    """Store semantic compression tags with counting (idempotent, tracks confidence)."""
    data = request.json
    file_path = data.get('file')
    project = data.get('project')
    symbols = data.get('symbols', [])  # List of {name, kind, line, end_line, tags}

    if not file_path:
        return jsonify({'success': False, 'error': 'Missing file parameter'}), 400

    idx = manager.get_local(project)
    if not idx:
        return jsonify({'success': False, 'error': 'No index available'}), 404

    tags_indexed = 0
    tags_incremented = 0
    mtime = int(time.time())
    project_key = project or 'default'

    # Use Redis for tag counting (dedup + confidence tracking)
    try:
        import redis
        r = redis.from_url(os.environ.get('REDIS_URL', 'redis://localhost:6379/0'))

        for sym in symbols:
            sym_name = sym.get('name', '')
            sym_kind = sym.get('kind', 'unknown')
            line = sym.get('line', 0)
            end_line = sym.get('end_line', line)
            tags = sym.get('tags', [])

            for tag in tags:
                # Key: tag_count:{project}:{file}:{symbol}:{tag}
                count_key = f"tag_count:{project_key}:{file_path}:{sym_name}:{tag}"

                # Increment count (creates key with value 1 if doesn't exist)
                new_count = r.incr(count_key)

                if new_count == 1:
                    # First time seeing this tag - add to inverted index
                    with idx.lock:
                        loc = Location(
                            file=file_path,
                            line=line,
                            col=0,
                            symbol_type='tag',
                            mtime=mtime,
                            symbol=sym_name,
                            symbol_kind=sym_kind,
                            end_line=end_line
                        )
                        idx.inverted_index[tag].append(loc)
                    tags_indexed += 1
                else:
                    # Already exists, just incremented count
                    tags_incremented += 1

                # Also store symbol metadata for retrieval
                meta_key = f"tag_meta:{project_key}:{file_path}:{sym_name}:{tag}"
                r.hset(meta_key, mapping={
                    'kind': sym_kind,
                    'line': line,
                    'end_line': end_line,
                    'updated': mtime
                })

        # Store enrichment timestamp
        enrich_key = f"enriched:{project_key}:{file_path}"
        r.hset(enrich_key, mapping={
            'enriched_at': mtime,
            'tags_count': tags_indexed + tags_incremented
        })

    except Exception as e:
        # Fallback: just append without dedup (legacy behavior)
        with idx.lock:
            for sym in symbols:
                sym_name = sym.get('name', '')
                sym_kind = sym.get('kind', 'unknown')
                line = sym.get('line', 0)
                end_line = sym.get('end_line', line)
                tags = sym.get('tags', [])

                for tag in tags:
                    loc = Location(
                        file=file_path,
                        line=line,
                        col=0,
                        symbol_type='tag',
                        mtime=mtime,
                        symbol=sym_name,
                        symbol_kind=sym_kind,
                        end_line=end_line
                    )
                    idx.inverted_index[tag].append(loc)
                    tags_indexed += 1

    return jsonify({
        'success': True,
        'file': file_path,
        'tags_indexed': tags_indexed,
        'tags_incremented': tags_incremented,
        'symbols_processed': len(symbols),
        'enriched_at': mtime
    })


@app.route('/outline/tags')
def get_symbol_tags():
    """Get semantic tags for symbols in a file, with confidence counts."""
    file_path = request.args.get('file', '')
    project = request.args.get('project')
    include_counts = request.args.get('counts', 'false').lower() == 'true'

    idx = manager.get_local(project)
    if not idx:
        return jsonify({'error': 'No index available', 'tags': {}}), 404

    project_key = project or 'default'
    tags_by_symbol = {}

    # Try to get counts from Redis
    try:
        import redis
        r = redis.from_url(os.environ.get('REDIS_URL', 'redis://localhost:6379/0'))

        # Scan for all tag counts for this file
        pattern = f"tag_count:{project_key}:{file_path}:*"
        cursor = 0
        while True:
            cursor, keys = r.scan(cursor, match=pattern, count=100)
            for key in keys:
                # Parse key: tag_count:{project}:{file}:{symbol}:{tag}
                parts = key.decode().split(':')
                if len(parts) >= 5:
                    symbol_name = parts[3]
                    tag = ':'.join(parts[4:])  # Handle tags with colons
                    count = int(r.get(key) or 1)

                    if symbol_name not in tags_by_symbol:
                        tags_by_symbol[symbol_name] = []

                    if include_counts:
                        tags_by_symbol[symbol_name].append({'tag': tag, 'count': count})
                    else:
                        if tag not in tags_by_symbol[symbol_name]:
                            tags_by_symbol[symbol_name].append(tag)

            if cursor == 0:
                break

    except Exception:
        # Fallback: use inverted index (no counts)
        with idx.lock:
            for tag, locations in idx.inverted_index.items():
                if not tag.startswith('#'):
                    continue

                for loc in locations:
                    if loc.file == file_path or loc.file.endswith(file_path):
                        symbol_name = loc.symbol or 'file'
                        if symbol_name not in tags_by_symbol:
                            tags_by_symbol[symbol_name] = []
                        if tag not in tags_by_symbol[symbol_name]:
                            tags_by_symbol[symbol_name].append(tag)

    return jsonify({
        'file': file_path,
        'tags': tags_by_symbol
    })


@app.route('/outline/pending')
def get_pending_enrichment():
    """Get files that need enrichment (modified since last enriched or never enriched)."""
    start = time.time()
    project = request.args.get('project')

    idx = manager.get_local(project)
    if not idx:
        return jsonify({'error': 'No index available', 'pending': [], 'ms': 0}), 404

    try:
        import redis
        r = redis.from_url(os.environ.get('REDIS_URL', 'redis://localhost:6379/0'))
    except Exception:
        r = None

    pending = []
    up_to_date = []

    for rel_path, meta in idx.files.items():
        # Check if file has been enriched
        enriched_at = 0
        if r:
            key = f"enriched:{project or 'default'}:{rel_path}"
            data = r.hgetall(key)
            if data and b'enriched_at' in data:
                enriched_at = int(data[b'enriched_at'])

        # Compare mtime to enriched_at
        if meta.mtime > enriched_at:
            pending.append({
                'file': rel_path,
                'language': meta.language,
                'mtime': meta.mtime,
                'enriched_at': enriched_at if enriched_at > 0 else None,
                'reason': 'never' if enriched_at == 0 else 'modified'
            })
        else:
            up_to_date.append(rel_path)

    # Sort pending by mtime (most recently modified first)
    pending.sort(key=lambda x: x['mtime'], reverse=True)

    return jsonify({
        'pending': pending,
        'pending_count': len(pending),
        'up_to_date_count': len(up_to_date),
        'total_files': len(idx.files),
        'ms': (time.time() - start) * 1000
    })


# ============================================================================
# Domain Pattern Candidates (Learned from quickstart)
# ============================================================================

@app.route('/patterns/candidates', methods=['GET', 'POST'])
def domain_candidates():
    """Store or retrieve project-specific domain keyword candidates.

    POST: Store word frequencies discovered during quickstart
    GET: Retrieve stored candidates for a project

    These are high-frequency words NOT in universal patterns -
    candidates for project-specific tags.
    """
    # Handle project param differently for GET vs POST
    if request.method == 'POST':
        data = request.get_json(silent=True) or {}
        project = request.args.get('project') or data.get('project', '')
    else:
        project = request.args.get('project', '')

    try:
        import redis
        r = redis.from_url(os.environ.get('REDIS_URL', 'redis://localhost:6379/0'))
    except Exception:
        return jsonify({'error': 'Redis not available'}), 503

    redis_key = f"aoa:{project or 'default'}:domain_candidates"

    if request.method == 'POST':
        if not data:
            return jsonify({'error': 'No data provided'}), 400

        candidates = data.get('candidates', {})  # {word: count}
        suggested_domain = data.get('suggested_domain', '')
        total_symbols = data.get('total_symbols', 0)

        # Store in Redis as hash
        r.delete(redis_key)  # Clear old data
        if candidates:
            r.hset(redis_key, mapping=candidates)
        r.hset(redis_key, '_meta_domain', suggested_domain)
        r.hset(redis_key, '_meta_symbols', str(total_symbols))
        r.hset(redis_key, '_meta_timestamp', str(int(time.time())))

        return jsonify({
            'success': True,
            'stored': len(candidates),
            'suggested_domain': suggested_domain
        })

    else:  # GET
        data = r.hgetall(redis_key)
        if not data:
            return jsonify({
                'candidates': {},
                'suggested_domain': '',
                'total_symbols': 0
            })

        # Parse stored data
        candidates = {}
        suggested_domain = ''
        total_symbols = 0
        timestamp = 0

        for k, v in data.items():
            key = k.decode() if isinstance(k, bytes) else k
            val = v.decode() if isinstance(v, bytes) else v

            if key == '_meta_domain':
                suggested_domain = val
            elif key == '_meta_symbols':
                total_symbols = int(val)
            elif key == '_meta_timestamp':
                timestamp = int(val)
            else:
                candidates[key] = int(val)

        # Sort by count descending
        sorted_candidates = dict(sorted(candidates.items(), key=lambda x: x[1], reverse=True))

        return jsonify({
            'candidates': sorted_candidates,
            'suggested_domain': suggested_domain,
            'total_symbols': total_symbols,
            'timestamp': timestamp
        })


@app.route('/patterns/learned', methods=['GET', 'POST'])
def learned_patterns():
    """Store or retrieve project-specific learned patterns.

    POST: Store keyword->tag mappings (from Claude analysis or user input)
    GET: Retrieve learned patterns for merging with universal patterns
    """
    if request.method == 'POST':
        data = request.get_json(silent=True) or {}
        project = request.args.get('project') or data.get('project', '')
    else:
        data = {}
        project = request.args.get('project', '')

    try:
        import redis
        r = redis.from_url(os.environ.get('REDIS_URL', 'redis://localhost:6379/0'))
    except Exception:
        return jsonify({'error': 'Redis not available'}), 503

    redis_key = f"aoa:{project or 'default'}:learned_patterns"

    if request.method == 'POST':
        if not data:
            return jsonify({'error': 'No data provided'}), 400

        patterns = data.get('patterns', {})  # {keyword: tag}

        # Store in Redis as hash
        if patterns:
            r.hset(redis_key, mapping=patterns)
            r.hset(redis_key, '_meta_timestamp', str(int(time.time())))

        return jsonify({
            'success': True,
            'stored': len(patterns)
        })

    else:  # GET
        data = r.hgetall(redis_key)
        if not data:
            return jsonify({'patterns': {}})

        patterns = {}
        for k, v in data.items():
            key = k.decode() if isinstance(k, bytes) else k
            val = v.decode() if isinstance(v, bytes) else v
            if not key.startswith('_meta_'):
                patterns[key] = val

        return jsonify({'patterns': patterns})


@app.route('/patterns/infer', methods=['POST'])
def infer_patterns():
    """Infer tags from symbol names using the pattern library.

    POST body: {"symbols": [{"name": "getUserById", "kind": "function"}, ...]}
    Returns: {"tags": [["#read", "#user"], ...]}

    Uses the same pattern library as the Python hooks (semantic-patterns.json + domain-patterns.json).
    """
    import re
    from pathlib import Path

    data = request.get_json(silent=True) or {}
    symbols = data.get('symbols', [])

    if not symbols:
        return jsonify({'tags': []})

    # Load pattern configs (cached after first call)
    config_paths = [
        Path('/app/config'),  # Docker mount
        Path(__file__).parent.parent / 'config',  # Local dev
        Path('/codebase/config'),  # Alternative
    ]

    semantic_patterns = {}
    domain_keywords = {}
    class_suffixes = {}

    for config_dir in config_paths:
        semantic_file = config_dir / 'semantic-patterns.json'
        domain_file = config_dir / 'domain-patterns.json'

        if semantic_file.exists():
            try:
                import json
                sdata = json.loads(semantic_file.read_text())
                for cat_name, cat_data in sdata.get('categories', {}).items():
                    patterns = set(p.lower() for p in cat_data.get('patterns', []))
                    semantic_patterns[cat_name] = {
                        'patterns': patterns,
                        'tag': cat_data.get('tag', f'#{cat_name}'),
                        'priority': cat_data.get('priority', 3)
                    }
                kind_patterns = sdata.get('kind_patterns', {}).get('patterns', {})
                class_suffixes.update(kind_patterns.get('class', {}).get('suffix_patterns', {}))
            except:
                pass

        if domain_file.exists():
            try:
                import json
                ddata = json.loads(domain_file.read_text())
                for domain_name, domain_data in ddata.get('domains', {}).items():
                    tag = domain_data.get('tag', f'#{domain_name}')
                    for keyword in domain_data.get('keywords', []):
                        domain_keywords[keyword.lower()] = tag
                for suffix, tag in ddata.get('technical_suffixes', {}).items():
                    if suffix != 'description':
                        class_suffixes[suffix] = tag
            except:
                pass

        if semantic_patterns or domain_keywords:
            break

    def tokenize(text):
        tokens = set()
        parts = re.split(r'[/_\-.\s]+', text)
        for part in parts:
            if not part:
                continue
            tokens.add(part.lower())
            camel_parts = re.findall(r'[A-Z]?[a-z]+|[A-Z]+(?=[A-Z][a-z]|\d|\W|$)|\d+', part)
            for cp in camel_parts:
                tokens.add(cp.lower())
        return tokens

    def match_semantic(tokens):
        tags = set()
        for cat_data in semantic_patterns.values():
            for token in tokens:
                for pattern in cat_data['patterns']:
                    if token.startswith(pattern) or token == pattern:
                        tags.add(cat_data['tag'])
                        break
        return tags

    def match_domain(tokens, full_text):
        tags = set()
        full_lower = full_text.lower()
        for keyword, tag in domain_keywords.items():
            if keyword in tokens or keyword in full_lower:
                tags.add(tag)
        return tags

    def match_suffix(name):
        tags = set()
        basename = name.split('.')[-1] if '.' in name else name
        for suffix, tag in class_suffixes.items():
            if basename.lower().endswith(suffix.lower()):
                tags.add(tag)
                break
        return tags

    # Process each symbol
    result_tags = []
    for sym in symbols:
        name = sym.get('name', '')
        kind = sym.get('kind', '')

        tags = set()
        tokens = tokenize(name)

        # Match patterns
        tags.update(match_semantic(tokens))
        tags.update(match_domain(tokens, name))
        tags.update(match_suffix(name))

        # Limit to 5 tags
        result_tags.append(list(tags)[:5])

    return jsonify({'tags': result_tags})


# ============================================================================
# Project Management Endpoints (Global Mode)
# ============================================================================

@app.route('/project/register', methods=['POST'])
def register_project():
    """Register a new project for indexing."""
    data = request.json
    project_id = data.get('id')
    name = data.get('name')
    path = data.get('path')

    if not all([project_id, name, path]):
        return jsonify({'success': False, 'error': 'Missing required fields: id, name, path'}), 400

    success, message, files = manager.register_project(project_id, name, path)

    return jsonify({
        'success': success,
        'message': message,
        'files': files
    })


@app.route('/project/<project_id>', methods=['DELETE'])
def unregister_project(project_id):
    """Unregister a project and remove its index."""
    success, message = manager.unregister_project(project_id)

    return jsonify({
        'success': success,
        'message': message
    })


@app.route('/projects')
def list_projects():
    """List all registered projects."""
    projects = []
    for pid, idx in manager.projects.items():
        projects.append({
            'id': pid,
            'name': idx.name,
            'files': len(idx.files),
            'symbols': len(idx.inverted_index)
        })

    return jsonify({
        'projects': projects,
        'global_mode': manager.global_mode
    })

@app.route('/files')
def files_search():
    start = time.time()
    pattern = request.args.get('match')
    mode = request.args.get('mode', 'recent')
    limit = int(request.args.get('limit', 50))
    project = request.args.get('project')

    idx = manager.get_local(project)
    if not idx:
        return jsonify({'error': 'No index available', 'results': [], 'ms': 0}), 404

    results = idx.list_files(pattern, mode, limit)

    return jsonify({
        'results': results,
        'index': idx.name,
        'project': project,
        'ms': (time.time() - start) * 1000
    })

@app.route('/changes')
def changes():
    start = time.time()
    since_param = request.args.get('since', '300')
    project = request.args.get('project')

    idx = manager.get_local(project)
    if not idx:
        return jsonify({'error': 'No index available', 'added': [], 'modified': [], 'deleted': [], 'ms': 0}), 404

    if since_param == 'session':
        since = idx.session_start
    else:
        since = int(time.time()) - int(since_param)

    changes_list = idx.changes_since(since)

    added = [c['file'] for c in changes_list if c['change_type'] == 'added']
    modified = [{'file': c['file'], 'lines_changed': c.get('lines_changed', [])}
                for c in changes_list if c['change_type'] == 'modified']
    deleted = [c['file'] for c in changes_list if c['change_type'] == 'deleted']

    return jsonify({
        'added': added,
        'modified': modified,
        'deleted': deleted,
        'index': idx.name,
        'project': project,
        'ms': (time.time() - start) * 1000
    })

@app.route('/file')
def file_content():
    start = time.time()
    path = request.args.get('path', '')
    lines = request.args.get('lines')
    symbol = request.args.get('symbol')

    full_path = manager.get_local().root / path
    if not full_path.exists():
        return jsonify({'error': 'File not found'}), 404

    content = full_path.read_text(encoding='utf-8', errors='ignore')
    all_lines = content.split('\n')

    if lines:
        parts = lines.split('-')
        start_l = int(parts[0]) - 1
        end_l = int(parts[1]) if len(parts) > 1 else len(all_lines)
        extracted = '\n'.join(all_lines[start_l:end_l])
        return jsonify({
            'content': extracted,
            'lines': (start_l + 1, end_l),
            'ms': (time.time() - start) * 1000
        })
    elif symbol:
        for i, line in enumerate(all_lines):
            if symbol in line:
                start_l = max(0, i - 5)
                end_l = min(len(all_lines), i + 20)
                extracted = '\n'.join(all_lines[start_l:end_l])
                return jsonify({
                    'content': extracted,
                    'lines': (start_l + 1, end_l),
                    'ms': (time.time() - start) * 1000
                })
        return jsonify({'error': 'Symbol not found'}), 404
    else:
        return jsonify({
            'content': content,
            'lines': (1, len(all_lines)),
            'ms': (time.time() - start) * 1000
        })

@app.route('/file/meta')
def file_meta():
    """Get file metadata (size, language, mtime) for baseline calculations."""
    path = request.args.get('path', '')
    project = request.args.get('project')

    idx = manager.get_local(project)
    if not idx:
        return jsonify({'error': 'No index available'}), 404

    # Look up file in index
    if path in idx.files:
        meta = idx.files[path]
        return jsonify({
            'path': path,
            'size': meta.size,
            'language': meta.language,
            'mtime': meta.mtime,
            'tokens_estimate': meta.size // 4  # Claude uses ~4 chars per token
        })
    else:
        return jsonify({'error': 'File not in index'}), 404


@app.route('/deps')
def deps():
    start = time.time()
    file = request.args.get('file')
    direction = request.args.get('direction', 'outgoing')
    local = manager.get_local()

    if not file:
        return jsonify({'error': 'file parameter required'}), 400

    with local.lock:
        if direction == 'outgoing':
            results = local.deps_outgoing.get(file, [])
        else:
            results = local.deps_incoming.get(file, [])

    return jsonify({
        'dependencies': results,
        'direction': direction,
        'ms': (time.time() - start) * 1000
    })

@app.route('/structure')
def structure():
    start = time.time()
    focus = request.args.get('focus', '')
    depth = int(request.args.get('depth', 2))
    local = manager.get_local()

    root = local.root / focus if focus else local.root

    def build_tree(path: Path, current_depth: int) -> dict:
        if current_depth > depth:
            return None

        result = {'name': path.name, 'type': 'dir' if path.is_dir() else 'file'}

        if path.is_dir():
            children = []
            try:
                for child in sorted(path.iterdir()):
                    if child.name.startswith('.'):
                        continue
                    if child.name in CodebaseIndex.IGNORE_DIRS:
                        continue
                    subtree = build_tree(child, current_depth + 1)
                    if subtree:
                        children.append(subtree)
            except PermissionError:
                pass
            result['children'] = children

        return result

    tree = build_tree(root, 0)

    return jsonify({
        'tree': tree,
        'ms': (time.time() - start) * 1000
    })


# ============================================================================
# API Endpoints - Repo Management
# ============================================================================

@app.route('/repos', methods=['GET'])
def list_repos():
    """List all knowledge repos."""
    return jsonify({
        'repos': manager.list_repos()
    })

@app.route('/repos', methods=['POST'])
def add_repo():
    """Add a new knowledge repo."""
    data = request.json
    name = data.get('name')
    url = data.get('url')

    if not name or not url:
        return jsonify({'error': 'name and url required'}), 400

    # Sanitize name
    name = re.sub(r'[^a-zA-Z0-9_-]', '', name)
    if not name:
        return jsonify({'error': 'Invalid repo name'}), 400

    success, message = manager.add_repo(name, url)

    if success:
        return jsonify({'success': True, 'message': message})
    else:
        return jsonify({'success': False, 'error': message}), 400

@app.route('/repos/<name>', methods=['DELETE'])
def remove_repo(name):
    """Remove a knowledge repo."""
    success, message = manager.remove_repo(name)

    if success:
        return jsonify({'success': True, 'message': message})
    else:
        return jsonify({'success': False, 'error': message}), 404


# ============================================================================
# API Endpoints - Repo Search (isolated)
# ============================================================================

@app.route('/repo/<name>/symbol')
def repo_symbol_search(name):
    """Search in a specific repo only."""
    start = time.time()

    repo = manager.get_repo(name)
    if not repo:
        return jsonify({'error': f"Repo '{name}' not found"}), 404

    q = request.args.get('q', '')
    mode = request.args.get('mode', 'recent')
    limit = int(request.args.get('limit', 20))

    results = repo.search(q, mode, limit)

    return jsonify({
        'results': results,
        'index': name,
        'ms': (time.time() - start) * 1000
    })

@app.route('/repo/<name>/multi', methods=['POST'])
def repo_multi_search(name):
    """Multi-term search in a specific repo only."""
    start = time.time()

    repo = manager.get_repo(name)
    if not repo:
        return jsonify({'error': f"Repo '{name}' not found"}), 404

    data = request.json
    terms = data.get('terms', [])
    mode = data.get('mode', 'recent')
    limit = int(data.get('limit', 20))

    results = repo.search_multi(terms, mode, limit)

    return jsonify({
        'results': results,
        'index': name,
        'ms': (time.time() - start) * 1000
    })

@app.route('/repo/<name>/files')
def repo_files(name):
    """List files in a specific repo."""
    start = time.time()

    repo = manager.get_repo(name)
    if not repo:
        return jsonify({'error': f"Repo '{name}' not found"}), 404

    pattern = request.args.get('match')
    mode = request.args.get('mode', 'recent')
    limit = int(request.args.get('limit', 50))

    results = repo.list_files(pattern, mode, limit)

    return jsonify({
        'results': results,
        'index': name,
        'ms': (time.time() - start) * 1000
    })

@app.route('/repo/<name>/file')
def repo_file_content(name):
    """Get file content from a specific repo."""
    start = time.time()

    repo = manager.get_repo(name)
    if not repo:
        return jsonify({'error': f"Repo '{name}' not found"}), 404

    path = request.args.get('path', '')
    lines = request.args.get('lines')

    full_path = repo.root / path
    if not full_path.exists():
        return jsonify({'error': 'File not found'}), 404

    content = full_path.read_text(encoding='utf-8', errors='ignore')
    all_lines = content.split('\n')

    if lines:
        parts = lines.split('-')
        start_l = int(parts[0]) - 1
        end_l = int(parts[1]) if len(parts) > 1 else len(all_lines)
        extracted = '\n'.join(all_lines[start_l:end_l])
        return jsonify({
            'content': extracted,
            'lines': (start_l + 1, end_l),
            'index': name,
            'ms': (time.time() - start) * 1000
        })
    else:
        return jsonify({
            'content': content,
            'lines': (1, len(all_lines)),
            'index': name,
            'ms': (time.time() - start) * 1000
        })

@app.route('/repo/<name>/deps')
def repo_deps(name):
    """Get dependencies from a specific repo."""
    start = time.time()

    repo = manager.get_repo(name)
    if not repo:
        return jsonify({'error': f"Repo '{name}' not found"}), 404

    file = request.args.get('file')
    direction = request.args.get('direction', 'outgoing')

    if not file:
        return jsonify({'error': 'file parameter required'}), 400

    with repo.lock:
        if direction == 'outgoing':
            results = repo.deps_outgoing.get(file, [])
        else:
            results = repo.deps_incoming.get(file, [])

    return jsonify({
        'dependencies': results,
        'direction': direction,
        'index': name,
        'ms': (time.time() - start) * 1000
    })


# ============================================================================
# API Endpoints - Pattern Search (Agent-driven AC)
# ============================================================================

@app.route('/pattern', methods=['POST'])
def pattern_search():
    """
    Agent-driven multi-pattern search with AC.

    Agent defines the patterns - no corpus needed.

    POST body:
    {
        "patterns": {
            "function_def": "def\\s+handleAuth\\s*\\(",
            "function_call": "handleAuth\\s*\\("
        },
        "repo": "flask",        # optional: search specific repo (default: local)
        "since": 604800,        # optional: only files modified in last N seconds
        "limit": 50             # optional: max results per pattern
    }
    """
    start = time.time()
    data = request.json

    patterns = data.get('patterns', {})
    repo_name = data.get('repo')  # None = local
    since = data.get('since')  # seconds ago
    limit = data.get('limit', 50)

    if not patterns:
        return jsonify({'error': 'patterns required'}), 400

    # Get the right index
    if repo_name:
        idx = manager.get_repo(repo_name)
        if not idx:
            return jsonify({'error': f"Repo '{repo_name}' not found"}), 404
    else:
        idx = manager.get_local()

    # Compile patterns
    compiled = {}
    for label, pattern in patterns.items():
        try:
            compiled[label] = re.compile(pattern, re.MULTILINE)
        except re.error as e:
            return jsonify({'error': f"Invalid pattern '{label}': {e}"}), 400

    # Time filter
    since_ts = None
    if since:
        since_ts = time.time() - int(since)

    # Search files
    results = {label: [] for label in patterns}
    files_searched = 0
    files_matched = 0

    with idx.lock:
        for rel_path, meta in idx.files.items():
            # Time filter
            if since_ts and meta.mtime < since_ts:
                continue

            files_searched += 1
            full_path = idx.root / rel_path

            try:
                content = full_path.read_text(encoding='utf-8', errors='ignore')
            except Exception:
                continue

            file_matched = False
            lines = content.split('\n')

            for label, regex in compiled.items():
                if len(results[label]) >= limit:
                    continue

                for match in regex.finditer(content):
                    # Find line number
                    line_start = content.count('\n', 0, match.start()) + 1
                    line_text = lines[line_start - 1].strip()[:80]

                    results[label].append({
                        'file': rel_path,
                        'line': line_start,
                        'match': match.group()[:100],
                        'context': line_text
                    })
                    file_matched = True

                    if len(results[label]) >= limit:
                        break

            if file_matched:
                files_matched += 1

    elapsed = (time.time() - start) * 1000

    return jsonify({
        'results': results,
        'stats': {
            'files_searched': files_searched,
            'files_matched': files_matched,
            'ms': elapsed
        },
        'index': repo_name or 'local'
    })


@app.route('/repo/<name>/pattern', methods=['POST'])
def repo_pattern_search(name):
    """Pattern search in a specific repo."""
    start = time.time()
    data = request.json or {}
    data['repo'] = name

    # Reuse the main pattern search
    repo = manager.get_repo(name)
    if not repo:
        return jsonify({'error': f"Repo '{name}' not found"}), 404

    patterns = data.get('patterns', {})
    since = data.get('since')
    limit = data.get('limit', 50)

    if not patterns:
        return jsonify({'error': 'patterns required'}), 400

    # Compile patterns
    compiled = {}
    for label, pattern in patterns.items():
        try:
            compiled[label] = re.compile(pattern, re.MULTILINE)
        except re.error as e:
            return jsonify({'error': f"Invalid pattern '{label}': {e}"}), 400

    since_ts = None
    if since:
        since_ts = time.time() - int(since)

    results = {label: [] for label in patterns}
    files_searched = 0
    files_matched = 0

    with repo.lock:
        for rel_path, meta in repo.files.items():
            if since_ts and meta.mtime < since_ts:
                continue

            files_searched += 1
            full_path = repo.root / rel_path

            try:
                content = full_path.read_text(encoding='utf-8', errors='ignore')
            except Exception:
                continue

            file_matched = False
            lines = content.split('\n')

            for label, regex in compiled.items():
                if len(results[label]) >= limit:
                    continue

                for match in regex.finditer(content):
                    line_start = content.count('\n', 0, match.start()) + 1
                    line_text = lines[line_start - 1].strip()[:80]

                    results[label].append({
                        'file': rel_path,
                        'line': line_start,
                        'match': match.group()[:100],
                        'context': line_text
                    })
                    file_matched = True

                    if len(results[label]) >= limit:
                        break

            if file_matched:
                files_matched += 1

    elapsed = (time.time() - start) * 1000

    return jsonify({
        'results': results,
        'stats': {
            'files_searched': files_searched,
            'files_matched': files_matched,
            'ms': elapsed
        },
        'index': name
    })


# ============================================================================
# API Endpoints - Intent Tracking
# ============================================================================

@app.route('/intent', methods=['POST'])
def record_intent():
    """
    Record an intent from tool usage.

    POST body:
    {
        "tool": "Edit",
        "files": ["/path/to/file.py"],
        "tags": ["#authentication", "#editing"],
        "session_id": "abc123",
        "tool_use_id": "toolu_xxx",  # Claude's correlation key
        "project_id": "uuid-here"    # Per-project isolation
    }
    """
    data = request.json

    tool = data.get('tool', 'unknown')
    files = data.get('files', [])
    tags = data.get('tags', [])
    session_id = data.get('session_id', 'unknown')
    tool_use_id = data.get('tool_use_id')  # Claude's toolu_xxx ID
    project_id = data.get('project_id')  # UUID for per-project isolation
    file_sizes = data.get('file_sizes', {})  # File path -> size for baseline calc
    output_size = data.get('output_size')  # Actual output size for REAL savings calc

    intent_index.record(tool, files, tags, session_id, tool_use_id, project_id, file_sizes, output_size)

    return jsonify({'success': True})


@app.route('/intent/tags')
def intent_tags():
    """Get all intent tags with counts."""
    project_id = request.args.get('project_id')
    tags = intent_index.all_tags(project_id)
    return jsonify({
        'tags': [{'tag': t, 'count': c} for t, c in tags],
        'project_id': project_id
    })


@app.route('/intent/files')
def intent_files_for_tag():
    """Get files associated with a tag."""
    tag = request.args.get('tag', '')
    project_id = request.args.get('project_id')
    if not tag.startswith('#'):
        tag = '#' + tag

    files = intent_index.files_for_tag(tag, project_id)
    return jsonify({
        'tag': tag,
        'files': files,
        'project_id': project_id
    })


@app.route('/intent/file')
def intent_tags_for_file():
    """Get tags associated with a file."""
    file = request.args.get('path', '')
    project_id = request.args.get('project_id')
    tags = intent_index.tags_for_file(file, project_id)
    return jsonify({
        'file': file,
        'tags': tags,
        'project_id': project_id
    })


@app.route('/intent/recent')
def intent_recent():
    """Get recent intent records."""
    since = request.args.get('since')
    limit = int(request.args.get('limit', 50))
    project_id = request.args.get('project_id')

    since_ts = None
    if since:
        since_ts = int(time.time()) - int(since)

    records = intent_index.recent(since_ts, limit, project_id)
    return jsonify({
        'records': records,
        'stats': intent_index.get_stats(project_id)
    })


@app.route('/intent/session')
def intent_session():
    """Get intent records for a session."""
    session_id = request.args.get('id', '')
    project_id = request.args.get('project_id')
    records = intent_index.session(session_id, project_id)
    return jsonify({
        'session_id': session_id,
        'records': records,
        'project_id': project_id
    })


@app.route('/intent/stats')
def intent_stats():
    """Get intent index statistics."""
    project_id = request.args.get('project_id')
    return jsonify(intent_index.get_stats(project_id))


@app.route('/metrics/token-rate')
def metrics_token_rate():
    """
    Calculate actual ms_per_token rate from session history.

    Derives the real processing rate from Claude session timestamps and token counts.
    This rate can be used to estimate time savings from token savings.

    Returns:
        ms_per_token: Median milliseconds per token processed
        range: Min/max/p25/p75 for showing variability
        samples: Number of data points analyzed
        confidence: high (50+), medium (20+), or low (<20)
        methodology: How the rate was calculated
    """
    from services.ranking.session_parser import SessionLogParser
    from pathlib import Path
    import os

    # Find Claude projects directory
    home = os.path.expanduser('~')
    projects_dir = Path(home) / '.claude' / 'projects'

    if not projects_dir.exists():
        return jsonify({
            'ms_per_token': 0,
            'samples': 0,
            'confidence': 'none',
            'error': 'No Claude projects directory found'
        })

    # Get the most recent project directory
    project_dirs = sorted(projects_dir.iterdir(), key=lambda p: p.stat().st_mtime, reverse=True)
    if not project_dirs:
        return jsonify({
            'ms_per_token': 0,
            'samples': 0,
            'confidence': 'none',
            'error': 'No session data found'
        })

    # Use the most recent project for rate calculation
    parser = SessionLogParser(project_dirs[0])
    rate_data = parser.calculate_token_rate()

    return jsonify(rate_data)


# ============================================================================
# Prediction Tracking API - Phase 2 Session Correlation + Phase 4 Rolling Metrics
# ============================================================================

# Rolling window constants for Hit@5 calculation
ROLLING_WINDOW_HOURS = 24
ROLLING_WINDOW_SECONDS = ROLLING_WINDOW_HOURS * 3600

@app.route('/predict/log', methods=['POST'])
def log_prediction():
    """
    Log a prediction for later hit/miss comparison.

    POST body:
    {
        "session_id": "uuid-xxx",
        "predicted_files": ["/src/file1.py", "/src/file2.py"],
        "tags": ["python", "api"],
        "trigger_file": "/src/current.py",
        "confidence": 0.85
    }

    Phase 4: Also logs to rolling ZSET for Hit@5 calculation over 24h window.
    """
    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Redis not available'}), 503

    data = request.json
    session_id = data.get('session_id', 'unknown')
    predicted_files = data.get('predicted_files', [])
    tags = data.get('tags', [])
    trigger_file = data.get('trigger_file', '')
    confidence = data.get('confidence', 0.0)

    if not predicted_files:
        return jsonify({'success': True, 'logged': 0})

    try:
        # Store prediction in Redis with TTL (60 seconds - predictions expire)
        import time as time_module
        timestamp = time_module.time()
        timestamp_ms = int(timestamp * 1000)
        prediction_key = f"aoa:prediction:{session_id}:{timestamp_ms}"

        prediction_data = {
            'session_id': session_id,
            'timestamp_ms': timestamp_ms,
            'predicted_files': predicted_files,
            'tags': tags,
            'trigger_file': trigger_file,
            'confidence': confidence,
            'hit': None  # Will be set by /predict/check
        }

        # Store prediction with 60s TTL (for quick lookup during active session)
        scorer.redis.client.setex(
            prediction_key,
            60,  # 60 second TTL
            json.dumps(prediction_data)
        )

        # Also add to session's prediction list for quick lookup
        session_predictions_key = f"aoa:predictions:{session_id}"
        scorer.redis.client.lpush(session_predictions_key, prediction_key)
        scorer.redis.client.expire(session_predictions_key, 3600)  # 1 hour TTL for session

        # Phase 4: Add to rolling predictions ZSET for Hit@5 calculation
        # Score = timestamp, Member = prediction_id
        # This persists beyond the 60s TTL for rolling metrics
        rolling_key = "aoa:rolling:predictions"
        scorer.redis.client.zadd(rolling_key, {prediction_key: timestamp})

        # Store prediction data in a hash that persists for rolling window
        rolling_data_key = f"aoa:rolling:data:{prediction_key}"
        scorer.redis.client.hset(rolling_data_key, mapping={
            'session_id': session_id,
            'timestamp': str(timestamp),
            'predicted_files': json.dumps(predicted_files[:5]),  # Top 5 for Hit@5
            'hit': '',  # Empty = not yet evaluated
        })
        scorer.redis.client.expire(rolling_data_key, ROLLING_WINDOW_SECONDS + 3600)  # 25h TTL

        # Cleanup: Remove predictions older than rolling window
        cutoff = timestamp - ROLLING_WINDOW_SECONDS
        scorer.redis.client.zremrangebyscore(rolling_key, 0, cutoff)

        return jsonify({
            'success': True,
            'logged': len(predicted_files),
            'prediction_key': prediction_key
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/predict/check', methods=['POST'])
def check_prediction_hit():
    """
    Check if a file access was predicted (called by intent-capture after Read).

    POST body:
    {
        "session_id": "uuid-xxx",
        "file": "/src/file.py"
    }

    Returns whether this file was in recent predictions.

    Phase 4: Also updates rolling data for Hit@5 calculation.
    A prediction batch is a "hit" if ANY of the top 5 files were read.
    """
    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'hit': False, 'error': 'Redis not available'}), 503

    data = request.json
    session_id = data.get('session_id', 'unknown')
    file_path = data.get('file', '')
    project_id = data.get('project_id', '')  # UUID for per-project metrics

    if not file_path:
        return jsonify({'hit': False})

    try:
        # Get recent predictions for this session
        session_predictions_key = f"aoa:predictions:{session_id}"
        prediction_keys = scorer.redis.client.lrange(session_predictions_key, 0, 10)

        for pred_key in prediction_keys:
            pred_key_str = pred_key.decode() if isinstance(pred_key, bytes) else pred_key
            pred_data = scorer.redis.client.get(pred_key_str)
            if pred_data:
                prediction = json.loads(pred_data)
                if file_path in prediction.get('predicted_files', []):
                    # Record the hit - global (system monitoring)
                    scorer.redis.client.incr('aoa:metrics:hits')

                    # Record per-project hit count (NOT fabricated savings)
                    # Real savings are calculated when we have both baseline + actual output
                    if project_id:
                        scorer.redis.client.incr(f'aoa:{project_id}:metrics:hits')

                    # Phase 4: Mark the prediction batch as a hit in rolling data
                    rolling_data_key = f"aoa:rolling:data:{pred_key_str}"
                    current_hit = scorer.redis.client.hget(rolling_data_key, 'hit')
                    if current_hit is not None:
                        # Only mark as hit if not already evaluated
                        current_hit_str = current_hit.decode() if isinstance(current_hit, bytes) else current_hit
                        if current_hit_str == '':
                            scorer.redis.client.hset(rolling_data_key, 'hit', '1')

                    return jsonify({
                        'hit': True,
                        'prediction_key': pred_key_str,
                        'confidence': prediction.get('confidence', 0)
                    })

        # No hit - record miss (global)
        scorer.redis.client.incr('aoa:metrics:misses')
        if project_id:
            scorer.redis.client.incr(f'aoa:{project_id}:metrics:misses')

        # Phase 4: Mark any unevaluated predictions as misses after a file read
        # (This is conservative - we only mark miss if we checked and didn't find a hit)
        # Note: We don't mark as miss here because the user might still read a predicted file later

        return jsonify({'hit': False})

    except Exception as e:
        return jsonify({'hit': False, 'error': str(e)}), 500


@app.route('/predict/stats')
def prediction_stats():
    """
    Get prediction hit/miss statistics.

    Phase 4: Includes rolling Hit@5 over 24h window.
    """
    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Redis not available'}), 503

    project_id = request.args.get('project_id')

    try:
        # Legacy cumulative counters (per-project if project_id provided)
        if project_id:
            hits = int(scorer.redis.client.get(f'aoa:{project_id}:metrics:hits') or 0)
            misses = int(scorer.redis.client.get(f'aoa:{project_id}:metrics:misses') or 0)
        else:
            hits = int(scorer.redis.client.get('aoa:metrics:hits') or 0)
            misses = int(scorer.redis.client.get('aoa:metrics:misses') or 0)

        total = hits + misses
        hit_rate = (hits / total * 100) if total > 0 else 0

        # Phase 4: Calculate rolling Hit@5 over 24h window
        rolling_stats = calculate_rolling_hit_rate()

        return jsonify({
            # Legacy stats
            'hits': hits,
            'misses': misses,
            'total': total,
            'hit_rate': round(hit_rate, 1),
            # Phase 4 rolling stats
            'rolling': rolling_stats,
            'project_id': project_id
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


def calculate_rolling_hit_rate(window_hours: int = 24) -> dict:
    """
    Calculate Hit@5 over a rolling time window.

    Hit@5 = (prediction batches with at least 1 hit) / (total evaluated batches)

    Returns:
        dict with:
        - window_hours: The time window
        - total_predictions: Number of predictions in window
        - evaluated: Number of predictions that have been evaluated
        - hits: Number of prediction batches with at least 1 hit
        - hit_at_5: Hit@5 rate (0.0 to 1.0)
        - hit_at_5_pct: Hit@5 as percentage (0 to 100)
    """
    import time as time_module

    if not RANKING_AVAILABLE or scorer is None:
        return {'error': 'Redis not available'}

    try:
        now = time_module.time()
        window_start = now - (window_hours * 3600)

        # Get all predictions in the rolling window
        rolling_key = "aoa:rolling:predictions"
        prediction_keys = scorer.redis.client.zrangebyscore(
            rolling_key, window_start, now
        )

        total_predictions = len(prediction_keys)
        evaluated = 0
        hits = 0
        misses = 0

        for pred_key in prediction_keys:
            pred_key_str = pred_key.decode() if isinstance(pred_key, bytes) else pred_key
            rolling_data_key = f"aoa:rolling:data:{pred_key_str}"

            hit_value = scorer.redis.client.hget(rolling_data_key, 'hit')
            if hit_value is not None:
                hit_str = hit_value.decode() if isinstance(hit_value, bytes) else hit_value
                if hit_str == '1':
                    hits += 1
                    evaluated += 1
                elif hit_str == '0':
                    misses += 1
                    evaluated += 1
                # Empty string means not yet evaluated

        hit_at_5 = hits / evaluated if evaluated > 0 else 0.0

        return {
            'window_hours': window_hours,
            'total_predictions': total_predictions,
            'evaluated': evaluated,
            'pending': total_predictions - evaluated,
            'hits': hits,
            'misses': misses,
            'hit_at_5': round(hit_at_5, 4),
            'hit_at_5_pct': round(hit_at_5 * 100, 1),
        }

    except Exception as e:
        return {'error': str(e)}


@app.route('/predict/finalize', methods=['POST'])
def finalize_predictions():
    """
    Finalize stale predictions as misses.

    Predictions older than `max_age_seconds` (default 300 = 5 minutes) that
    haven't been marked as hits are marked as misses.

    POST body (optional):
    {
        "max_age_seconds": 300
    }

    Returns count of predictions finalized.
    """
    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Redis not available'}), 503

    import time as time_module

    data = request.json or {}
    max_age_seconds = data.get('max_age_seconds', 300)  # 5 minutes default

    try:
        now = time_module.time()
        cutoff = now - max_age_seconds

        # Get predictions older than max_age that haven't been evaluated
        rolling_key = "aoa:rolling:predictions"
        stale_keys = scorer.redis.client.zrangebyscore(
            rolling_key, 0, cutoff
        )

        finalized = 0
        for pred_key in stale_keys:
            pred_key_str = pred_key.decode() if isinstance(pred_key, bytes) else pred_key
            rolling_data_key = f"aoa:rolling:data:{pred_key_str}"

            hit_value = scorer.redis.client.hget(rolling_data_key, 'hit')
            if hit_value is not None:
                hit_str = hit_value.decode() if isinstance(hit_value, bytes) else hit_value
                if hit_str == '':
                    # Not yet evaluated - mark as miss
                    scorer.redis.client.hset(rolling_data_key, 'hit', '0')
                    finalized += 1

        return jsonify({
            'finalized': finalized,
            'checked': len(stale_keys),
            'max_age_seconds': max_age_seconds
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


# ============================================================================
# Weight Tuner API - Phase 4 Thompson Sampling
# ============================================================================

@app.route('/tuner/weights')
def tuner_weights():
    """
    Get current optimized weights via Thompson Sampling.

    Each call samples from the Beta distributions and returns the best arm.
    Use for exploration (learning which weights work best).

    Returns:
        {
            "weights": {"recency": 0.4, "frequency": 0.3, "tag": 0.3},
            "arm_idx": 2,
            "arm_name": "default"
        }
    """
    if tuner is None:
        return jsonify({'error': 'Tuner not available'}), 503

    try:
        weights = tuner.select_weights()
        arm_idx = weights.pop('_arm_idx', 0)
        arm = tuner.ARMS[arm_idx]

        return jsonify({
            'weights': weights,
            'arm_idx': arm_idx,
            'arm_name': arm.get('name', f'arm-{arm_idx}')
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/tuner/best')
def tuner_best():
    """
    Get the best performing weights (exploitation only, no exploration).

    Returns the arm with highest mean success rate.
    Use for production predictions once you have enough data.

    Returns:
        {
            "weights": {"recency": 0.5, "frequency": 0.3, "tag": 0.2},
            "arm_idx": 0,
            "mean": 0.78
        }
    """
    if tuner is None:
        return jsonify({'error': 'Tuner not available'}), 503

    try:
        best = tuner.get_best_weights()
        arm_idx = best.pop('_arm_idx', 0)
        mean = best.pop('_mean', 0.5)
        arm = tuner.ARMS[arm_idx]

        return jsonify({
            'weights': best,
            'arm_idx': arm_idx,
            'arm_name': arm.get('name', f'arm-{arm_idx}'),
            'mean': round(mean, 4)
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/tuner/stats')
def tuner_stats():
    """
    Get statistics for all arms, sorted by mean success rate.

    Returns:
        {
            "arms": [
                {"arm_idx": 0, "name": "recency-heavy", "mean": 0.78, ...},
                ...
            ],
            "total_samples": 150
        }
    """
    if tuner is None:
        return jsonify({'error': 'Tuner not available'}), 503

    try:
        stats = tuner.get_stats()
        total_samples = sum(arm['samples'] for arm in stats)

        return jsonify({
            'arms': stats,
            'total_samples': total_samples
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/tuner/feedback', methods=['POST'])
def tuner_feedback():
    """
    Record hit/miss feedback for a specific arm.

    POST body:
    {
        "arm_idx": 2,
        "hit": true
    }

    Returns confirmation of the update.
    """
    if tuner is None:
        return jsonify({'error': 'Tuner not available'}), 503

    data = request.json or {}
    arm_idx = data.get('arm_idx')
    hit = data.get('hit', False)

    if arm_idx is None:
        return jsonify({'error': 'arm_idx required'}), 400

    try:
        tuner.record_feedback(hit=hit, arm_idx=arm_idx)

        # Get updated stats for this arm
        alpha, beta = tuner._get_arm_stats(arm_idx)

        return jsonify({
            'success': True,
            'arm_idx': arm_idx,
            'hit': hit,
            'new_alpha': alpha,
            'new_beta': beta,
            'new_mean': round(alpha / (alpha + beta), 4)
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/tuner/reset', methods=['POST'])
def tuner_reset():
    """
    Reset all arm statistics to priors.

    Use with caution - this erases all learned data.
    """
    if tuner is None:
        return jsonify({'error': 'Tuner not available'}), 503

    try:
        tuner.reset()
        return jsonify({'success': True, 'message': 'All arms reset to priors'})
    except Exception as e:
        return jsonify({'error': str(e)}), 500


# ============================================================================
# Metrics API - Phase 4 Unified Accuracy Dashboard
# ============================================================================

@app.route('/metrics')
def get_metrics():
    """
    Unified metrics endpoint showing accuracy, tuner performance, and trends.

    Query params:
        project_id: UUID for per-project metrics (optional, for future per-project support)

    Returns:
        {
            "hit_at_5": 0.72,
            "hit_at_5_pct": 72.0,
            "target": 0.90,
            "gap": 0.18,
            "trend": "improving",

            "rolling": {
                "window_hours": 24,
                "total_predictions": 150,
                "evaluated": 120,
                "hits": 86,
                "hit_at_5": 0.72
            },

            "tuner": {
                "best_arm": "recency-heavy",
                "best_weights": {"recency": 0.5, ...},
                "best_mean": 0.78,
                "total_samples": 150
            },

            "legacy": {
                "hits": 200,
                "misses": 100,
                "hit_rate": 66.7
            }
        }
    """
    # Accept project_id for future per-project metrics support
    # TODO: Implement per-project Redis key prefixing for metrics
    project_id = request.args.get('project_id')

    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Ranking not available'}), 503

    try:
        # Get rolling stats
        rolling = calculate_rolling_hit_rate()

        # Get tuner stats
        tuner_stats = {}
        if tuner is not None:
            best = tuner.get_best_weights()
            arm_idx = best.pop('_arm_idx', 0)
            mean = best.pop('_mean', 0.5)
            arm = tuner.ARMS[arm_idx]
            all_stats = tuner.get_stats()

            tuner_stats = {
                'best_arm': arm.get('name', f'arm-{arm_idx}'),
                'best_arm_idx': arm_idx,
                'best_weights': best,
                'best_mean': round(mean, 4),
                'total_samples': sum(a['samples'] for a in all_stats),
            }

        # Legacy cumulative stats (per-project if project_id provided, else global)
        # Note: tokens_saved and time_saved_ms are DEPRECATED - they were fabricated estimates
        # Real savings require capturing actual output tokens (Phase 2)
        if project_id:
            hits = int(scorer.redis.client.get(f'aoa:{project_id}:metrics:hits') or 0)
            misses = int(scorer.redis.client.get(f'aoa:{project_id}:metrics:misses') or 0)
            # DEPRECATED: These were fake hardcoded estimates (1500 tokens/hit, 50ms/hit)
            # Real savings will be tracked via intent records with baseline + actual output
            tokens_saved = int(scorer.redis.client.get(f'aoa:{project_id}:savings:tokens:real') or 0)
            time_saved_ms = 0  # Not tracked yet
        else:
            hits = int(scorer.redis.client.get('aoa:metrics:hits') or 0)
            misses = int(scorer.redis.client.get('aoa:metrics:misses') or 0)
            # DEPRECATED: These were fake hardcoded estimates
            tokens_saved = int(scorer.redis.client.get('aoa:savings:tokens:real') or 0)
            time_saved_ms = 0  # Not tracked yet

        total = hits + misses
        legacy_rate = (hits / total * 100) if total > 0 else 0

        # Calculate main metrics
        hit_at_5 = rolling.get('hit_at_5', 0.0)
        target = 0.90

        # Determine trend (would need historical data for real trend)
        # For now, compare to legacy rate
        if rolling.get('evaluated', 0) > 10:
            if hit_at_5 > (legacy_rate / 100) + 0.05:
                trend = 'improving'
            elif hit_at_5 < (legacy_rate / 100) - 0.05:
                trend = 'declining'
            else:
                trend = 'stable'
        else:
            trend = 'insufficient_data'

        # Get real savings from intent index (file_size vs output_size measurements)
        intent_savings = {}
        if intent_index:
            intent_stats = intent_index.get_stats(project_id)
            intent_savings = intent_stats.get('savings', {})

        # Total savings = Redis + Intent (Intent is the primary/real source)
        total_tokens_saved = tokens_saved + intent_savings.get('tokens', 0)
        # Get time_sec from intent_savings (calculated at 7.5ms per token)
        intent_time_sec = intent_savings.get('time_sec', 0)
        savings_data = {
            'tokens': total_tokens_saved,
            'baseline': intent_savings.get('baseline', 0),
            'actual': intent_savings.get('actual', 0),
            'measured_records': intent_savings.get('measured_records', 0),
            'time_sec': intent_time_sec,  # Estimated from token savings
        }

        return jsonify({
            # Primary metrics
            'hit_at_5': hit_at_5,
            'hit_at_5_pct': rolling.get('hit_at_5_pct', 0.0),
            'target': target,
            'target_pct': target * 100,
            'gap': round(target - hit_at_5, 4),
            'trend': trend,

            # Detailed rolling stats
            'rolling': rolling,

            # Tuner stats
            'tuner': tuner_stats,

            # Legacy stats (cumulative)
            'legacy': {
                'hits': hits,
                'misses': misses,
                'total': total,
                'hit_rate': round(legacy_rate, 1),
            },

            # Savings (cumulative) - include intent index real measurements
            'savings': savings_data
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/metrics/tokens')
def get_token_metrics():
    """
    Get token usage and cost statistics from Claude session logs.

    Returns:
        {
            "input_tokens": 1234567,
            "output_tokens": 234567,
            "cache_read_tokens": 567890,
            "total_tokens": 1469134,
            "message_count": 150,

            "cost": {
                "input": 18.52,
                "output": 17.59,
                "total": 36.11
            },

            "savings": {
                "from_cache": 7.65,
                "cache_hit_rate": 0.42
            }
        }
    """
    try:
        from ranking.session_parser import SessionLogParser

        # Get project path from environment
        import os
        project_path = os.environ.get('CODEBASE_ROOT', '/home/corey/aOa')

        parser = SessionLogParser(project_path)
        token_stats = parser.get_token_usage()

        return jsonify(token_stats)

    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/predict')
def predict_files():
    """
    Get predicted files with optional snippet prefetch.

    This is the main prediction endpoint for P2-005.
    Returns ranked files with first N lines of each file for context injection.

    Query params:
        tags: Comma-separated tags to filter/boost by (optional)
        keywords: Comma-separated keywords from prompt (optional, treated as tags)
        limit: Maximum files to return (default: 5)
        snippet_lines: Number of lines to prefetch per file (default: 20, 0 to disable)
        file: Trigger file for co-occurrence lookup (optional)

    Returns:
        {
            "files": [
                {
                    "path": "/src/api/routes.py",
                    "confidence": 0.85,
                    "snippet": "first 20 lines..."
                }
            ],
            "predictions": ["/src/api/routes.py", ...],  # Simple list for backward compat
            "ms": 4.2
        }
    """
    start = time.time()

    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({
            'error': 'Ranking module not available',
            'files': [],
            'predictions': [],
            'ms': (time.time() - start) * 1000
        }), 503

    # Parse parameters
    tag_param = request.args.get('tags', request.args.get('tag', ''))
    keyword_param = request.args.get('keywords', '')
    file_param = request.args.get('file', '')

    # Combine tags and keywords
    tags = [t.strip().lstrip('#') for t in tag_param.split(',') if t.strip()]
    keywords = [k.strip() for k in keyword_param.split(',') if k.strip()]
    all_tags = list(set(tags + keywords))

    limit = int(request.args.get('limit', 5))
    snippet_lines = int(request.args.get('snippet_lines', 20))

    try:
        # Get ranked files from scorer
        results = scorer.get_ranked_files(tags=all_tags if all_tags else None, limit=limit * 2)

        # Get transition predictions if trigger file provided
        transition_preds = {}
        transition_boost = 0.0
        project_root = os.environ.get('CODEBASE_ROOT', '/codebase')
        host_root = '/home/corey/aOa'

        if file_param and SESSION_PARSER_AVAILABLE:
            try:
                # Try with the file param as-is first
                trans_results = SessionLogParser.predict_next(scorer.redis, file_param, limit=10)

                # If no results, try normalizing the path
                if not trans_results and file_param.startswith(host_root):
                    normalized = file_param[len(host_root) + 1:]  # Remove /home/corey/aOa/
                    trans_results = SessionLogParser.predict_next(scorer.redis, normalized, limit=10)
                elif not trans_results and file_param.startswith(project_root):
                    normalized = file_param[len(project_root) + 1:]  # Remove /codebase/
                    trans_results = SessionLogParser.predict_next(scorer.redis, normalized, limit=10)

                # Store predictions with both absolute and relative paths for matching
                for f, prob in trans_results:
                    transition_preds[f] = prob
                    # Also store absolute path variant
                    if not f.startswith('/'):
                        transition_preds[os.path.join(host_root, f)] = prob

                transition_boost = 0.3  # Boost factor for transition matches
            except Exception:
                pass  # Transitions are optional enhancement

        # Build response with snippets
        files = []
        seen_paths = set()

        for r in results:
            file_path = r['file']
            # Use calibrated confidence from scorer (P2-001)
            # Falls back to normalized score for backward compatibility
            confidence = r.get('confidence', min(r.get('score', 0.0) / 100.0, 1.0))

            # Boost confidence if file is also predicted by transitions
            if file_path in transition_preds:
                trans_prob = transition_preds[file_path]
                confidence = min(1.0, confidence + trans_prob * transition_boost)

            file_data = {
                'path': file_path,
                'confidence': round(confidence, 3)
            }

            # Read snippet if requested
            if snippet_lines > 0:
                snippet = read_file_snippet(file_path, snippet_lines)
                if snippet:
                    file_data['snippet'] = snippet

            files.append(file_data)
            seen_paths.add(file_path)

            if len(files) >= limit:
                break

        # Add high-probability transition predictions not in scorer results
        if transition_preds and len(files) < limit:
            for trans_file, trans_prob in sorted(transition_preds.items(),
                                                  key=lambda x: x[1], reverse=True):
                if trans_file not in seen_paths and trans_prob >= 0.1:
                    file_data = {
                        'path': trans_file,
                        'confidence': round(trans_prob * 0.8, 3),  # Scale down since not in scorer
                        'source': 'transition'
                    }
                    if snippet_lines > 0:
                        snippet = read_file_snippet(trans_file, snippet_lines)
                        if snippet:
                            file_data['snippet'] = snippet
                    files.append(file_data)
                    if len(files) >= limit:
                        break

        # Re-sort by confidence
        files.sort(key=lambda x: x['confidence'], reverse=True)
        files = files[:limit]

        return jsonify({
            'files': files,
            'predictions': [f['path'] for f in files],  # Backward compat
            'tags_used': all_tags,
            'trigger_file': file_param if file_param else None,
            'transition_matches': len([f for f in files if f['path'] in transition_preds]),
            'ms': round((time.time() - start) * 1000, 2)
        })

    except Exception as e:
        return jsonify({
            'error': str(e),
            'files': [],
            'predictions': [],
            'ms': (time.time() - start) * 1000
        }), 500


def read_file_snippet(file_path: str, max_lines: int = 20) -> str:
    """
    Read first N lines of a file for snippet prefetch.

    Returns empty string if file doesn't exist or can't be read.
    Handles common text files, skips binary files.
    """
    import os

    # Translate host paths to container paths
    # File paths in Redis are stored as /home/corey/aOa/... but in container they're at /codebase/...
    CODEBASE_ROOT = os.environ.get('CODEBASE_ROOT', '/codebase')
    HOST_PATH_PREFIX = '/home/corey/aOa'

    if file_path.startswith(HOST_PATH_PREFIX):
        file_path = file_path.replace(HOST_PATH_PREFIX, CODEBASE_ROOT, 1)

    # Resolve to absolute path if needed
    if not os.path.isabs(file_path):
        # Try common base paths
        for base in [CODEBASE_ROOT, os.getcwd()]:
            full_path = os.path.join(base, file_path)
            if os.path.exists(full_path):
                file_path = full_path
                break

    if not os.path.exists(file_path):
        return ''

    # Skip binary files by extension
    binary_exts = {'.pyc', '.so', '.o', '.a', '.exe', '.dll', '.bin', '.dat',
                   '.png', '.jpg', '.jpeg', '.gif', '.ico', '.pdf', '.zip', '.tar', '.gz'}
    _, ext = os.path.splitext(file_path)
    if ext.lower() in binary_exts:
        return ''

    try:
        with open(file_path, 'r', encoding='utf-8', errors='ignore') as f:
            lines = []
            for i, line in enumerate(f):
                if i >= max_lines:
                    break
                # Truncate very long lines
                if len(line) > 500:
                    line = line[:500] + '...\n'
                lines.append(line)
            return ''.join(lines)
    except (IOError, OSError):
        return ''


# ============================================================================
# Ranking API - Predictive File Scoring
# ============================================================================

# Global scorer and tuner instances
scorer = None
tuner = None  # Phase 4: Thompson Sampling weight tuner


@app.route('/rank')
def rank_files():
    """
    Get files ranked by composite score (recency + frequency + tag affinity).

    Query params:
        tag: Comma-separated tags to filter/boost by (optional)
        limit: Maximum files to return (default: 10)
        db: Redis database number (for testing, optional)

    Returns:
        {
            "files": ["/src/api/routes.py", ...],
            "details": [{"file": "...", "score": 0.85, ...}, ...],
            "ms": 4.2
        }
    """
    start = time.time()

    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({
            'error': 'Ranking module not available',
            'files': [],
            'details': [],
            'ms': (time.time() - start) * 1000
        }), 503

    # Parse parameters
    tag_param = request.args.get('tag', '')
    tags = [t.strip().lstrip('#') for t in tag_param.split(',') if t.strip()]
    limit = int(request.args.get('limit', 10))
    db = request.args.get('db')
    db = int(db) if db else None

    # Get ranked files
    try:
        results = scorer.get_ranked_files(tags=tags if tags else None, limit=limit, db=db)

        return jsonify({
            'files': [r['file'] for r in results],
            'details': results,
            'tags_used': tags,
            'ms': round((time.time() - start) * 1000, 2)
        })
    except Exception as e:
        return jsonify({
            'error': str(e),
            'files': [],
            'details': [],
            'ms': (time.time() - start) * 1000
        }), 500


@app.route('/rank/stats')
def rank_stats():
    """Get ranking system statistics."""
    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Ranking module not available'}), 503

    return jsonify(scorer.get_stats())


@app.route('/rank/record', methods=['POST'])
def rank_record():
    """
    Record a file access for scoring.

    POST body:
        {
            "file": "/src/api/routes.py",
            "tags": ["api", "python"]
        }
    """
    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Ranking module not available'}), 503

    data = request.json or {}
    file_path = data.get('file')
    tags = data.get('tags', [])

    if not file_path:
        return jsonify({'error': 'file parameter required'}), 400

    scores = scorer.record_access(file_path, tags=tags)
    return jsonify({
        'recorded': file_path,
        'scores': scores
    })


# ============================================================================
# Transition Model API - Phase 3 Session Log Learning
# ============================================================================

# Global session parser instance
session_parser = None

try:
    from ranking.session_parser import SessionLogParser
    SESSION_PARSER_AVAILABLE = True
except ImportError:
    SESSION_PARSER_AVAILABLE = False


@app.route('/transitions/sync', methods=['POST'])
def sync_transitions():
    """
    Sync file transitions from Claude session logs to Redis.

    This parses ~/.claude/projects/*/agent-*.jsonl to extract
    file access patterns and store transition probabilities in Redis.

    POST body (optional):
    {
        "project_path": "/home/corey/aOa"  # default
    }

    Returns:
    {
        "keys_written": 57,
        "total_transitions": 94,
        "stats": {...}
    }
    """
    global session_parser

    if not SESSION_PARSER_AVAILABLE:
        return jsonify({'error': 'Session parser not available'}), 503

    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Redis not available'}), 503

    start = time.time()
    data = request.json or {}
    project_path = data.get('project_path', '/home/corey/aOa')

    try:
        session_parser = SessionLogParser(project_path)
        stats = session_parser.get_stats()
        result = session_parser.sync_to_redis(scorer.redis)

        return jsonify({
            'success': True,
            'keys_written': result['keys_written'],
            'total_transitions': result['total_transitions'],
            'stats': stats,
            'ms': round((time.time() - start) * 1000, 2)
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/transitions/predict')
def predict_from_transitions():
    """
    Get predicted next files based on transition model.

    Query params:
        file: Current file being accessed (required)
        limit: Maximum predictions to return (default: 5)

    Returns:
    {
        "predictions": [
            {"file": ".context/BOARD.md", "probability": 0.7},
            {"file": "src/hooks/intent-prefetch.py", "probability": 0.1}
        ],
        "source_file": ".context/CURRENT.md",
        "ms": 1.2
    }
    """
    if not SESSION_PARSER_AVAILABLE:
        return jsonify({'error': 'Session parser not available'}), 503

    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Redis not available'}), 503

    start = time.time()
    current_file = request.args.get('file', '')
    limit = int(request.args.get('limit', 5))

    if not current_file:
        return jsonify({'error': 'file parameter required'}), 400

    try:
        predictions = SessionLogParser.predict_next(
            scorer.redis, current_file, limit=limit
        )

        return jsonify({
            'predictions': [
                {'file': f, 'probability': round(p, 4)}
                for f, p in predictions
            ],
            'source_file': current_file,
            'ms': round((time.time() - start) * 1000, 2)
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/transitions/stats')
def transition_stats():
    """
    Get statistics about the transition model.

    Returns session parsing stats and Redis key counts.
    """
    if not SESSION_PARSER_AVAILABLE:
        return jsonify({'error': 'Session parser not available'}), 503

    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Redis not available'}), 503

    try:
        # Count transition keys in Redis
        transition_keys = scorer.redis.keys('aoa:transition:*')

        # Get session parser stats if initialized
        parser_stats = None
        if session_parser:
            parser_stats = session_parser.get_stats()

        return jsonify({
            'transition_keys': len(transition_keys),
            'parser_stats': parser_stats
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


# ============================================================================
# Context API - Natural Language Intent to Files (P3-003, P3-004)
# ============================================================================

# Stopwords for keyword extraction
STOPWORDS = {
    'the', 'a', 'an', 'is', 'are', 'was', 'were', 'be', 'been', 'being',
    'have', 'has', 'had', 'do', 'does', 'did', 'will', 'would', 'could',
    'should', 'may', 'might', 'must', 'shall', 'can', 'need', 'dare',
    'to', 'of', 'in', 'for', 'on', 'with', 'at', 'by', 'from', 'as',
    'into', 'through', 'during', 'before', 'after', 'above', 'below',
    'between', 'under', 'again', 'further', 'then', 'once', 'here',
    'there', 'when', 'where', 'why', 'how', 'all', 'each', 'few',
    'more', 'most', 'other', 'some', 'such', 'no', 'nor', 'not',
    'only', 'own', 'same', 'so', 'than', 'too', 'very', 'just',
    'and', 'but', 'if', 'or', 'because', 'until', 'while', 'this',
    'that', 'these', 'those', 'what', 'which', 'who', 'whom',
    'i', 'you', 'he', 'she', 'it', 'we', 'they', 'me', 'him', 'her',
    'us', 'them', 'my', 'your', 'his', 'its', 'our', 'their',
    'fix', 'add', 'update', 'change', 'modify', 'implement', 'create',
    'make', 'get', 'set', 'find', 'look', 'check', 'help', 'want',
    'need', 'try', 'work', 'use', 'file', 'code', 'function', 'class'
}

# Intent patterns from intent-capture.py (tag mapping)
INTENT_PATTERNS = [
    (r'auth|login|session|oauth|jwt|password', ['authentication', 'security']),
    (r'test[s]?[/_]|_test\.|\bspec[s]?\b|pytest|unittest', ['testing']),
    (r'config|settings|\.env|\.yaml|\.yml|\.json', ['configuration']),
    (r'api|endpoint|route|handler|controller', ['api']),
    (r'index|search|query|grep|find', ['search']),
    (r'model|schema|entity|db|database|migration|sql', ['data']),
    (r'component|view|template|page|ui|style|css|html', ['frontend']),
    (r'deploy|docker|k8s|ci|cd|pipeline|github', ['devops']),
    (r'error|exception|catch|throw|raise|fail', ['errors']),
    (r'log|debug|trace|print|console', ['logging']),
    (r'cache|redis|memory|store', ['caching']),
    (r'async|await|promise|thread|concurrent', ['async']),
    (r'hook|plugin|extension|middleware', ['hooks']),
    (r'doc|readme|comment|docstring', ['documentation']),
    (r'util|helper|common|shared|lib', ['utilities']),
    (r'ranking|score|predict|confidence', ['ranking']),
    (r'transition|session|pattern', ['transitions']),
]


def extract_keywords(text: str) -> list:
    """
    Extract meaningful keywords from natural language intent.

    Simple approach: tokenize, lowercase, filter stopwords.
    """
    import re
    # Tokenize: extract words
    tokens = re.findall(r'[a-zA-Z][a-zA-Z0-9_]*', text.lower())

    # Filter: remove stopwords, keep meaningful tokens
    keywords = [t for t in tokens if t not in STOPWORDS and len(t) > 2]

    # Dedupe while preserving order
    seen = set()
    unique = []
    for k in keywords:
        if k not in seen:
            seen.add(k)
            unique.append(k)

    return unique


def map_keywords_to_tags(keywords: list) -> list:
    """
    Map extracted keywords to intent tags.

    Matches keywords against INTENT_PATTERNS.
    """
    import re
    matched_tags = set()
    combined = ' '.join(keywords)

    for pattern, tags in INTENT_PATTERNS:
        if re.search(pattern, combined, re.IGNORECASE):
            matched_tags.update(tags)

    return list(matched_tags)


@app.route('/context', methods=['POST'])
def context_search():
    """
    Natural language intent -> ranked files + snippets.

    POST body:
    {
        "intent": "fix the auth bug in login",
        "limit": 5,
        "snippet_lines": 10,
        "trigger_file": ".context/CURRENT.md"  (optional)
    }

    Returns:
    {
        "intent": "fix the auth bug in login",
        "keywords": ["auth", "bug", "login"],
        "tags_matched": ["authentication", "security"],
        "files": [
            {
                "path": "src/auth/login.py",
                "confidence": 0.85,
                "snippet": "..."
            }
        ],
        "ms": 12.5
    }
    """
    start = time.time()

    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Ranking not available'}), 503

    data = request.json or {}
    intent = data.get('intent', '')
    limit = int(data.get('limit', 5))
    snippet_lines = int(data.get('snippet_lines', 10))
    trigger_file = data.get('trigger_file', '')

    if not intent:
        return jsonify({'error': 'intent required'}), 400

    # Step 1: Extract keywords
    keywords = extract_keywords(intent)

    if not keywords:
        return jsonify({
            'error': 'No keywords extracted from intent',
            'intent': intent,
            'keywords': []
        }), 400

    # Step 1.5: Check cache (normalized keyword key)
    cache_key = f"aoa:context:{':'.join(sorted(keywords))}"
    try:
        cached = scorer.redis.client.get(cache_key)
        if cached:
            cached_result = json.loads(cached)
            cached_result['cached'] = True
            cached_result['ms'] = round((time.time() - start) * 1000, 2)
            return jsonify(cached_result)
    except Exception:
        pass  # Cache miss or error, continue

    # Step 2: Map keywords to tags
    tags_matched = map_keywords_to_tags(keywords)

    # Step 3: Get ranked files (using tags as boost)
    all_tags = list(set(keywords + tags_matched))
    results = scorer.get_ranked_files(tags=all_tags, limit=limit * 2)

    # Step 4: Get transition predictions if trigger file provided
    transition_preds = {}
    host_root = '/home/corey/aOa'
    if trigger_file and SESSION_PARSER_AVAILABLE:
        try:
            trans_results = SessionLogParser.predict_next(scorer.redis, trigger_file, limit=10)
            for f, prob in trans_results:
                transition_preds[f] = prob
                if not f.startswith('/'):
                    transition_preds[os.path.join(host_root, f)] = prob
        except Exception:
            pass

    # Step 5: Build response with snippets
    files = []
    seen_paths = set()

    for r in results:
        file_path = r['file']
        confidence = r.get('confidence', min(r.get('score', 0.0) / 100.0, 1.0))

        # Boost if in transition predictions
        if file_path in transition_preds:
            confidence = min(1.0, confidence + transition_preds[file_path] * 0.3)

        file_data = {
            'path': file_path,
            'confidence': round(confidence, 3)
        }

        if snippet_lines > 0:
            snippet = read_file_snippet(file_path, snippet_lines)
            if snippet:
                file_data['snippet'] = snippet

        files.append(file_data)
        seen_paths.add(file_path)

        if len(files) >= limit:
            break

    # Add high-probability transition predictions
    if transition_preds and len(files) < limit:
        for trans_file, trans_prob in sorted(transition_preds.items(),
                                              key=lambda x: x[1], reverse=True):
            if trans_file not in seen_paths and trans_prob >= 0.1:
                file_data = {
                    'path': trans_file,
                    'confidence': round(trans_prob * 0.8, 3),
                    'source': 'transition'
                }
                if snippet_lines > 0:
                    snippet = read_file_snippet(trans_file, snippet_lines)
                    if snippet:
                        file_data['snippet'] = snippet
                files.append(file_data)
                if len(files) >= limit:
                    break

    # Sort by confidence
    files.sort(key=lambda x: x['confidence'], reverse=True)
    files = files[:limit]

    # Build response
    result = {
        'intent': intent,
        'keywords': keywords,
        'tags_matched': tags_matched,
        'files': files,
        'trigger_file': trigger_file if trigger_file else None,
        'cached': False
    }

    # Cache result (1 hour TTL, skip snippets for cache efficiency)
    try:
        cache_data = {
            'intent': intent,
            'keywords': keywords,
            'tags_matched': tags_matched,
            'files': [{'path': f['path'], 'confidence': f['confidence']} for f in files],
            'trigger_file': trigger_file if trigger_file else None
        }
        scorer.redis.client.setex(cache_key, 3600, json.dumps(cache_data))
    except Exception:
        pass  # Cache write failure is non-fatal

    result['ms'] = round((time.time() - start) * 1000, 2)
    return jsonify(result)


# ============================================================================
# Memory API - Dynamic Working Context (Phase 5)
# ============================================================================

# Domain patterns for prose generation
DOMAIN_PATTERNS = {
    r'auth|login|session|oauth': 'authentication',
    r'api|endpoint|route|handler': 'API layer',
    r'test|spec|mock': 'testing',
    r'config|settings|env': 'configuration',
    r'index|search|query': 'search infrastructure',
    r'rank|score|predict': 'ranking system',
    r'hook|capture|intent': 'intent tracking',
    r'gate|proxy|route': 'gateway',
    r'redis|cache|store': 'data layer',
    r'doc|readme|md': 'documentation',
}


def time_band(seconds_ago: float) -> str:
    """Convert seconds ago to human-readable time band."""
    if seconds_ago < 60:
        return "just now"
    if seconds_ago < 180:
        return "moments ago"
    if seconds_ago < 600:
        return f"{int(seconds_ago / 60)}m ago"
    if seconds_ago < 1800:
        return "recently"
    if seconds_ago < 3600:
        return "earlier this session"
    return "earlier today"


def confidence_phrase(score: float) -> str:
    """Convert numeric confidence to natural phrase."""
    if score > 0.8:
        return "main focus"
    if score > 0.6:
        return "actively working on"
    if score > 0.4:
        return "recently touched"
    return "in context"


def detect_domain(file_path: str) -> str:
    """Detect domain from file path."""
    path_lower = file_path.lower()
    for pattern, domain in DOMAIN_PATTERNS.items():
        if re.search(pattern, path_lower):
            return domain
    # Fallback to directory name
    parts = file_path.split('/')
    if len(parts) > 1:
        return parts[-2] if parts[-2] not in ('src', 'lib', 'app') else parts[-1].split('.')[0]
    return "general"


def get_recent_tags(limit: int = 5) -> List[str]:
    """Get recent intent tags."""
    try:
        stats = intent_index.get_stats()
        tags = stats.get('tags', {})
        # Sort by count, take top N
        sorted_tags = sorted(tags.items(), key=lambda x: x[1], reverse=True)[:limit]
        return [t[0].lstrip('#') for t, _ in sorted_tags]
    except Exception:
        return []


@app.route('/memory')
def get_memory():
    """
    Dynamic working memory - current context as LLM-readable prose.

    Returns structured narrative of:
    - Current focus (what you're working on)
    - Active files (recently touched)
    - Predicted next files
    - Intent signals

    Query params:
        format: prose (default), structured, compact
        window: time window in minutes (default 20)

    Example response (prose):
        ## Working Memory

        You're currently focused on the ranking system, specifically the scorer.

        **Active Files** (last 20 minutes):
        - src/ranking/scorer.py (5 touches, 3m ago) - main focus
        - src/index/indexer.py (2 touches, 12m ago) - actively working on

        **Predicted Next**:
        - src/ranking/redis_client.py (65% likely)

        **Intent Signals**: #python, #editing, #search
    """
    start = time.time()

    fmt = request.args.get('format', 'prose')
    window_mins = int(request.args.get('window', 20))
    window_secs = window_mins * 60

    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({
            'memory': 'Working memory unavailable (Redis not connected)',
            'format': fmt,
            'ms': round((time.time() - start) * 1000, 2)
        })

    try:
        now = time.time()

        # 1. Get recent files by recency
        recent_files = scorer.get_top_files_by_recency(limit=20)

        # Filter to window and enrich with data
        active_files = []
        for file_path, last_ts in recent_files:
            age = now - last_ts
            if age > window_secs:
                continue

            freq = scorer.get_frequency_score(file_path) or 0
            active_files.append({
                'path': file_path,
                'last_access': last_ts,
                'age_seconds': age,
                'time_band': time_band(age),
                'frequency': int(freq),
                'domain': detect_domain(file_path),
            })

        # 2. Detect primary focus
        focus_domain = "general"
        focus_file = None
        if active_files:
            # Most frequent in window is likely focus
            by_freq = sorted(active_files, key=lambda x: x['frequency'], reverse=True)
            focus_file = by_freq[0]['path']
            focus_domain = by_freq[0]['domain']

        # 3. Get predictions for next files
        predicted_next = []
        if focus_file and SESSION_PARSER_AVAILABLE:
            try:
                # Get relative path for transition lookup
                rel_path = focus_file
                if rel_path.startswith('/home/corey/aOa/'):
                    rel_path = rel_path[len('/home/corey/aOa/'):]

                preds = SessionLogParser.predict_next(scorer.redis, rel_path, limit=3)
                for pred_file, prob in preds:
                    predicted_next.append({
                        'path': pred_file,
                        'probability': round(prob * 100),
                    })
            except Exception:
                pass

        # 4. Get recent tags
        recent_tags = get_recent_tags(5)

        # 5. Calculate mode (read/write ratio from recent intents)
        mode = "exploring"
        try:
            records = intent_index.recent(limit=20)
            if records:
                writes = sum(1 for r in records if r.get('tool', '').lower() in ('write', 'edit', 'notebookedit'))
                reads = sum(1 for r in records if r.get('tool', '').lower() in ('read', 'glob', 'grep'))
                if writes > reads:
                    mode = "writing"
                elif reads > writes * 2:
                    mode = "reading"
                else:
                    mode = "mixed"
        except Exception:
            pass

        # Generate output based on format
        if fmt == 'compact':
            # Minimal token format
            file_list = ','.join(f"{os.path.basename(f['path'])}({f['frequency']}x,{f['time_band']})"
                                 for f in active_files[:5])
            pred_list = ','.join(f"{os.path.basename(p['path'])}({p['probability']}%)"
                                 for p in predicted_next)
            tag_list = ','.join(recent_tags)

            memory = f"FOCUS: {os.path.basename(focus_file or 'none')} ({focus_domain})\n"
            memory += f"ACTIVE: {file_list}\n"
            if pred_list:
                memory += f"NEXT: {pred_list}\n"
            memory += f"TAGS: {tag_list}\n"
            memory += f"MODE: {mode}"

        elif fmt == 'structured':
            # JSON with explanations
            return jsonify({
                'focus': {
                    'domain': focus_domain,
                    'file': focus_file,
                    'explanation': f"Based on highest frequency in last {window_mins}m"
                },
                'active_files': [{
                    'path': f['path'],
                    'frequency': f['frequency'],
                    'recency': f['time_band'],
                    'role': confidence_phrase(f['frequency'] / 10)
                } for f in active_files[:5]],
                'predicted_next': [{
                    'path': p['path'],
                    'probability': p['probability'],
                    'why': 'frequently follows current focus'
                } for p in predicted_next],
                'intent_signals': recent_tags,
                'mode': mode,
                'window_minutes': window_mins,
                'ms': round((time.time() - start) * 1000, 2)
            })

        else:
            # Prose format (default)
            lines = ["## Working Memory", ""]

            if focus_file:
                lines.append(f"You're currently focused on **{focus_domain}**, specifically `{os.path.basename(focus_file)}`.")
            else:
                lines.append("No recent file activity detected.")

            lines.append("")

            if active_files:
                lines.append(f"**Active Files** (last {window_mins} minutes):")
                for f in active_files[:5]:
                    role = confidence_phrase(f['frequency'] / 10)
                    lines.append(f"- `{f['path']}` ({f['frequency']}x, {f['time_band']}) - {role}")
                lines.append("")

            if predicted_next:
                lines.append("**Predicted Next**:")
                for p in predicted_next:
                    lines.append(f"- `{p['path']}` ({p['probability']}% likely)")
                lines.append("")

            if recent_tags:
                tag_str = ', '.join(f"#{t}" for t in recent_tags)
                lines.append(f"**Intent Signals**: {tag_str}")
                lines.append("")

            lines.append(f"**Mode**: {mode}")

            memory = '\n'.join(lines)

        return jsonify({
            'memory': memory,
            'format': fmt,
            'files_analyzed': len(active_files),
            'ms': round((time.time() - start) * 1000, 2)
        })

    except Exception as e:
        return jsonify({
            'error': str(e),
            'memory': '',
            'ms': round((time.time() - start) * 1000, 2)
        }), 500


# ============================================================================
# Main
# ============================================================================

def main():
    global manager, intent_index, scorer, tuner

    codebase_root = os.environ.get('CODEBASE_ROOT', '')
    repos_root = os.environ.get('REPOS_ROOT', './repos')
    config_dir = os.environ.get('CONFIG_DIR', '/config')
    indexes_dir = os.environ.get('INDEXES_DIR', '/indexes')
    port = int(os.environ.get('PORT', 9999))

    # Detect global mode
    global_mode = not codebase_root and Path(config_dir).exists()

    print("=" * 60)
    print("aOa Index Service - Multi-Index Architecture")
    if global_mode:
        print("Mode: GLOBAL (multi-project)")
    else:
        print("Mode: LEGACY (single project)")
    print("=" * 60)

    # Initialize ranking scorer and weight tuner FIRST (need Redis for intent index)
    redis_client = None
    if RANKING_AVAILABLE:
        scorer = Scorer()
        if scorer.redis.ping():
            print("Ranking scorer initialized (Redis connected)")
            redis_client = scorer.redis
            # Phase 4: Initialize weight tuner
            tuner = WeightTuner(scorer.redis)
            print("Weight tuner initialized (8 arms)")
        else:
            print("Ranking scorer initialized (Redis not available)")
            scorer = None
            tuner = None
    else:
        print("Ranking module not available")

    # Initialize intent index with Redis for persistence
    intent_index = IntentIndex(redis_client=redis_client)
    if redis_client:
        print("Intent index initialized (Redis-backed, persists across restarts)")
    else:
        print("Intent index initialized (in-memory only)")

    if global_mode:
        print(f"Config directory: {config_dir}")
        print(f"Indexes directory: {indexes_dir}")
    else:
        print(f"Local codebase: {codebase_root}")
    print(f"Repos directory: {repos_root}")
    print()

    # Create index manager
    manager = IndexManager(
        codebase_root if codebase_root else None,
        repos_root,
        config_dir if global_mode else None,
        indexes_dir if global_mode else None
    )

    # Initialize indexes
    manager.init_local()
    manager.init_repos()

    print()
    if manager.local:
        print(f"Local: {len(manager.local.files)} files, {len(manager.local.inverted_index)} symbols")
    if manager.projects:
        print(f"Projects: {len(manager.projects)} project indexes loaded")
    print(f"Repos: {len(manager.repos)} knowledge repos loaded")
    print()

    try:
        print(f"Listening on http://0.0.0.0:{port}")
        app.run(host='0.0.0.0', port=port, threaded=True)
    finally:
        manager.shutdown()


if __name__ == '__main__':
    main()
