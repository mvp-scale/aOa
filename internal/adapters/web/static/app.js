/* ══════════════════════════════════════════════════════════
   aOa Dashboard — Application Logic
   5-tab SPA: Live · Recon · Intel · Debrief · Arsenal
   ══════════════════════════════════════════════════════════ */
(function() {
'use strict';

/* ── State ── */
var activeTab = 'live';
var pollTimer = null;
var cache = {};
var heroData = null;
var lastConvCount = 0;

/* ══════════════════════════════════════════════════════════
   HELPERS
   ══════════════════════════════════════════════════════════ */
function setText(id, val) {
  var el = document.getElementById(id);
  if (el) el.textContent = val;
}
function setHtml(id, val) {
  var el = document.getElementById(id);
  if (el) el.innerHTML = val;
}
function escapeHtml(s) {
  if (!s) return '';
  var d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}
function fmtK(n) {
  if (n === undefined || n === null) return '-';
  n = Number(n);
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M';
  if (n >= 100000) return Math.round(n / 1000) + 'k';
  if (n >= 1000) return (n / 1000).toFixed(1) + 'k';
  return String(n);
}
function fmtPct(n) {
  if (n === undefined || n === null) return '-';
  return (Number(n) * 100).toFixed(0) + '%';
}
function fmtMin(min) {
  if (!min || min <= 0) return '-';
  if (min >= 60) return Math.floor(min / 60) + 'h ' + Math.round(min % 60) + 'm';
  return Math.round(min) + ' min';
}
function relTime(ts) {
  if (!ts) return '-';
  var s = Math.floor(Date.now() / 1000) - ts;
  if (s < 0) s = 0;
  if (s < 5) return 'now';
  if (s < 60) return s + 's';
  if (s < 3600) return Math.floor(s / 60) + 'm';
  return Math.floor(s / 3600) + 'h';
}
function truncPath(p, max) {
  if (!p) return '';
  if (p.length <= max) return p;
  return '...' + p.substring(p.length - max + 3);
}
function truncText(text, max) {
  if (!text) return '';
  if (text.length <= max) return text;
  return text.substring(0, max) + '...';
}
function fmtDate(ts) {
  if (!ts) return '-';
  var d = new Date(ts * 1000);
  return (d.getMonth() + 1) + '/' + d.getDate();
}
function fmtDuration(startTs, endTs) {
  if (!startTs || !endTs) return '-';
  var s = endTs - startTs;
  if (s < 60) return s + 's';
  if (s < 3600) return Math.floor(s / 60) + 'm';
  return Math.floor(s / 3600) + 'h ' + Math.round((s % 3600) / 60) + 'm';
}
function randChoice(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}
function safeFetch(url) {
  return fetch(url).then(function(r) {
    if (!r.ok) throw new Error(r.status);
    return r.json();
  });
}

/* ══════════════════════════════════════════════════════════
   THEME
   ══════════════════════════════════════════════════════════ */
var savedTheme = localStorage.getItem('aoa-theme');
if (savedTheme) document.documentElement.setAttribute('data-theme', savedTheme);

function updateThemeIcon() {
  var isDark = document.documentElement.getAttribute('data-theme') === 'dark';
  setText('themeIcon', isDark ? '\u263D' : '\u2600');
}
document.getElementById('themeToggle').addEventListener('click', function() {
  var cur = document.documentElement.getAttribute('data-theme');
  var next = cur === 'dark' ? 'light' : 'dark';
  document.documentElement.setAttribute('data-theme', next);
  localStorage.setItem('aoa-theme', next);
  updateThemeIcon();
});
updateThemeIcon();

/* ══════════════════════════════════════════════════════════
   CLOCK
   ══════════════════════════════════════════════════════════ */
function tickClock() {
  var d = new Date();
  setText('clock',
    String(d.getHours()).padStart(2, '0') + ':' +
    String(d.getMinutes()).padStart(2, '0') + ':' +
    String(d.getSeconds()).padStart(2, '0'));
}
tickClock();
setInterval(tickClock, 1000);

/* ══════════════════════════════════════════════════════════
   TAB SWITCHING
   ══════════════════════════════════════════════════════════ */
var tabs = document.querySelectorAll('.nav-tab');
var tabContents = document.querySelectorAll('.tab-content');

function switchTab(name) {
  activeTab = name;
  for (var i = 0; i < tabs.length; i++) {
    tabs[i].classList.toggle('active', tabs[i].getAttribute('data-tab') === name);
  }
  for (var j = 0; j < tabContents.length; j++) {
    tabContents[j].classList.toggle('active', tabContents[j].id === 'tab-' + name);
  }
  location.hash = name;
  renderHero(name);
  poll(); // immediate fetch for new tab
}

for (var i = 0; i < tabs.length; i++) {
  tabs[i].addEventListener('click', function() {
    switchTab(this.getAttribute('data-tab'));
  });
}

// Restore tab from URL hash
var hashTab = location.hash.replace('#', '');
if (hashTab && document.getElementById('tab-' + hashTab)) {
  switchTab(hashTab);
}

/* ══════════════════════════════════════════════════════════
   HERO STORIES
   ══════════════════════════════════════════════════════════ */
var HERO_STORIES = {
  live: [
    { outcome: 'want full context that lasts the entire task', exclusion: 'context cliff that kills momentum at 60%' },
    { outcome: 'need every guided read to extend the session', exclusion: 'token waste from unguided file scans' },
    { outcome: 'want to ship the feature in one unbroken session', exclusion: 'starting over when the model loses context' }
  ],
  recon: [
    { outcome: 'want full visibility into codebase risk', exclusion: 'manual code review that misses what matters' },
    { outcome: 'need security and quality concerns surfaced before they ship', exclusion: 'audit surprises discovered in production' },
    { outcome: 'want dimensional intelligence across every file', exclusion: 'blind spots hiding in code nobody reviews' }
  ],
  intel: [
    { outcome: 'want a system that learns their codebase deeply', exclusion: 'starting from zero every session' },
    { outcome: 'need domain intelligence that sharpens with every prompt', exclusion: 'static tools that never adapt to how you work' },
    { outcome: 'want their working patterns to drive smarter results', exclusion: 'generic search that treats every query the same' }
  ],
  debrief: [
    { outcome: 'want to see exactly what happened and why', exclusion: 'scrolling through terminal history to reconstruct the timeline' },
    { outcome: 'need to understand where tokens went and what they bought', exclusion: 'guessing why context filled up so fast' },
    { outcome: 'want a clear record of every action and its impact', exclusion: 'losing track of what was searched, read, and written' }
  ],
  arsenal: [
    { outcome: 'want their system configured and ready to perform', exclusion: 'setup friction that delays the real work' },
    { outcome: 'need full visibility into daemon health and indexing state', exclusion: 'wondering whether the system is actually running' },
    { outcome: 'want one place to verify everything is wired correctly', exclusion: 'debugging configuration spread across scattered files' }
  ]
};
var HERO_IDENTITIES = ['10x Developers', 'Relentless Builders', 'Precision Engineers', 'Full-Stack Architects', 'High-Velocity Teams'];
var HERO_SEPARATORS = ['minus the', 'instead of', 'bypassing', 'without'];

// Try loading hero.json for overrides
safeFetch('/hero.json').then(function(data) {
  heroData = data;
  if (data.identities) HERO_IDENTITIES = data.identities;
  if (data.separators) HERO_SEPARATORS = data.separators;
  if (data.tabs) {
    for (var t in data.tabs) {
      if (data.tabs[t].stories) HERO_STORIES[t] = data.tabs[t].stories;
    }
  }
  renderHero(activeTab);
}).catch(function() {});

function renderHero(tab) {
  var stories = HERO_STORIES[tab];
  if (!stories || stories.length === 0) return;
  var story = randChoice(stories);
  var identity = randChoice(HERO_IDENTITIES);
  var separator = randChoice(HERO_SEPARATORS);
  var el = document.getElementById('heroHeadline-' + tab);
  if (el) {
    el.innerHTML =
      '<span class="hero-identity">' + escapeHtml(identity) + '</span> ' +
      '<span class="hero-outcome">' + escapeHtml(story.outcome) + '</span>' +
      '<span class="hero-pause">\u2009.\u2009.\u2009.</span>' +
      '<span class="hero-line2"><span class="hero-separator">' + escapeHtml(separator) + '</span> ' +
      '<span class="hero-exclusion">' + escapeHtml(story.exclusion) + '.</span></span>';
  }
}

// Render initial heroes for all tabs
['live', 'recon', 'intel', 'debrief', 'arsenal'].forEach(function(t) { renderHero(t); });

/* ══════════════════════════════════════════════════════════
   POLLING
   ══════════════════════════════════════════════════════════ */
function poll() {
  // Always fetch health for status badge
  safeFetch('/api/health').then(function(d) {
    cache.health = d;
    var badge = document.getElementById('statusBadge');
    badge.className = 'status-badge live';
    setText('statusText', 'LIVE');
    setText('footerStatus', 'Online');
  }).catch(function() {
    var badge = document.getElementById('statusBadge');
    badge.className = 'status-badge offline';
    setText('statusText', 'OFFLINE');
    setText('footerStatus', 'Offline');
  });

  switch (activeTab) {
    case 'live':
      safeFetch('/api/runway').then(function(d) { cache.runway = d; renderLive(); }).catch(function() {});
      safeFetch('/api/stats').then(function(d) { cache.stats = d; renderLive(); }).catch(function() {});
      safeFetch('/api/activity/feed').then(function(d) { cache.activity = d; renderLiveActivity(); }).catch(function() {});
      break;
    case 'intel':
      safeFetch('/api/stats').then(function(d) { cache.stats = d; renderIntel(); }).catch(function() {});
      safeFetch('/api/domains').then(function(d) { cache.domains = d; renderIntel(); }).catch(function() {});
      safeFetch('/api/bigrams').then(function(d) { cache.bigrams = d; renderIntel(); }).catch(function() {});
      break;
    case 'debrief':
      safeFetch('/api/conversation/metrics').then(function(d) { cache.convMetrics = d; renderDebrief(); }).catch(function() {});
      safeFetch('/api/conversation/feed').then(function(d) { cache.convFeed = d; renderDebrief(); }).catch(function() {});
      break;
    case 'arsenal':
      safeFetch('/api/sessions').then(function(d) { cache.sessions = d; renderArsenal(); }).catch(function() {});
      safeFetch('/api/config').then(function(d) { cache.config = d; renderArsenal(); }).catch(function() {});
      safeFetch('/api/runway').then(function(d) { cache.runway = d; renderArsenal(); }).catch(function() {});
      break;
    case 'recon':
      // No data endpoints yet
      break;
  }
}

function startPolling() {
  poll();
  pollTimer = setInterval(poll, 3000);
}

/* ══════════════════════════════════════════════════════════
   RENDER: LIVE TAB
   ══════════════════════════════════════════════════════════ */
function renderLive() {
  var rw = cache.runway || {};
  var st = cache.stats || {};

  // Hero metrics
  setText('hm-live-0', fmtPct(rw.tokens_saved && rw.tokens_used ? (rw.tokens_saved / (rw.tokens_used + rw.tokens_saved)) : 0));
  setText('hm-live-1', rw.tokens_saved ? fmtK(rw.tokens_saved) : '-');
  setText('hm-live-2', fmtK(rw.tokens_saved || 0));
  setText('hm-live-3', rw.delta_minutes ? fmtMin(rw.delta_minutes) : '-');

  // Hero support line
  var parts = [];
  parts.push('<span class="g">' + fmtMin(rw.runway_minutes) + '</span> runway');
  parts.push('<span class="g">' + fmtK(rw.tokens_saved || 0) + '</span> tokens saved');
  parts.push('<span class="c">' + (st.domain_count || 0) + '</span> domains');
  parts.push('<span class="c">' + (st.prompt_count || 0) + '</span> prompts');
  if (rw.counterfact_minutes) parts.push('without aOa: <span class="r">' + fmtMin(rw.counterfact_minutes) + '</span>');
  setHtml('heroSupport-live', parts.join(' &middot; '));

  // Stats grid
  var promptN = st.prompt_count || 0;
  var autotuneProgress = promptN % 50;
  setText('ls-searches', st.prompt_count || 0);
  setText('ls-files', st.index_files || 0);
  setText('ls-autotune', autotuneProgress + '/50');
  setText('ls-burn', rw.burn_rate_per_min ? fmtK(Math.round(rw.burn_rate_per_min)) + '/min' : '-');

  // Guided ratio and savings need session data
  safeFetch('/api/sessions').then(function(d) {
    cache.sessions = d;
    var sessions = d.sessions || [];
    var totalReads = 0, totalGuided = 0, totalSaved = 0;
    for (var i = 0; i < sessions.length; i++) {
      totalReads += (sessions[i].read_count || 0);
      totalGuided += (sessions[i].guided_read_count || 0);
      totalSaved += (sessions[i].tokens_saved || 0);
    }
    totalSaved += (rw.tokens_saved || 0);
    var ratio = totalReads > 0 ? totalGuided / totalReads : 0;
    setText('ls-guided', fmtPct(ratio));
    setText('hm-live-0', fmtPct(ratio));
    if (totalGuided > 0) {
      setText('ls-savings', fmtK(Math.round(totalSaved / totalGuided)));
      setText('hm-live-1', fmtK(Math.round(totalSaved / totalGuided)));
    }
  }).catch(function() {});
}

/* ── Live: Activity Table ── */
function renderLiveActivity() {
  var feed = cache.activity || {};
  var entries = feed.entries || [];
  setText('activityCount', entries.length);

  var html = '';
  for (var i = 0; i < entries.length; i++) {
    var e = entries[i];
    var actionClass = getActionPillClass(e.action);
    var sourceClass = (e.source === 'aOa') ? 'text-cyan' : 'text-dim';
    var attribClass = getAttribPillClass(e.attrib);
    var impactHtml = getImpactHtml(e.impact);

    html += '<tr>' +
      '<td class="mono text-dim" style="font-size:11px;white-space:nowrap">' + relTime(e.timestamp) + '</td>' +
      '<td><span class="pill ' + actionClass + '">' + escapeHtml(e.action) + '</span></td>' +
      '<td class="' + sourceClass + ' mono" style="font-size:11px">' + escapeHtml(e.source) + '</td>' +
      '<td><span class="pill ' + attribClass + '">' + escapeHtml(e.attrib || '-') + '</span></td>' +
      '<td class="mono" style="font-size:11px">' + impactHtml + '</td>' +
      '<td class="mono truncate" style="font-size:11px;color:var(--dim)" title="' + escapeHtml(e.target) + '">' + escapeHtml(truncPath(e.target, 50)) + '</td>' +
      '</tr>';
  }
  document.getElementById('activityTbody').innerHTML = html;
}

function getActionPillClass(action) {
  if (!action) return 'pill-dim';
  var a = action.toLowerCase();
  if (a === 'read') return 'pill-green';
  if (a === 'search') return 'pill-cyan';
  if (a === 'write') return 'pill-purple';
  if (a === 'edit') return 'pill-yellow';
  if (a === 'grep' || a === 'glob') return 'pill-red';
  if (a === 'bash') return 'pill-red';
  return 'pill-dim';
}

function getAttribPillClass(attrib) {
  if (!attrib || attrib === '-') return 'pill-dim';
  var a = attrib.toLowerCase();
  if (a === 'indexed') return 'pill-cyan';
  if (a.indexOf('guided') !== -1) return 'pill-green';
  if (a === 'productive') return 'pill-green';
  if (a === 'unguided') return 'pill-red';
  if (a === 'regex') return 'pill-yellow';
  return 'pill-dim';
}

function getImpactHtml(impact) {
  if (!impact) return '<span class="text-mute">-</span>';
  var s = String(impact);
  if (s.indexOf('saved') !== -1 || s.indexOf('+') !== -1) return '<span class="text-green">' + escapeHtml(s) + '</span>';
  if (s.indexOf('waste') !== -1 || s.indexOf('-') !== -1) return '<span class="text-red">' + escapeHtml(s) + '</span>';
  if (s.indexOf('hit') !== -1 || /\d/.test(s)) return '<span class="text-cyan">' + escapeHtml(s) + '</span>';
  return '<span class="text-dim">' + escapeHtml(s) + '</span>';
}

/* ══════════════════════════════════════════════════════════
   RENDER: INTEL TAB
   ══════════════════════════════════════════════════════════ */
function renderIntel() {
  var st = cache.stats || {};
  var dm = cache.domains || {};
  var bg = cache.bigrams || {};

  // Hero metrics
  setText('hm-intel-0', st.domain_count || 0);
  setText('hm-intel-1', st.core_count || 0);
  setText('hm-intel-2', st.term_count || 0);
  setText('hm-intel-3', st.bigram_count || 0);

  // Hero support
  var totalHits = 0;
  if (dm.domains) {
    for (var i = 0; i < dm.domains.length; i++) totalHits += (dm.domains[i].hits || 0);
  }
  var sup = [];
  sup.push('<span class="p">' + (st.domain_count || 0) + '</span> domains');
  sup.push('<span class="g">' + (st.core_count || 0) + '</span> core');
  sup.push('<span class="c">' + (st.term_count || 0) + '</span> terms');
  sup.push('<span class="b">' + (st.keyword_count || 0) + '</span> keywords');
  sup.push('<span class="y">' + (st.bigram_count || 0) + '</span> bigrams');
  sup.push('<span class="g">' + (totalHits ? totalHits.toFixed(1) : '0') + '</span> total hits');
  setHtml('heroSupport-intel', sup.join(' &middot; '));

  // Stats
  setText('is-domains', st.domain_count || 0);
  setText('is-core', st.core_count || 0);
  setText('is-terms', st.term_count || 0);
  setText('is-keywords', st.keyword_count || 0);
  setText('is-bigrams', st.bigram_count || 0);
  setText('is-totalhits', totalHits ? totalHits.toFixed(1) : '0');

  // Domain Rankings
  var domains = dm.domains || [];
  setText('domainCount', domains.length + ' domains');
  var dhtml = '';
  for (var d = 0; d < domains.length; d++) {
    var dom = domains[d];
    var termPills = '';
    if (dom.terms) {
      for (var t = 0; t < dom.terms.length; t++) {
        var tname = dom.terms[t];
        var thits = (dom.term_hits && dom.term_hits[tname]) || 0;
        var tclass = thits >= 10 ? 'term-hot' : (thits >= 3 ? 'term-warm' : 'term-cold');
        termPills += '<span class="term-pill ' + tclass + '">' + escapeHtml(tname) + '</span>';
      }
    }
    var tierClass = dom.tier === 'core' ? 'pill-green' : (dom.tier === 'context' ? 'pill-cyan' : 'pill-dim');
    dhtml += '<tr>' +
      '<td class="text-dim mono" style="font-size:11px">' + (d + 1) + '</td>' +
      '<td class="domain-name">@' + escapeHtml(dom.name) + '</td>' +
      '<td class="domain-hits">' + (dom.hits !== undefined ? dom.hits.toFixed(1) : '0') + '</td>' +
      '<td><span class="pill ' + tierClass + '">' + escapeHtml(dom.tier || '-') + '</span></td>' +
      '<td>' + termPills + '</td>' +
      '</tr>';
  }
  document.getElementById('domainTbody').innerHTML = dhtml;

  // N-gram Metrics
  renderNgramSection('ngramBigrams', bg.bigrams || {}, 'cyan');
  renderNgramSection('ngramCohitKw', bg.cohit_kw_term || {}, 'green');
  renderNgramSection('ngramCohitTd', bg.cohit_term_domain || {}, 'purple');
  var totalNgrams = (bg.count || 0) + (bg.cohit_kw_count || 0) + (bg.cohit_td_count || 0);
  setText('ngramCount', totalNgrams + ' total');
}

function renderNgramSection(containerId, data, colorClass) {
  var container = document.getElementById(containerId);
  if (!container) return;

  var items = [];
  for (var key in data) {
    if (data.hasOwnProperty(key)) items.push({ name: key, count: data[key] });
  }
  items.sort(function(a, b) { return b.count - a.count; });
  items = items.slice(0, 15);

  var maxVal = items.length > 0 ? items[0].count : 1;
  if (maxVal === 0) maxVal = 1;

  var html = '';
  for (var i = 0; i < items.length; i++) {
    var pct = Math.round((items[i].count / maxVal) * 100);
    html += '<div class="ngram-row">' +
      '<span class="ngram-name">' + escapeHtml(items[i].name) + '</span>' +
      '<span class="ngram-bar-wrap"><span class="ngram-bar ' + colorClass + '" style="width:' + pct + '%"></span></span>' +
      '<span class="ngram-count">' + items[i].count + '</span>' +
      '</div>';
  }
  if (items.length === 0) {
    html = '<div class="ngram-row"><span class="ngram-name text-mute">no data</span></div>';
  }
  container.innerHTML = html;
}

/* ══════════════════════════════════════════════════════════
   RENDER: DEBRIEF TAB
   ══════════════════════════════════════════════════════════ */
function renderDebrief() {
  var cm = cache.convMetrics || {};
  var cf = cache.convFeed || {};

  // Hero metrics
  setText('hm-debrief-0', fmtK(cm.input_tokens || 0));
  setText('hm-debrief-1', fmtK(cm.output_tokens || 0));
  setText('hm-debrief-2', fmtK(cm.cache_read_tokens || 0));
  setText('hm-debrief-3', fmtK(cm.cache_read_tokens || 0));

  // Hero support
  var totalTokens = (cm.input_tokens || 0) + (cm.output_tokens || 0);
  var sup = [];
  sup.push('<span class="c">' + (cm.turn_count || 0) + '</span> turns');
  sup.push('<span class="g">' + fmtK(totalTokens) + '</span> total tokens');
  sup.push('cache hit <span class="b">' + fmtPct(cm.cache_hit_rate || 0) + '</span>');
  sup.push('saved <span class="g">' + fmtK(cm.cache_read_tokens || 0) + '</span>');
  setHtml('heroSupport-debrief', sup.join(' &middot; '));

  // Stats
  setText('ds-input', fmtK(cm.input_tokens || 0));
  setText('ds-output', fmtK(cm.output_tokens || 0));
  setText('ds-cread', fmtK(cm.cache_read_tokens || 0));
  setText('ds-cwrite', fmtK(cm.cache_write_tokens || 0));
  setText('ds-hitrate', fmtPct(cm.cache_hit_rate || 0));
  setText('convCount', (cf.count || 0) + ' turns');

  // Conversation Feed
  var turns = cf.turns || [];
  var shouldScroll = (turns.length !== lastConvCount);
  lastConvCount = turns.length;

  // Messages column
  var mhtml = '';
  for (var i = 0; i < turns.length; i++) {
    var turn = turns[i];
    mhtml += '<div class="conv-turn-sep">Turn ' + (i + 1) + '</div>';

    if (turn.role === 'user') {
      mhtml += '<div class="conv-msg user">' +
        '<div class="conv-msg-header"><span class="conv-msg-role text-yellow">User</span>' +
        '<span class="conv-msg-meta">' + relTime(turn.timestamp) + '</span></div>' +
        '<div class="conv-msg-text">' + escapeHtml(truncText(turn.text, 500)) + '</div></div>';
    } else if (turn.role === 'assistant') {
      if (turn.thinking_text) {
        mhtml += '<div class="conv-msg thinking" onclick="this.classList.toggle(\'expanded\')">' +
          '<div class="conv-msg-header"><span class="conv-msg-role text-purple">Thinking</span>' +
          '<span class="think-toggle">click to expand</span></div>' +
          '<div class="conv-msg-text">' + escapeHtml(truncText(turn.thinking_text, 2000)) + '</div></div>';
      }
      var modelTag = turn.model ? '<span class="pill pill-dim" style="margin-left:6px">' + escapeHtml(turn.model) + '</span>' : '';
      var tokenTag = turn.output_tokens ? '<span class="conv-msg-meta" style="margin-left:auto">' + fmtK(turn.output_tokens) + ' tok</span>' : '';
      mhtml += '<div class="conv-msg assistant">' +
        '<div class="conv-msg-header"><span class="conv-msg-role text-green">Assistant</span>' +
        modelTag + tokenTag +
        '<span class="conv-msg-meta" style="margin-left:8px">' + relTime(turn.timestamp) + '</span></div>' +
        '<div class="conv-msg-text">' + escapeHtml(truncText(turn.text, 500)) + '</div></div>';
    }
  }
  mhtml += '<div class="conv-now">NOW</div>';
  document.getElementById('convMessages').innerHTML = mhtml;

  if (shouldScroll) {
    var msgContainer = document.getElementById('convMessages');
    msgContainer.scrollTop = msgContainer.scrollHeight;
  }

  // Actions column
  var ahtml = '';
  for (var j = 0; j < turns.length; j++) {
    var t = turns[j];
    var actions = t.actions || [];
    var toolNames = t.tool_names || [];
    if (actions.length === 0 && toolNames.length === 0) continue;

    ahtml += '<div class="conv-action-group">';
    ahtml += '<div class="conv-action-turn">Turn ' + (j + 1) + '</div>';

    if (actions.length === 0 && toolNames.length > 0) {
      for (var tn = 0; tn < toolNames.length; tn++) {
        ahtml += '<div class="conv-action-item">' +
          '<span class="conv-tool-chip ' + getToolChipClass(toolNames[tn]) + '">' + escapeHtml(toolNames[tn]) + '</span></div>';
      }
    }

    for (var a = 0; a < actions.length; a++) {
      var act = actions[a];
      var impCls = act.impact && (act.impact.indexOf('+') !== -1 || act.impact.indexOf('saved') !== -1) ? 'text-green' : 'text-dim';
      ahtml += '<div class="conv-action-item">' +
        '<span class="conv-tool-chip ' + getToolChipClass(act.tool) + '">' + escapeHtml(act.tool) + '</span>' +
        '<span class="conv-action-path" title="' + escapeHtml(act.target) + '">' + escapeHtml(truncPath(act.target, 30)) + '</span>' +
        (act.impact ? '<span class="conv-action-impact ' + impCls + '">' + escapeHtml(act.impact) + '</span>' : '') +
        '</div>';
    }
    ahtml += '</div>';
  }
  document.getElementById('convActions').innerHTML = ahtml;
}

function getToolChipClass(tool) {
  if (!tool) return 'chip-other';
  var t = tool.toLowerCase();
  if (t.indexOf('read') !== -1) return 'chip-read';
  if (t.indexOf('write') !== -1) return 'chip-write';
  if (t.indexOf('edit') !== -1) return 'chip-edit';
  if (t.indexOf('bash') !== -1) return 'chip-bash';
  if (t.indexOf('grep') !== -1) return 'chip-grep';
  if (t.indexOf('glob') !== -1) return 'chip-glob';
  if (t.indexOf('search') !== -1) return 'chip-search';
  return 'chip-other';
}

/* ══════════════════════════════════════════════════════════
   RENDER: ARSENAL TAB
   ══════════════════════════════════════════════════════════ */
function renderArsenal() {
  var ss = cache.sessions || {};
  var cf = cache.config || {};
  var rw = cache.runway || {};

  var sessions = ss.sessions || [];
  var sessionCount = ss.count || sessions.length;

  // Aggregate stats
  var totalSaved = rw.tokens_saved || 0;
  var totalReads = 0, totalGuidedReads = 0, totalPrompts = 0;

  for (var i = 0; i < sessions.length; i++) {
    var s = sessions[i];
    totalReads += (s.read_count || 0);
    totalGuidedReads += (s.guided_read_count || 0);
    totalPrompts += (s.prompt_count || 0);
    totalSaved += (s.tokens_saved || 0);
  }

  var overallRatio = totalReads > 0 ? totalGuidedReads / totalReads : 0;
  var totalUnguided = Math.max(0, totalReads - totalGuidedReads);
  var readVelocity = totalPrompts > 0 ? (totalReads / totalPrompts).toFixed(1) : '-';

  // Hero metrics
  setText('hm-arsenal-0', fmtK(totalSaved));
  setText('hm-arsenal-1', rw.delta_minutes ? fmtMin(rw.delta_minutes) : '-');
  setText('hm-arsenal-2', fmtK(totalUnguided * 200));
  setText('hm-arsenal-3', fmtPct(overallRatio));

  // Hero support
  var sup = [];
  sup.push('<span class="g">' + fmtK(totalSaved) + '</span> tokens saved');
  if (rw.delta_minutes) sup.push('<span class="b">' + fmtMin(rw.delta_minutes) + '</span> extended');
  sup.push('<span class="c">' + sessionCount + '</span> sessions');
  sup.push('guided <span class="g">' + fmtPct(overallRatio) + '</span>');
  if (rw.counterfact_minutes) sup.push('without aOa: <span class="r">' + fmtMin(rw.counterfact_minutes) + '</span>');
  setHtml('heroSupport-arsenal', sup.join(' &middot; '));

  // Stats
  setText('as-saved', fmtK(totalSaved));
  setText('as-cost', fmtK(totalUnguided * 200));
  setText('as-sessions', sessionCount);
  setText('as-extended', rw.delta_minutes ? fmtMin(rw.delta_minutes) : '-');
  setText('as-ratio', fmtPct(overallRatio));
  setText('as-velocity', readVelocity);
  setText('sessionCount', sessionCount + ' sessions');
  setText('savingsLabel', fmtK(totalSaved) + ' saved');

  // Savings Chart
  renderSavingsChart(sessions);

  // Session History Table
  var shtml = '';
  for (var j = 0; j < sessions.length; j++) {
    var sess = sessions[j];
    var shortId = (sess.session_id || '').substring(0, 8);
    var gr = sess.guided_ratio !== undefined ? (sess.guided_ratio * 100).toFixed(0) : 0;
    var grWidth = Math.min(100, Math.max(0, Number(gr)));
    var waste = Math.max(0, (sess.read_count || 0) - (sess.guided_read_count || 0));
    var rp = sess.prompt_count > 0 ? ((sess.read_count || 0) / sess.prompt_count).toFixed(1) : '-';

    shtml += '<tr>' +
      '<td class="session-id" title="' + escapeHtml(sess.session_id) + '">' + escapeHtml(shortId) + '</td>' +
      '<td class="mono text-dim" style="font-size:11px;white-space:nowrap">' + fmtDate(sess.start_time) + '</td>' +
      '<td class="mono text-dim" style="font-size:11px">' + fmtDuration(sess.start_time, sess.end_time) + '</td>' +
      '<td class="mono" style="font-size:11px">' + (sess.prompt_count || 0) + '</td>' +
      '<td class="mono" style="font-size:11px">' + (sess.read_count || 0) + '</td>' +
      '<td class="mono" style="font-size:11px">' + gr + '%<span class="mini-bar-wrap"><span class="mini-bar" style="width:' + grWidth + '%"></span></span></td>' +
      '<td class="mono text-green" style="font-size:11px">' + fmtK(sess.tokens_saved || 0) + '</td>' +
      '<td class="mono text-red" style="font-size:11px">' + waste + '</td>' +
      '<td class="mono text-dim" style="font-size:11px">' + rp + '</td>' +
      '</tr>';
  }
  document.getElementById('sessionTbody').innerHTML = shtml;

  // System Status
  setText('sys-uptime', cf.uptime_seconds ? fmtMin(cf.uptime_seconds / 60) : '-');
  setText('sys-db', cf.db_path ? truncPath(cf.db_path, 30) : '-');
  setText('sys-files', cf.index_files || '-');
  setText('sys-tokens', cf.index_tokens || '-');
  setText('sys-pid', cf.project_id ? cf.project_id.substring(0, 12) : '-');

  // Learning Curve
  renderLearningCurve(sessions);
}

/* ── Savings Chart (div-based bars) ── */
function renderSavingsChart(sessions) {
  var area = document.getElementById('savingsChartArea');
  if (!area) return;

  if (sessions.length === 0) {
    area.innerHTML = '<div style="flex:1;display:flex;align-items:center;justify-content:center;color:var(--mute);font-size:12px">No session data</div>';
    return;
  }

  // Group by date
  var byDate = {};
  for (var i = 0; i < sessions.length; i++) {
    var s = sessions[i];
    if (!s.start_time) continue;
    var d = new Date(s.start_time * 1000);
    var key = (d.getMonth() + 1) + '/' + d.getDate();
    if (!byDate[key]) byDate[key] = { actual: 0, counterfact: 0 };
    var inputTok = s.input_tokens || 0;
    var saved = s.tokens_saved || 0;
    byDate[key].actual += inputTok;
    byDate[key].counterfact += inputTok + saved;
  }

  var dates = Object.keys(byDate);
  if (dates.length === 0) {
    area.innerHTML = '<div style="flex:1;display:flex;align-items:center;justify-content:center;color:var(--mute);font-size:12px">No session data</div>';
    return;
  }

  var maxVal = 0;
  for (var k = 0; k < dates.length; k++) {
    var v = byDate[dates[k]];
    if (v.counterfact > maxVal) maxVal = v.counterfact;
    if (v.actual > maxVal) maxVal = v.actual;
  }
  if (maxVal === 0) maxVal = 1;

  var html = '';
  for (var b = 0; b < dates.length; b++) {
    var entry = byDate[dates[b]];
    var actualH = Math.max(2, Math.round((entry.actual / maxVal) * 140));
    var cfH = Math.max(2, Math.round((entry.counterfact / maxVal) * 140));
    html += '<div class="chart-bar-group">' +
      '<div class="chart-bars">' +
      '<div class="chart-bar counterfact" style="height:' + cfH + 'px" title="Counterfactual: ' + fmtK(entry.counterfact) + '"></div>' +
      '<div class="chart-bar actual" style="height:' + actualH + 'px" title="Actual: ' + fmtK(entry.actual) + '"></div>' +
      '</div>' +
      '<div class="chart-label">' + dates[b] + '</div>' +
      '</div>';
  }
  area.innerHTML = html;
}

/* ── Learning Curve (canvas) ── */
function renderLearningCurve(sessions) {
  var canvas = document.getElementById('learningCurveCanvas');
  if (!canvas) return;
  var ctx = canvas.getContext('2d');

  var parent = canvas.parentElement;
  var rect = parent.getBoundingClientRect();
  var dpr = window.devicePixelRatio || 1;
  var w = rect.width - 24; // padding
  var h = 120;
  canvas.width = w * dpr;
  canvas.height = h * dpr;
  canvas.style.width = w + 'px';
  canvas.style.height = h + 'px';
  ctx.scale(dpr, dpr);
  ctx.clearRect(0, 0, w, h);

  var ratios = [];
  for (var i = 0; i < sessions.length; i++) {
    if (sessions[i].guided_ratio !== undefined) ratios.push(sessions[i].guided_ratio);
  }

  if (ratios.length < 2) {
    ctx.fillStyle = getComputedStyle(document.documentElement).getPropertyValue('--mute').trim() || '#55555f';
    ctx.font = '12px Inter, sans-serif';
    ctx.textAlign = 'center';
    ctx.fillText('Need 2+ sessions for curve', w / 2, h / 2);
    return;
  }

  var padL = 10, padR = 10, padT = 10, padB = 20;
  var chartW = w - padL - padR;
  var chartH = h - padT - padB;

  var greenColor = getComputedStyle(document.documentElement).getPropertyValue('--green').trim() || '#34d399';

  // Area fill
  ctx.beginPath();
  ctx.moveTo(padL, padT + chartH);
  for (var j = 0; j < ratios.length; j++) {
    var x = padL + (j / (ratios.length - 1)) * chartW;
    var y = padT + chartH - (ratios[j] * chartH);
    ctx.lineTo(x, y);
  }
  ctx.lineTo(padL + chartW, padT + chartH);
  ctx.closePath();
  ctx.fillStyle = greenColor.replace(')', ',0.12)').replace('rgb', 'rgba');
  ctx.fill();

  // Line
  ctx.beginPath();
  for (var k = 0; k < ratios.length; k++) {
    var lx = padL + (k / (ratios.length - 1)) * chartW;
    var ly = padT + chartH - (ratios[k] * chartH);
    if (k === 0) ctx.moveTo(lx, ly);
    else ctx.lineTo(lx, ly);
  }
  ctx.strokeStyle = greenColor;
  ctx.lineWidth = 2;
  ctx.stroke();

  // Dots
  for (var m = 0; m < ratios.length; m++) {
    var dx = padL + (m / (ratios.length - 1)) * chartW;
    var dy = padT + chartH - (ratios[m] * chartH);
    ctx.beginPath();
    ctx.arc(dx, dy, 3, 0, Math.PI * 2);
    ctx.fillStyle = greenColor;
    ctx.fill();
  }

  // X-axis labels
  var muteColor = getComputedStyle(document.documentElement).getPropertyValue('--mute').trim() || '#55555f';
  ctx.fillStyle = muteColor;
  ctx.font = '9px JetBrains Mono, monospace';
  ctx.textAlign = 'center';
  for (var n = 0; n < ratios.length; n++) {
    var tx = padL + (n / (ratios.length - 1)) * chartW;
    ctx.fillText('S' + (n + 1), tx, h - 4);
  }
}

/* ── Start ── */
startPolling();

})();
