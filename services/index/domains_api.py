#!/usr/bin/env python3
"""
Domain Learning API - Flask Blueprint

Extracted from indexer.py for maintainability (CH-01).
Contains all /domains/* routes for semantic domain learning.

Routes:
  - /domains/seed (deprecated)
  - /domains/init-skeleton
  - /domains/unenriched
  - /domains/pending
  - /domains/enrich
  - /domains/enrichment-status
  - /domains/unenrich
  - /domains/enrichment-prompt
  - /domains/stats
  - /domains/list
  - /domains/lookup
  - /domains/learn
  - /domains/trigger-learn
  - /domains/self-learn
  - /domains/add-context
  - /domains/autotune
  - /domains/tune/math
  - /domains/tune
  - /domains/tuned
  - /domains/add
  - /domains/learned
  - /domains/submit-tags
  - /domains/unmatched-tags
  - /domains/goal-history

Registration:
    from domains_api import domains_bp, init_domains_api
    init_domains_api(manager, intent_index, DOMAINS_AVAILABLE, DomainLearner)
    app.register_blueprint(domains_bp)
"""

import os
import time
from flask import Blueprint, jsonify, request

# Blueprint for domain routes
domains_bp = Blueprint('domains', __name__)

# These are set by init_domains_api() from the main indexer
_manager = None
_intent_index = None
_domains_available = False
_DomainLearner = None
_Domain = None


def init_domains_api(manager, intent_index, domains_available, domain_learner_class, domain_class=None):
    """Initialize the domains API with dependencies from the main indexer."""
    global _manager, _intent_index, _domains_available, _DomainLearner, _Domain
    _manager = manager
    _intent_index = intent_index
    _domains_available = domains_available
    _DomainLearner = domain_learner_class
    _Domain = domain_class


def _get_orphan_tags(project_id: str, limit: int = 50) -> list[str]:
    """
    Get tags from recent intents that don't match any existing domain term.

    GL-090: Orphan tags indicate semantic gaps - areas users work on
    that aren't covered by existing domains.
    """
    try:
        learner = _DomainLearner(project_id)

        # Get all existing terms across all domains
        existing_terms = set()
        for domain_name in learner.get_all_domains():
            terms = learner.get_domain_terms(domain_name)
            existing_terms.update(terms)
            # Also add keywords for each term
            for term in terms:
                keywords = learner.get_term_keywords(term)
                existing_terms.update(keywords)

        # Get recent intent records
        records = _intent_index.recent(None, 50, project_id)
        if not records:
            return []

        # Collect tags from intents
        tag_counts = {}
        for r in records:
            tags = r.get('tags', [])
            for tag in tags:
                # Skip domain tags (@...) and very short tags
                if tag.startswith('@') or tag.startswith('#') or len(tag) < 3:
                    continue
                # Clean the tag
                clean_tag = tag.lower().strip()
                # Skip if it matches an existing term
                if clean_tag in existing_terms:
                    continue
                tag_counts[clean_tag] = tag_counts.get(clean_tag, 0) + 1

        # P3-4: Boost with explicit orphan hit counts (from direct searches)
        orphan_hits = learner.get_orphan_hits()
        for tag, hits in orphan_hits.items():
            if tag in tag_counts:
                tag_counts[tag] += hits  # Boost existing tags
            elif tag not in existing_terms and len(tag) >= 3:
                tag_counts[tag] = hits  # Add new tags from direct searches

        # Sort by frequency (including hits) and return top N
        sorted_tags = sorted(tag_counts.items(), key=lambda x: -x[1])
        return [tag for tag, count in sorted_tags[:limit] if count >= 2]

    except Exception as e:
        print(f"[OrphanTags] Error: {e}", flush=True)
        return []


# =============================================================================
# Domain Routes
# =============================================================================

