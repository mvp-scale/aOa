//go:build !lean

package treesitter

import (
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// L19.9 — E1 import-edge extraction unit tests
// =============================================================================

// TestExtractImportsGo verifies Go import-edge extraction for all three
// import shapes: single, grouped, and aliased.
func TestExtractImportsGo(t *testing.T) {
	p := NewParser()

	source := []byte(`package main

import "fmt"

import (
	"os"
	"path/filepath"
)

import alias "net/http"

func main() {}
`)

	tree, langName, err := p.ParseToTree("main.go", source)
	require.NoError(t, err)
	require.NotNil(t, tree)
	defer tree.Close()

	assert.Equal(t, "go", langName)

	edges := extractImportsGo(tree.RootNode(), source, "main.go")
	require.Len(t, edges, 4, "expected 4 import edges: fmt, os, filepath, net/http")

	paths := collectImportPaths(edges)
	assert.Contains(t, paths, "fmt")
	assert.Contains(t, paths, "os")
	assert.Contains(t, paths, "path/filepath")
	assert.Contains(t, paths, "net/http")

	// All edges carry file:line provenance (G7)
	for _, e := range edges {
		assert.Equal(t, "main.go", e.FromFile, "FromFile must be set (G7)")
		assert.Greater(t, e.StartLine, uint32(0), "StartLine must be non-zero (G7)")
	}
}

// TestExtractImportsGo_BlankDot verifies blank-identifier and dot imports.
func TestExtractImportsGo_BlankDot(t *testing.T) {
	p := NewParser()

	source := []byte(`package main

import (
	_ "database/sql/driver"
	. "fmt"
)
`)

	tree, _, err := p.ParseToTree("main.go", source)
	require.NoError(t, err)
	require.NotNil(t, tree)
	defer tree.Close()

	edges := extractImportsGo(tree.RootNode(), source, "main.go")
	require.Len(t, edges, 2)
	paths := collectImportPaths(edges)
	assert.Contains(t, paths, "database/sql/driver")
	assert.Contains(t, paths, "fmt")
}

// TestExtractImportsPython verifies Python import extraction for import_statement
// and import_from_statement shapes.
func TestExtractImportsPython(t *testing.T) {
	p := NewParser()

	source := []byte(`import os
import sys
from os.path import join, exists
import numpy as np
from typing import List
`)

	tree, langName, err := p.ParseToTree("app.py", source)
	require.NoError(t, err)
	require.NotNil(t, tree)
	defer tree.Close()

	assert.Equal(t, "python", langName)

	edges := extractImportsPython(tree.RootNode(), source, "app.py")
	require.GreaterOrEqual(t, len(edges), 5, "expected at least 5 import edges")

	paths := collectImportPaths(edges)
	assert.Contains(t, paths, "os")
	assert.Contains(t, paths, "sys")
	assert.Contains(t, paths, "os.path")
	assert.Contains(t, paths, "numpy")
	assert.Contains(t, paths, "typing")

	for _, e := range edges {
		assert.Equal(t, "app.py", e.FromFile, "FromFile must be set (G7)")
		assert.Greater(t, e.StartLine, uint32(0), "StartLine must be non-zero (G7)")
	}
}

// TestExtractImportsPython_Relative verifies that relative imports are captured
// with their relative path indicator.
func TestExtractImportsPython_Relative(t *testing.T) {
	p := NewParser()

	source := []byte(`from . import module
`)

	tree, _, err := p.ParseToTree("pkg/sub.py", source)
	require.NoError(t, err)
	require.NotNil(t, tree)
	defer tree.Close()

	edges := extractImportsPython(tree.RootNode(), source, "pkg/sub.py")
	require.GreaterOrEqual(t, len(edges), 1, "expected at least one relative import edge")
	assert.Equal(t, "pkg/sub.py", edges[0].FromFile)
	assert.Greater(t, edges[0].StartLine, uint32(0))
}

// TestExtractImportsJS verifies JS import extraction for named, default,
// namespace, and side-effect import shapes.
func TestExtractImportsJS(t *testing.T) {
	p := NewParser()

	source := []byte(`import React from 'react'
import { useState, useEffect } from 'react'
import * as path from 'path'
import './styles.css'
`)

	tree, langName, err := p.ParseToTree("app.js", source)
	require.NoError(t, err)
	require.NotNil(t, tree)
	defer tree.Close()

	assert.Equal(t, "javascript", langName)

	edges := extractImportsJS(tree.RootNode(), source, "app.js")
	require.Len(t, edges, 4, "expected 4 import edges")

	paths := collectImportPaths(edges)
	assert.Contains(t, paths, "react")
	assert.Contains(t, paths, "path")
	assert.Contains(t, paths, "./styles.css")

	for _, e := range edges {
		assert.Equal(t, "app.js", e.FromFile, "FromFile must be set (G7)")
		assert.Greater(t, e.StartLine, uint32(0), "StartLine must be non-zero (G7)")
	}
}

// TestExtractImportsTS verifies TypeScript import extraction (same grammar as JS).
func TestExtractImportsTS(t *testing.T) {
	p := NewParser()

	source := []byte(`import { Component } from '@angular/core'
import type { User } from './types'
`)

	tree, langName, err := p.ParseToTree("app.ts", source)
	require.NoError(t, err)
	require.NotNil(t, tree)
	defer tree.Close()

	assert.Equal(t, "typescript", langName)

	edges := extractImportsJS(tree.RootNode(), source, "app.ts")
	require.GreaterOrEqual(t, len(edges), 1, "expected at least 1 import edge for TypeScript")

	paths := collectImportPaths(edges)
	assert.Contains(t, paths, "@angular/core")

	for _, e := range edges {
		assert.Equal(t, "app.ts", e.FromFile)
		assert.Greater(t, e.StartLine, uint32(0))
	}
}

// TestParseFileToMetaAndFacts_SymbolsAndEdges verifies that the combined parse
// produces both symbols and import edges in a single tree-sitter pass.
func TestParseFileToMetaAndFacts_SymbolsAndEdges(t *testing.T) {
	p := NewParser()

	source := []byte(`package main

import (
	"fmt"
	"os"
)

func Hello(name string) string {
	return fmt.Sprintf("hello %s", name)
}

type Server struct {
	port int
}
`)

	metas, edges, err := p.ParseFileToMetaAndFacts("main.go", source)
	require.NoError(t, err)

	// Symbols extracted correctly
	require.GreaterOrEqual(t, len(metas), 2, "expected at least Hello + Server")
	names := collectMetaNames(metas)
	assert.Contains(t, names, "Hello")
	assert.Contains(t, names, "Server")

	// Import edges extracted correctly
	require.GreaterOrEqual(t, len(edges), 2, "expected at least fmt + os import edges")
	paths := collectImportPaths(edges)
	assert.Contains(t, paths, "fmt")
	assert.Contains(t, paths, "os")

	// Provenance stamps (G7)
	for _, e := range edges {
		assert.Equal(t, "main.go", e.FromFile)
		assert.Greater(t, e.StartLine, uint32(0))
	}
}

// TestParseFileToMeta_BackwardCompat verifies that ParseFileToMeta behavior is
// byte-identical to before (zero regression for existing callers).
func TestParseFileToMeta_BackwardCompat(t *testing.T) {
	p := NewParser()

	source := []byte(`package main

import "fmt"

func Hello() {
	fmt.Println("hello")
}

type Handler struct{}
`)

	// Old API — must still work identically
	metas, err := p.ParseFileToMeta("main.go", source)
	require.NoError(t, err)
	require.NotNil(t, metas)

	// New API — symbols must match exactly
	metasNew, edges, err := p.ParseFileToMetaAndFacts("main.go", source)
	require.NoError(t, err)

	require.Equal(t, len(metas), len(metasNew), "ParseFileToMeta and ParseFileToMetaAndFacts must return same symbol count")
	for i := range metas {
		assert.Equal(t, metas[i].Name, metasNew[i].Name, "symbol names must match at index %d", i)
		assert.Equal(t, metas[i].Kind, metasNew[i].Kind, "symbol kinds must match at index %d", i)
		assert.Equal(t, metas[i].StartLine, metasNew[i].StartLine, "symbol start lines must match at index %d", i)
	}

	// New API also produces edges
	assert.GreaterOrEqual(t, len(edges), 1, "ParseFileToMetaAndFacts must also produce import edges")
}

// TestParseFileToMetaAndFacts_UnknownLang verifies graceful handling of
// unsupported languages — no error, empty results.
func TestParseFileToMetaAndFacts_UnknownLang(t *testing.T) {
	p := NewParser()

	metas, edges, err := p.ParseFileToMetaAndFacts("config.xyz", []byte("something"))
	assert.NoError(t, err)
	assert.Nil(t, metas)
	assert.Nil(t, edges)
}

// TestParseFileToMetaAndFacts_EmptyFile verifies graceful handling of empty files.
func TestParseFileToMetaAndFacts_EmptyFile(t *testing.T) {
	p := NewParser()

	metas, edges, err := p.ParseFileToMetaAndFacts("main.go", []byte(""))
	assert.NoError(t, err)
	assert.Empty(t, metas)
	assert.Empty(t, edges)
}

// TestFactParser_InterfaceSatisfied verifies that *Parser satisfies ports.FactParser
// at compile time (fails to compile if the interface is not satisfied).
func TestFactParser_InterfaceSatisfied(t *testing.T) {
	var _ ports.FactParser = NewParser()
}

// --- test helpers ---

// collectImportPaths returns ImportPath strings from a slice of ImportEdge.
func collectImportPaths(edges []ports.ImportEdge) []string {
	out := make([]string, len(edges))
	for i, e := range edges {
		out[i] = e.ImportPath
	}
	return out
}

// collectMetaNames returns Name strings from a slice of *SymbolMeta.
func collectMetaNames(metas []*ports.SymbolMeta) []string {
	out := make([]string, len(metas))
	for i, m := range metas {
		out[i] = m.Name
	}
	return out
}