@domains_bp.route('/domains/seed', methods=['POST'])
def seed_domains():
    """
    DEPRECATED: Universal domain seeding removed in GL-084.

    Use /aoa-setup skill to generate project-specific domains instead.
    """
    return jsonify({
        'error': 'Universal domain seeding removed. Use /aoa-setup to generate project-specific domains.',
        'success': False,
        'deprecated': True
    }), 410  # 410 Gone


# =========================================================================
# GL-085: Lazy Domain Enrichment Endpoints
# =========================================================================

@domains_bp.route('/domains/init-skeleton', methods=['POST'])
def domains_init_skeleton():
    """
    Initialize domains from skeleton (names + terms only, no keywords).

    GL-085: Called by /aoa-start skill. Sets enriched=false on all domains.
    Keywords are added lazily via hook-triggered enrichment.

    POST body: {
        "project_id": "xxx",
        "domains": [
            {"name": "@domain", "description": "...", "terms": ["term1", "term2"]}
        ]
    }
    """
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    domains = data.get('domains', [])

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    if not domains:
        return jsonify({'error': 'No domains provided'}), 400

    if len(domains) > 40:
        return jsonify({'error': f'Too many domains ({len(domains)}), max 40'}), 400

    try:
        learner = _DomainLearner(project_id)
        result = learner.init_skeleton(domains)
        return jsonify({
            'success': True,
            'project_id': project_id,
            **result
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/unenriched')
def domains_unenriched():
    """
    Get one domain that needs keyword enrichment.

    GL-085: Called by hook to find next domain to enrich.

    Returns: {"domain": {"name": "@x", "description": "...", "terms": [...]}}
    Or: {"domain": null} if all enriched
    """
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    project_id = request.args.get('project_id')
    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = _DomainLearner(project_id)
        domain = learner.get_unenriched_domain()
        status = learner.get_enrichment_status()
        return jsonify({
            'domain': domain,
            'enrichment': status
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/pending')
def domains_pending():
    """
    Get list of unenriched domain names.

    GL-088: Used by `aoa domains pending` for batch processing.
    """
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    project_id = request.args.get('project_id')
    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    limit = int(request.args.get('limit', 10))

    try:
        learner = _DomainLearner(project_id)
        domains = learner.get_unenriched_domains(limit)
        status = learner.get_enrichment_status()
        return jsonify({
            'domains': domains,
            'enrichment': status
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/enrich', methods=['POST'])
def domains_enrich():
    """
    Enrich a domain with keywords for its terms.

    GL-085: Called by hook after Haiku generates keywords.

    POST body: {
        "project_id": "xxx",
        "domain": "@domain_name",
        "term_keywords": {
            "term1": ["kw1", "kw2"],
            "term2": ["kw3", "kw4"]
        }
    }
    """
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    domain_name = data.get('domain')
    term_keywords = data.get('term_keywords', {})

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    if not domain_name:
        return jsonify({'error': 'Missing domain parameter'}), 400

    try:
        learner = _DomainLearner(project_id)
        result = learner.enrich_domain(domain_name, term_keywords)
        status = learner.get_enrichment_status()

        # GL-047: Rebuild keyword matcher if enrichment added new keywords
        all_keywords = []
        for kws in term_keywords.values():
            all_keywords.extend(kws)

        if all_keywords and _intent_index and _intent_index.redis:
            # Rebuild keyword index for this project
            try:
                idx = _manager.get_local(project_id)
                if idx:
                    from indexer import get_keyword_matcher
                    # Force rebuild by passing the intent index
                    # The matcher will pick up new keywords from Redis
                    matcher = get_keyword_matcher(project_id, _intent_index)
                    if matcher:
                        matcher.update_from_domains(
                            learner.get_all_domains(),
                            learner.get_domain_terms,
                            learner.get_term_keywords
                        )
                        # Store in Redis for persistence
                        proj = _intent_index._project_key(project_id)
                        r = _intent_index.redis.client if hasattr(_intent_index.redis, 'client') else _intent_index.redis
                        # Update keyword count metric
                        r.hset(f'aoa:{proj}:metrics', 'keyword_count', str(len(all_keywords)))
            except Exception as e:
                print(f"[Enrich] Keyword rebuild warning: {e}", flush=True)
                # Non-fatal - continue with response

        return jsonify({
            'success': True,
            'project_id': project_id,
            'domain': domain_name,
            'keywords_added': len(all_keywords),
            'enrichment': status,
            **result
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/enrichment-status')
def domains_enrichment_status():
    """Get current enrichment progress."""
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    project_id = request.args.get('project_id')
    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = _DomainLearner(project_id)
        status = learner.get_enrichment_status()
        return jsonify(status)
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/unenrich', methods=['POST'])
def domains_unenrich():
    """Mark a domain as unenriched (for re-enrichment)."""
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    domain_name = data.get('domain')

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400
    if not domain_name:
        return jsonify({'error': 'Missing domain parameter'}), 400

    try:
        learner = _DomainLearner(project_id)
        learner.set_domain_enriched(domain_name, False)
        return jsonify({'success': True, 'domain': domain_name, 'enriched': False})
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/enrichment-prompt')
def domains_enrichment_prompt():
    """Get the enrichment prompt for the next unenriched domain."""
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    project_id = request.args.get('project_id')
    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = _DomainLearner(project_id)
        domain = learner.get_unenriched_domain()

        if not domain:
            return jsonify({
                'prompt': None,
                'domain': None,
                'message': 'All domains are enriched'
            })

        prompt = learner.get_enrichment_prompt(domain)
        return jsonify({
            'prompt': prompt,
            'domain': domain
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/stats')
def domains_stats():
    """Get domain statistics for the project."""
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    project_id = request.args.get('project_id')
    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = _DomainLearner(project_id)
        stats = learner.get_stats()
        # Add enrichment status to stats
        enrichment = learner.get_enrichment_status()
        stats['enrichment'] = enrichment
        return jsonify(stats)
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/list')
def domains_list():
    """List all domains for a project."""
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    project_id = request.args.get('project_id')
    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    # Parse limit parameter (default: return all)
    limit = request.args.get('limit', type=int)

    try:
        learner = _DomainLearner(project_id)
        domain_names = learner.get_all_domains()

        domains = []
        for name in domain_names:
            terms = list(learner.get_domain_terms(name))
            keywords = []
            for term in terms:
                keywords.extend(list(learner.get_term_keywords(term)))

            # Get domain metadata for hits and enriched status
            meta = learner.get_domain_meta(name)
            enriched = learner.is_domain_enriched(name)
            hits = int(float(meta.get('hits', 0) or 0))

            domains.append({
                'name': name,
                'terms': terms,
                'keywords': keywords,
                'term_count': len(terms),
                'keyword_count': len(keywords),
                'enriched': enriched,
                'hits': hits,
            })

        # Sort by keyword count (most populated first)
        domains.sort(key=lambda d: -d['keyword_count'])

        # Apply limit if specified (after sorting)
        total_domains = len(domains)
        if limit and limit > 0:
            domains = domains[:limit]

        return jsonify({
            'project_id': project_id,
            'domains': domains,
            'total_domains': total_domains,  # Total before limiting
            'total_terms': sum(d['term_count'] for d in domains),
            'total_keywords': sum(d['keyword_count'] for d in domains)
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/lookup')
def domains_lookup():
    """Look up which domain(s) a term belongs to."""
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    project_id = request.args.get('project_id')
    term = request.args.get('term', '').lower().strip()

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400
    if not term:
        return jsonify({'error': 'Missing term parameter'}), 400

    try:
        learner = _DomainLearner(project_id)
        domains = learner.get_domains_for_term(term)

        return jsonify({
            'term': term,
            'domains': domains,
            'match_count': len(domains)
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/learn', methods=['POST'])
def domains_learn():
    """
    Submit new domain knowledge from Haiku analysis.

    POST body: {
        "project_id": "xxx",
        "domains": [
            {
                "name": "@domain",
                "description": "...",
                "terms": ["term1", "term2"],
                "keywords": {"term1": ["kw1"], "term2": ["kw2"]}
            }
        ]
    }
    """
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    domains = data.get('domains', [])

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    if not domains:
        return jsonify({'error': 'No domains provided'}), 400

    try:
        learner = _DomainLearner(project_id)
        added = 0
        updated = 0

        for domain_data in domains:
            name = domain_data.get('name')
            if not name:
                continue

            # Add or update domain
            existing = learner.get_domain_terms(name)
            if existing:
                updated += 1
            else:
                added += 1

            # Add terms
            terms = domain_data.get('terms', [])
            for term in terms:
                learner.add_term_to_domain(name, term)

            # Add keywords
            keywords = domain_data.get('keywords', {})
            for term, kws in keywords.items():
                for kw in kws:
                    learner.add_keyword_to_term(term, kw)

        return jsonify({
            'success': True,
            'project_id': project_id,
            'domains_added': added,
            'domains_updated': updated
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/trigger-learn', methods=['POST'])
def domains_trigger_learn():
    """
    Trigger domain learning for orphan tags.

    GL-090: Called when orphan threshold reached.
    Returns orphan tags for Haiku to categorize.
    """
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    limit = data.get('limit', 30)

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = _DomainLearner(project_id)
        orphans = _get_orphan_tags(project_id, limit)

        return jsonify({
            'project_id': project_id,
            'orphan_tags': orphans,
            'count': len(orphans),
            'existing_domains': list(learner.get_all_domains())
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/self-learn', methods=['POST'])
def domains_self_learn():
    """
    Self-learning endpoint for domain expansion.

    GL-090: Analyzes recent intents and suggests domain updates.
    Called periodically or when orphan threshold is high.
    """
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = _DomainLearner(project_id)

        # Get orphan tags
        orphans = _get_orphan_tags(project_id, 50)

        # Get existing domain structure
        domains = learner.get_all_domains()
        domain_terms = {}
        for d in domains:
            domain_terms[d] = learner.get_domain_terms(d)

        # Suggest assignments (simple heuristic - could be enhanced with Haiku)
        suggestions = []
        for orphan in orphans[:20]:
            # Find best matching domain based on term overlap
            best_domain = None
            best_score = 0

            for domain, terms in domain_terms.items():
                # Simple overlap scoring
                score = sum(1 for t in terms if orphan in t or t in orphan)
                if score > best_score:
                    best_score = score
                    best_domain = domain

            if best_domain and best_score > 0:
                suggestions.append({
                    'tag': orphan,
                    'domain': best_domain,
                    'confidence': min(best_score * 0.3, 0.9)
                })

        return jsonify({
            'project_id': project_id,
            'orphan_count': len(orphans),
            'suggestions': suggestions,
            'domains': list(domains)
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/get-haiku-prompt', methods=['POST'])
def domains_get_haiku_prompt():
    """
    RB-14: Get the full Haiku prompt for intent generation.

    Gets recent prompts via /cc/prompts, combines with existing domains,
    and returns the complete prompt for a Task agent to run Haiku.

    POST body: {
        "project_id": "uuid",
        "project_path": "/path/to/project",
        "limit": 25  # optional, defaults to 25
    }

    Returns: {
        "prompt": "the full haiku prompt...",
        "output_file": "/path/to/.aoa/domains/intent.json",
        "prompt_count": 25,
        "existing_domains": ["@cli", "@search", ...]
    }
    """
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    project_path = data.get('project_path', '/codebase')
    limit = data.get('limit', 25)

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        import requests

        # Get recent prompts via internal call to /cc/prompts
        # Use localhost since we're in the same container
        index_url = os.environ.get('INDEX_URL', 'http://localhost:8080')
        prompts_response = requests.get(
            f"{index_url}/cc/prompts",
            params={'limit': limit, 'project_path': project_path},
            timeout=5
        )
        prompts_data = prompts_response.json()
        prompts = prompts_data.get('prompts', [])

        if not prompts:
            return jsonify({
                'error': 'No prompts found',
                'project_path': project_path
            }), 400

        # Get existing domains and build the Haiku prompt
        learner = _DomainLearner(project_id)
        existing_domains = list(learner.get_all_domains())
        haiku_prompt = learner.get_intent_prompt(prompts, existing_domains)
        output_file = learner.get_intent_file_path()

        return jsonify({
            'prompt': haiku_prompt,
            'output_file': output_file,
            'prompt_count': len(prompts),
            'existing_domains': existing_domains,
            'project_id': project_id
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/haiku-pending', methods=['GET', 'POST'])
def domains_haiku_pending():
    """
    RB-14: Get/set Haiku learning pending flag.

    GET: Returns pending status and prompt data
    POST: Set pending data or clear flag

    POST body to set:
    {
        "project_id": "xxx",
        "prompt": "the haiku prompt...",
        "output_file": "/path/to/intent.json",
        "prompt_count": 25
    }

    POST body to clear:
    {
        "project_id": "xxx",
        "clear": true
    }
    """
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    if request.method == 'GET':
        project_id = request.args.get('project_id')
        if not project_id:
            return jsonify({'error': 'Missing project_id parameter'}), 400

        try:
            learner = _DomainLearner(project_id)
            # Check Redis for pending flag
            pending_data = learner.redis.client.get(f"aoa:{project_id}:haiku_pending")
            if pending_data:
                import json
                data = json.loads(pending_data)
                return jsonify({
                    'pending': True,
                    'prompt': data.get('prompt', ''),
                    'output_file': data.get('output_file', ''),
                    'prompt_count': data.get('prompt_count', 0)
                })
            return jsonify({'pending': False})
        except Exception as e:
            return jsonify({'error': str(e)}), 500

    # POST - set or clear
    data = request.json or {}
    project_id = data.get('project_id')

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = _DomainLearner(project_id)
        key = f"aoa:{project_id}:haiku_pending"

        if data.get('clear'):
            learner.redis.client.delete(key)
            return jsonify({'success': True, 'cleared': True})

        # Set pending data with 5-minute TTL (cleared on next prompt or expires)
        import json
        pending_data = json.dumps({
            'prompt': data.get('prompt', ''),
            'output_file': data.get('output_file', ''),
            'prompt_count': data.get('prompt_count', 0)
        })
        learner.redis.client.setex(key, 300, pending_data)

        return jsonify({'success': True, 'set': True})

    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/add-context', methods=['POST'])
def domains_add_context():
    """
    GL-090: Add a new context-tier domain from Haiku-generated skeleton.

    POST body: {
        "project_id": "uuid",
        "name": "@domain_name",
        "description": "what this domain covers",
        "terms": ["term1", "term2"],
        "keywords": {"term1": ["kw1", "kw2"]}
    }
    """
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    name = data.get('name')
    description = data.get('description', '')
    terms = data.get('terms', [])
    keywords = data.get('keywords', {})

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400
    if not name:
        return jsonify({'error': 'Missing name parameter'}), 400
    if not terms:
        return jsonify({'error': 'Missing terms parameter'}), 400

    # A72-22: Validate domain data - no null/empty/invalid values
    if name in ('null', 'None', '', None) or not isinstance(name, str):
        return jsonify({'error': f'Invalid domain name: {name}'}), 400
    if not name.startswith('@'):
        return jsonify({'error': f'Domain name must start with @: {name}'}), 400

    # Validate terms are meaningful strings
    valid_terms = [t for t in terms if isinstance(t, str) and len(t) >= 2 and t not in ('null', 'None', '')]
    if not valid_terms:
        return jsonify({'error': 'No valid terms provided (must be strings with length >= 2)'}), 400

    # Validate keywords if provided
    if keywords:
        valid_keywords = {}
        for term, kws in keywords.items():
            if isinstance(term, str) and len(term) >= 2 and term not in ('null', 'None', ''):
                valid_kws = [kw for kw in kws if isinstance(kw, str) and len(kw) >= 2 and kw not in ('null', 'None', '')]
                if valid_kws:
                    valid_keywords[term] = valid_kws
        keywords = valid_keywords

    try:
        learner = _DomainLearner(project_id)

        # A72-CAP: Enforce context tier cap before adding
        if not learner.can_add_context_domain():
            return jsonify({'error': f'Context tier full (max {learner.CONTEXT_DOMAINS_MAX})'}), 400

        # Create Domain object (use validated terms)
        domain = _Domain(
            name=name,
            description=description,
            confidence=0.8,  # High confidence for user-provided domains
            terms=valid_terms
        )

        # Add domain with context tier
        learner.add_domain(domain, source='haiku', tier='context')

        # Add keywords for terms
        keyword_count = 0
        for term, kws in keywords.items():
            for kw in kws:
                learner.add_keyword_to_term(kw, term)  # keyword first, then term
                keyword_count += 1

        return jsonify({
            'success': True,
            'project_id': project_id,
            'domain': name,
            'terms_added': len(terms),
            'keywords_added': keyword_count
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/autotune', methods=['POST'])
def domains_autotune():
    """
    GL-091: Run autotune cycle on domains.

    Performs:
    - Decay all hit counts
    - Promote high-hit context domains to core
    - Demote stale core domains to context
    - Prune low-hit context domains
    """
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = _DomainLearner(project_id)

        # Run math-based tuning
        result = learner.run_math_tune()

        return jsonify({
            'success': True,
            'project_id': project_id,
            **result
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/tune/math', methods=['POST'])
def domains_tune_math():
    """
    GL-091: Run math-based tuning (no LLM).

    Operations:
    1. Decay all hits by 20%
    2. Promote context → core if hits >= threshold
    3. Demote core → context if stale
    4. Prune context if hits < floor
    """
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = _DomainLearner(project_id)
        result = learner.run_math_tune()

        # Get updated stats
        stats = learner.get_stats()

        return jsonify({
            'success': True,
            'project_id': project_id,
            'tune_result': result,
            'stats': stats
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/tune', methods=['POST'])
def domains_tune():
    """
    Request domain tuning via Haiku.

    GL-091: Returns tuning prompt for Haiku to analyze domain health.
    """
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = _DomainLearner(project_id)

        # Get tuning context
        stats = learner.get_stats()
        domains = learner.get_all_domains()

        # Build tuning prompt
        prompt = learner.get_tune_prompt()

        return jsonify({
            'project_id': project_id,
            'prompt': prompt,
            'stats': stats,
            'domain_count': len(domains)
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/tuned', methods=['POST'])
def domains_tuned():
    """
    Submit tuning results from Haiku.

    POST body: {
        "project_id": "xxx",
        "actions": [
            {"action": "promote", "domain": "@search"},
            {"action": "demote", "domain": "@legacy"},
            {"action": "prune", "domain": "@unused"}
        ]
    }
    """
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    actions = data.get('actions', [])

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = _DomainLearner(project_id)
        results = learner.apply_tune_actions(actions)

        return jsonify({
            'success': True,
            'project_id': project_id,
            **results
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/add', methods=['POST'])
def domains_add():
    """
    Add a new domain or update existing.

    POST body: {
        "project_id": "xxx",
        "domain": {
            "name": "@domain",
            "description": "...",
            "terms": ["term1", "term2"]
        }
    }
    """
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    domain_data = data.get('domain', {})

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    if not domain_data or not domain_data.get('name'):
        return jsonify({'error': 'Missing domain or domain.name'}), 400

    try:
        learner = _DomainLearner(project_id)

        # Check if domain exists
        existing_terms = learner.get_domain_terms(domain_data['name'])
        is_update = len(existing_terms) > 0

        # Create Domain object
        from services.domains.learner import Domain
        domain = Domain(
            name=domain_data['name'],
            description=domain_data.get('description', ''),
            confidence=0.8,
            terms=domain_data.get('terms', [])
        )

        # Add domain
        result = learner.add_domain(domain, source='api')

        # Add terms if provided
        terms = domain_data.get('terms', [])
        for term in terms:
            learner.add_term_to_domain(domain_data['name'], term)

        return jsonify({
            'success': True,
            'project_id': project_id,
            'domain': domain_data['name'],
            'is_update': is_update,
            'terms_added': len(terms),
            **result
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/learned', methods=['POST'])
def domains_learned():
    """
    Bulk add learned domain mappings.

    POST body: {
        "project_id": "xxx",
        "mappings": {
            "tag1": "@domain1",
            "tag2": "@domain2"
        }
    }

    GL-090: Used after Haiku categorizes orphan tags.
    """
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    mappings = data.get('mappings', {})

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    if not mappings:
        return jsonify({'error': 'No mappings provided'}), 400

    try:
        learner = _DomainLearner(project_id)
        added = 0
        skipped = 0

        for tag, domain in mappings.items():
            if not domain:
                skipped += 1
                continue

            # Normalize domain name
            if not domain.startswith('@'):
                domain = f'@{domain}'

            # Add tag as keyword to the domain's first term
            terms = list(learner.get_domain_terms(domain))
            if terms:
                learner.add_keyword_to_term(terms[0], tag)
                added += 1
            else:
                # Domain doesn't exist - create it with tag as term
                from services.domains.learner import Domain
                new_domain = Domain(
                    name=domain,
                    description=f'Auto-created for tag: {tag}',
                    confidence=0.7,
                    terms=[tag]
                )
                learner.add_domain(new_domain, source='learned')
                added += 1

        return jsonify({
            'success': True,
            'project_id': project_id,
            'added': added,
            'skipped': skipped
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/submit-tags', methods=['POST'])
def domains_submit_tags():
    """
    Submit tags for domain matching and learning.

    POST body: {
        "project_id": "xxx",
        "tags": ["tag1", "tag2", "tag3"]
    }

    Returns matched domains and orphan tags.
    """
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    tags = data.get('tags', [])

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = _DomainLearner(project_id)

        matched = {}
        orphans = []

        for tag in tags:
            domains = learner.get_domains_for_term(tag)
            if domains:
                matched[tag] = domains
                # Increment hit counter for matched domains
                for d in domains:
                    learner.increment_domain_hits(d)
            else:
                orphans.append(tag)
                # Record orphan hit for learning
                learner.record_orphan_hit(tag)

        return jsonify({
            'project_id': project_id,
            'matched': matched,
            'orphans': orphans,
            'match_rate': len(matched) / len(tags) if tags else 0
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/unmatched-tags')
def domains_unmatched_tags():
    """Get tags that don't match any domain (orphans)."""
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    project_id = request.args.get('project_id')
    limit = int(request.args.get('limit', 20))

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        orphans = _get_orphan_tags(project_id, limit)
        return jsonify({
            'project_id': project_id,
            'orphan_tags': orphans,
            'count': len(orphans)
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@domains_bp.route('/domains/goal-history')
def domains_goal_history():
    """
    GL-078: Get recent prompt records (goal + tags) for display/learning.

    Returns the last N prompts with their goals and associated tags.
    Used for grouped display in `aoa domains` and for GL-072 learning.

    Query params:
        project_id: project ID
        limit: max prompts to return (default 10)
    """
    if not _domains_available:
        return jsonify({'error': 'Domain learning module not available'}), 500

    project_id = request.args.get('project_id')
    limit = int(request.args.get('limit', 10))

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = _DomainLearner(project_id)
        prompts = learner.get_prompt_records(limit=limit)

        return jsonify({
            'project_id': project_id,
            'prompts': prompts,
            'count': len(prompts),
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500
