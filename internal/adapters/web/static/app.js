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
/* Change-detecting value setter: if the displayed text differs,
   update it and fire a color-matched glow that fades over 2s.
   Skips glow on first render (prev was empty or placeholder). */
function setGlow(id, val) {
  var el = document.getElementById(id);
  if (!el) return;
  var str = String(val);
  var prev = el.textContent;
  if (prev === str) return;
  el.textContent = str;
  if (prev && prev !== '-') {
    el.classList.remove('num-glow');
    void el.offsetWidth;
    el.classList.add('num-glow');
    setTimeout(function() { el.classList.remove('num-glow'); }, 60);
  }
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
function fmtTime(ms) {
  if (!ms || ms <= 0) return '-';
  if (ms >= 86400000) { var d = Math.floor(ms / 86400000); var h = Math.round((ms % 86400000) / 3600000); return d + 'd ' + h + 'h'; }
  if (ms >= 3600000) { var h2 = Math.floor(ms / 3600000); var m = Math.round((ms % 3600000) / 60000); return h2 + 'h ' + m + 'm'; }
  if (ms >= 60000) { var m2 = Math.floor(ms / 60000); var s = Math.round((ms % 60000) / 1000); return m2 + 'm ' + s + 's'; }
  if (ms >= 1000) return (ms / 1000).toFixed(1) + 's';
  return ms + 'ms';
}
function fmtPct(n) {
  if (n === undefined || n === null) return '-';
  return (Number(n) * 100).toFixed(0) + '%';
}
function fmtMin(min) {
  if (!min || min <= 0) return '-';
  if (min >= 1440) { var d = Math.floor(min / 1440); var h = Math.round((min % 1440) / 60); return d + 'd ' + h + 'h'; }
  if (min >= 60) return Math.floor(min / 60) + 'h ' + Math.round(min % 60) + 'm';
  return Math.round(min) + 'm';
}

/* ── Pricing ── */
var PRICING = {
  'claude-opus-4-6':            { input: 15.0, output: 75.0, cacheRead: 1.50 },
  'claude-sonnet-4-6':          { input: 3.0,  output: 15.0, cacheRead: 0.30 },
  'claude-haiku-4-5-20251001':  { input: 0.80, output: 4.0,  cacheRead: 0.08 },
  '_default':                    { input: 15.0, output: 75.0, cacheRead: 1.50 }
};
function getModelPricing(model) {
  if (!model) return PRICING['_default'];
  for (var key in PRICING) {
    if (key !== '_default' && model.indexOf(key) === 0) return PRICING[key];
  }
  // Prefix match: try shorter prefixes
  if (model.indexOf('opus') !== -1) return PRICING['claude-opus-4-6'];
  if (model.indexOf('sonnet') !== -1) return PRICING['claude-sonnet-4-6'];
  if (model.indexOf('haiku') !== -1) return PRICING['claude-haiku-4-5-20251001'];
  return PRICING['_default'];
}
function fmtDollar(amount) {
  if (amount === undefined || amount === null || isNaN(amount)) return '-';
  if (amount >= 1) return '$' + amount.toFixed(2);
  if (amount >= 0.01) return '$' + amount.toFixed(2);
  if (amount >= 0.001) return '$' + amount.toFixed(3);
  if (amount > 0) return '<$0.01';
  return '$0.00';
}
function calcSessionCost(input, output, cacheRead, model) {
  var p = getModelPricing(model);
  return (input * p.input + output * p.output + cacheRead * p.cacheRead) / 1000000;
}
function calcCacheSavings(cacheRead, model) {
  var p = getModelPricing(model);
  return cacheRead * (p.input - p.cacheRead) / 1000000;
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
function isNearBottom(el) {
  return el.scrollHeight - el.scrollTop - el.clientHeight < 50;
}

// Lightweight markdown renderer for assistant/thinking text
function renderMd(text) {
  if (!text) return '';
  // Escape HTML first
  var s = escapeHtml(text);
  // Code blocks: ```lang\n...\n```
  s = s.replace(/```(\w*)\n([\s\S]*?)```/g, function(m, lang, code) {
    return '<pre class="md-code-block"><code>' + code + '</code></pre>';
  });
  // Inline code: `text`
  s = s.replace(/`([^`\n]+)`/g, '<code class="md-inline-code">$1</code>');
  // Bold: **text**
  s = s.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  // Italic: *text* (but not inside **)
  s = s.replace(/(?<!\*)\*([^*]+)\*(?!\*)/g, '<em>$1</em>');
  // Line breaks
  s = s.replace(/\n/g, '<br>');
  return s;
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
  // Restart poll timer: 1s for debrief (live thinking), 3s for others
  if (pollTimer) clearInterval(pollTimer);
  var interval = (name === 'debrief') ? 1000 : 3000;
  pollTimer = setInterval(poll, interval);
  poll(); // immediate fetch for new tab
}

for (var i = 0; i < tabs.length; i++) {
  tabs[i].addEventListener('click', function() {
    switchTab(this.getAttribute('data-tab'));
  });
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

// Restore tab from URL hash (must be after HERO_STORIES is defined)
var hashTab = location.hash.replace('#', '');
if (hashTab && document.getElementById('tab-' + hashTab)) {
  switchTab(hashTab);
}

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
    // Fetch version once (doesn't change during daemon lifetime)
    if (!cache.config) {
      safeFetch('/api/config').then(function(c) {
        cache.config = c;
        if (c.version) setText('footerVersion', c.version);
      }).catch(function() {});
    }
  }).catch(function() {
    var badge = document.getElementById('statusBadge');
    badge.className = 'status-badge offline';
    setText('statusText', 'OFFLINE');
    setText('footerStatus', 'Offline');
  });

  switch (activeTab) {
    case 'live':
      Promise.all([
        safeFetch('/api/runway').then(function(d) { cache.runway = d; }).catch(function() {}),
        safeFetch('/api/stats').then(function(d) { cache.stats = d; }).catch(function() {}),
        safeFetch('/api/sessions').then(function(d) { cache.sessions = d; }).catch(function() {}),
        safeFetch('/api/conversation/metrics').then(function(d) { cache.convMetrics = d; }).catch(function() {}),
        safeFetch('/api/activity/feed').then(function(d) { cache.activity = d; }).catch(function() {})
      ]).then(function() {
        renderLive();
        renderLiveActivity();
      });
      break;
    case 'intel':
      Promise.all([
        safeFetch('/api/stats').then(function(d) { cache.stats = d; }).catch(function() {}),
        safeFetch('/api/domains').then(function(d) { cache.domains = d; }).catch(function() {}),
        safeFetch('/api/bigrams').then(function(d) { cache.bigrams = d; }).catch(function() {})
      ]).then(function() { renderIntel(); });
      break;
    case 'debrief':
      Promise.all([
        safeFetch('/api/conversation/metrics').then(function(d) { cache.convMetrics = d; }).catch(function() {}),
        safeFetch('/api/conversation/feed').then(function(d) { cache.convFeed = d; }).catch(function() {}),
        safeFetch('/api/conversation/tools').then(function(d) { cache.convTools = d; }).catch(function() {}),
        safeFetch('/api/runway').then(function(d) { cache.runway = d; }).catch(function() {})
      ]).then(function() { renderDebrief(); });
      break;
    case 'arsenal':
      Promise.all([
        safeFetch('/api/sessions').then(function(d) { cache.sessions = d; }).catch(function() {}),
        safeFetch('/api/config').then(function(d) { cache.config = d; }).catch(function() {}),
        safeFetch('/api/runway').then(function(d) { cache.runway = d; }).catch(function() {}),
        safeFetch('/api/conversation/metrics').then(function(d) { cache.convMetrics = d; }).catch(function() {})
      ]).then(function() { renderArsenal(); });
      break;
    case 'recon':
      safeFetch('/api/recon').then(function(d) {
        cache.recon = d;
        reconAnnotateInvestigated(d);
        renderRecon();
      }).catch(function() {});
      break;
  }
}

function startPolling() {
  poll();
  var interval = (activeTab === 'debrief') ? 1000 : 3000;
  pollTimer = setInterval(poll, interval);
}

/* ══════════════════════════════════════════════════════════
   RENDER: LIVE TAB
   ══════════════════════════════════════════════════════════ */
function renderLive() {
  var rw = cache.runway || {};
  var st = cache.stats || {};

  // Compute all-time totals from cached sessions + current session runway
  var ss = cache.sessions || {};
  var sessions = ss.sessions || [];
  var totalReads = 0, totalGuided = 0, totalSaved = 0, totalTimeSavedMs = 0;
  for (var i = 0; i < sessions.length; i++) {
    totalReads += (sessions[i].read_count || 0);
    totalGuided += (sessions[i].guided_read_count || 0);
    totalSaved += (sessions[i].tokens_saved || 0);
    totalTimeSavedMs += (sessions[i].time_saved_ms || 0);
  }
  // Include current session counts from runway
  totalReads += (rw.read_count || 0);
  totalGuided += (rw.guided_read_count || 0);
  totalSaved += (rw.tokens_saved || 0);
  totalTimeSavedMs += (rw.time_saved_ms || 0);
  var ratio = totalReads > 0 ? totalGuided / totalReads : 0;

  // Cost: prefer real total_cost_usd from status line, fallback to estimate
  var pricing = getModelPricing(rw.model);
  var costSaved = totalSaved * pricing.input / 1000000;
  var realCost = rw.total_cost_usd || 0;

  // Context: prefer real remaining_pct from status line hook
  var ctxRemainingPct = rw.ctx_remaining_pct || 0;
  var ctxUsedPct = rw.ctx_used_pct || 0;
  if (!ctxRemainingPct && ctxUsedPct > 0) ctxRemainingPct = 100 - ctxUsedPct;
  var ctxUsed = rw.ctx_used || 0;
  var ctxMax = rw.ctx_max || rw.context_window_max || 0;

  // Hero metrics — always visible, show "-" when no data
  setGlow('hm-live-0', totalSaved > 0 ? fmtK(totalSaved) : '-');
  setGlow('hm-live-1', costSaved > 0 ? fmtDollar(costSaved) : '-');
  setGlow('hm-live-2', totalTimeSavedMs > 0 ? fmtTime(totalTimeSavedMs) : '-');
  var promptN = st.prompt_count || 0;
  var autotuneProgress = promptN % 50;
  setGlow('hm-live-3', autotuneProgress + '/50');

  // Hero support line — only show parts that have data
  var parts = [];
  if (rw.burn_rate_per_min) parts.push('burn <span class="r">' + fmtK(Math.round(rw.burn_rate_per_min)) + '/m</span>');
  if (ctxRemainingPct > 0) {
    var ctxDisplay = ctxRemainingPct.toFixed(0) + '% left';
    if (ctxUsed > 0 && ctxMax > 0) ctxDisplay = fmtK(ctxUsed) + '/' + fmtK(ctxMax) + ' (' + ctxRemainingPct.toFixed(0) + '% left)';
    parts.push('ctx <span class="c">' + ctxDisplay + '</span>');
  }
  if (rw.model) parts.push('<span class="p">' + rw.model + '</span>');
  if (st.prompt_count) parts.push('turn <span class="c">' + st.prompt_count + '</span>');
  if (rw.shadow_total_saved > 0) parts.push('shadow <span class="g">' + fmtK(rw.shadow_total_saved) + ' saved</span>');
  setHtml('heroSupport-live', parts.length > 0 ? parts.join(' &middot; ') : '');

  // Stats grid — top row: situation, bottom row: aOa value
  setGlow('ls-ctxused', ctxUsedPct > 0 ? ctxUsedPct.toFixed(0) + '%' : '-');
  setGlow('ls-burn', rw.burn_rate_per_min ? fmtK(Math.round(rw.burn_rate_per_min)) + '/min' : '-');
  setGlow('ls-cost', realCost > 0 ? fmtDollar(realCost) : '-');
  setGlow('ls-guided', fmtPct(ratio));
  setGlow('ls-shadow', rw.shadow_total_saved > 0 ? fmtK(rw.shadow_total_saved) + ' tok' : '-');

  // Cache savings $ — dollars saved by prompt cache serving tokens at reduced rate
  var cacheSavings = 0;
  var cm = cache.convMetrics || {};
  var cacheReadTokens = cm.cache_read_tokens || 0;
  if (cacheReadTokens > 0) cacheSavings = calcCacheSavings(cacheReadTokens, rw.model);
  for (var j = 0; j < sessions.length; j++) {
    if (sessions[j].cache_read_tokens) cacheSavings += calcCacheSavings(sessions[j].cache_read_tokens, sessions[j].model || rw.model);
  }
  setGlow('ls-cachesave', cacheSavings > 0 ? fmtDollar(cacheSavings) : '-');
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
    var sourceClass = (e.source === 'aOa') ? 'text-blue' : 'text-dim';
    var isUnguided = e.attrib === 'unguided';
    var isGuided = e.attrib && e.attrib.indexOf('guided') !== -1 && !isUnguided;
    var isSearch = e.action === 'Search';
    var impactHtml = getImpactHtml(e.impact, isUnguided, isGuided, isSearch);
    var targetHtml = renderTarget(e.target, isUnguided);

    var learnedHtml = e.learned ? '<span class="pill pill-green">' + escapeHtml(e.learned) + '</span>' : '';

    html += '<tr>' +
      '<td class="mono text-dim" style="font-size:11px;white-space:nowrap">' + relTime(e.timestamp) + '</td>' +
      '<td><span class="pill ' + actionClass + '">' + escapeHtml(e.action) + '</span></td>' +
      '<td class="' + sourceClass + ' mono" style="font-size:11px">' + escapeHtml(e.source) + '</td>' +
      '<td>' + renderAttribPills(e.attrib) + '</td>' +
      '<td class="mono" style="font-size:11px">' + impactHtml + '</td>' +
      '<td>' + learnedHtml + '</td>' +
      '<td class="mono truncate" style="font-size:11px" title="' + escapeHtml(e.target) + '">' + targetHtml + '</td>' +
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

var CREATIVE_WORDS = {'crafted':1,'authored':1,'forged':1,'innovated':1};
var LEARN_WORDS = {'trained':1,'fine-tuned':1,'calibrated':1,'converged':1,'reinforced':1,'optimized':1,'weighted':1,'adapted':1};
function getAttribPillClass(attrib) {
  if (!attrib || attrib === '-') return 'pill-dim';
  var a = attrib.toLowerCase();
  if (a === 'unguided') return 'pill-red';
  if (CREATIVE_WORDS[a]) return 'pill-purple';
  if (LEARN_WORDS[a]) return 'pill-green';
  if (a === 'indexed') return 'pill-cyan';
  if (a.indexOf('guided') !== -1) return 'pill-green';
  if (a === 'regex' || a === 'multi-and' || a === 'multi-or') return 'pill-yellow';
  return 'pill-dim';
}
function renderAttribPills(attrib) {
  if (!attrib || attrib === '-') return '<span class="pill pill-dim">-</span>';
  return '<span class="pill ' + getAttribPillClass(attrib) + '">' + escapeHtml(attrib) + '</span>';
}

function getImpactHtml(impact, isUnguided, isGuided, isSearch) {
  if (!impact) return '<span class="text-mute">-</span>';
  var s = String(impact);
  if (s === '-') return '<span class="text-mute">-</span>';
  // Attrib-driven: unguided = red cost, guided = green savings
  if (isUnguided) return '<span class="text-red">' + escapeHtml(s) + '</span>';
  if (isGuided) return '<span class="text-green">' + escapeHtml(s) + '</span>';
  // Highlight numbers, keep the rest dim
  return highlightNumbers(s);
}
function highlightNumbers(s) {
  return '<span class="text-dim">' + s.replace(/(\d+\.?\d*)/g, '</span><span class="text-cyan">$1</span><span class="text-dim">') + '</span>';
}
function renderTarget(target, isUnguided) {
  if (!target) return '<span class="text-dim">-</span>';
  var t = truncPath(target, 50);
  if (isUnguided) return '<span style="color:var(--red)">' + escapeHtml(t) + '</span>';
  // aOa grep/egrep: "aOa" blue, "grep"/"egrep" green, rest dim
  var m = t.match(/^(aOa)\s+(e?grep)(.*)$/);
  if (m) {
    return '<span style="color:var(--blue)">' + escapeHtml(m[1]) + '</span> ' +
           '<span style="color:var(--green)">' + escapeHtml(m[2]) + '</span>' +
           '<span class="text-dim">' + escapeHtml(m[3]) + '</span>';
  }
  return '<span class="text-dim">' + escapeHtml(t) + '</span>';
}

/* ══════════════════════════════════════════════════════════
   RENDER: INTEL TAB
   ══════════════════════════════════════════════════════════ */
var prevDomainData = null;
var prevTermHitsMap = {};
var prevBigramData = null;

function renderIntel() {
  var st = cache.stats || {};
  var dm = cache.domains || {};
  var bg = cache.bigrams || {};

  var domainCount = st.domain_count || 0;
  var coreCount = st.core_count || 0;
  var termCount = st.term_count || 0;
  var kwCount = st.keyword_count || 0;
  var promptCount = st.prompt_count || 0;

  // Computed metrics
  var domainVelocity = promptCount > 0 ? (domainCount / promptCount).toFixed(2) : '-';
  var kwDomainRate = kwCount > 0 ? (domainCount / kwCount * 100).toFixed(0) + '%' : '-';
  var learningRate = promptCount > 0 ? (kwCount / promptCount).toFixed(1) : '-';

  // Term coherence: count terms that have domains / total terms
  var termsWithDomains = 0;
  if (dm.domains) {
    var termSet = {};
    for (var di = 0; di < dm.domains.length; di++) {
      var dTerms = dm.domains[di].terms || [];
      for (var ti = 0; ti < dTerms.length; ti++) {
        termSet[dTerms[ti]] = true;
      }
    }
    termsWithDomains = Object.keys(termSet).length;
  }
  var termCoherence = termCount > 0 ? (termsWithDomains / termCount * 100).toFixed(0) + '%' : '-';

  // Hero metrics
  setGlow('hm-intel-0', coreCount);
  setGlow('hm-intel-1', domainVelocity);
  setGlow('hm-intel-2', termCoherence);
  setGlow('hm-intel-3', kwDomainRate);

  // Hero support
  var totalHits = 0;
  if (dm.domains) {
    for (var i = 0; i < dm.domains.length; i++) totalHits += (dm.domains[i].hits || 0);
  }
  var sup = [];
  sup.push('<span class="g">' + coreCount + '</span> mastered');
  sup.push('<span class="c">' + termCount + '</span> concepts');
  sup.push('<span class="b">' + kwCount + '</span> vocabulary');
  sup.push('<span class="g">' + (totalHits ? totalHits.toFixed(1) : '0') + '</span> evidence');
  setHtml('heroSupport-intel', sup.join(' &middot; '));

  // Stats — pipeline: observe → extract → structure → master → pattern → prove
  setGlow('is-observed', st.file_hit_count || 0);
  setGlow('is-vocabulary', kwCount);
  setGlow('is-concepts', termCount);
  setGlow('is-domains', domainCount);
  setGlow('is-patterns', st.bigram_count || 0);
  setGlow('is-evidence', totalHits ? totalHits.toFixed(1) : '0');

  // Domain Rankings (with change-tracking visual effects)
  renderDomains(dm);

  // N-gram Metrics (with change-tracking visual effects)
  renderBigrams(bg);
  var totalNgrams = (bg.count || 0) + (bg.cohit_kw_count || 0) + (bg.cohit_td_count || 0);
  setText('ngramCount', totalNgrams + ' total');
}

/* ── Domain rendering with flash/glow effects ── */
function detectTermChanges(domainName, termHits) {
  var changed = [];
  var th = termHits || {};
  for (var term in th) {
    var key = domainName + ':' + term;
    var prev = prevTermHitsMap[key] || 0;
    if (th[term] > prev) changed.push(term);
  }
  return changed;
}

function updateTermHitsCache(domains) {
  prevTermHitsMap = {};
  for (var i = 0; i < domains.length; i++) {
    var d = domains[i];
    var th = d.term_hits || {};
    for (var term in th) {
      prevTermHitsMap[d.name + ':' + term] = th[term];
    }
  }
}

function glowTermPills(container, domainName, changedTerms) {
  var row = container.querySelector('[data-dname="' + CSS.escape(domainName) + '"]');
  if (!row) return;
  // Soft text glow on the hits cell
  var hitsCell = row.querySelector('.domain-hits');
  if (hitsCell) {
    hitsCell.classList.remove('text-glow'); void hitsCell.offsetWidth;
    hitsCell.classList.add('text-glow');
  }
  // Soft diffuse glow on each changed term pill — .lit adds glow + green tint,
  // then removing it after a frame lets the 2.5s CSS transition fade it back
  for (var i = 0; i < changedTerms.length; i++) {
    var pill = row.querySelector('[data-term="' + CSS.escape(changedTerms[i]) + '"]');
    if (pill) {
      pill.classList.add('lit');
      setTimeout((function(p) { return function() { p.classList.remove('lit'); }; })(pill), 60);
    }
  }
}

function renderDomains(domainsResult) {
  var list = (domainsResult.domains || []).slice(0, 24);
  setText('domainCount', list.length + ' domains');

  if (list.length === 0) {
    document.getElementById('domainTbody').innerHTML =
      '<tr><td colspan="4" class="text-mute" style="text-align:center;padding:20px">No domains yet</td></tr>';
    prevDomainData = domainsResult;
    return;
  }

  var prevList = prevDomainData ? (prevDomainData.domains || []) : [];
  var structureChanged = !prevDomainData || list.length !== prevList.length;
  if (!structureChanged) {
    for (var i = 0; i < list.length; i++) {
      if (list[i].name !== prevList[i].name) { structureChanged = true; break; }
    }
  }

  var prevHitsMap = {};
  for (var p = 0; p < prevList.length; p++) prevHitsMap[prevList[p].name] = prevList[p].hits;

  var tbody = document.getElementById('domainTbody');

  if (structureChanged) {
    // Full rebuild
    var dhtml = '';
    for (var d = 0; d < list.length; d++) {
      var dom = list[d];
      var termsHtml = '';
      if (dom.terms && dom.terms.length > 0) {
        var shown = dom.terms.slice(0, 10);
        for (var t = 0; t < shown.length; t++) {
          termsHtml += '<span class="term-pill" data-term="' + escapeHtml(shown[t]) + '">' + escapeHtml(shown[t]) + '</span>';
        }
        if (dom.terms.length > 10) {
          termsHtml += '<span class="term-pill" style="background:var(--border-subtle);color:var(--dim)">+' + (dom.terms.length - 10) + '</span>';
        }
      }
      dhtml += '<tr data-dname="' + escapeHtml(dom.name) + '">' +
        '<td class="text-dim mono" style="font-size:11px">' + (d + 1) + '</td>' +
        '<td class="domain-name">@' + escapeHtml(dom.name) + '</td>' +
        '<td class="domain-hits" data-dhits="' + escapeHtml(dom.name) + '">' + (dom.hits !== undefined ? dom.hits.toFixed(1) : '0') + '</td>' +
        '<td class="d-terms">' + termsHtml + '</td>' +
        '</tr>';
    }
    tbody.innerHTML = dhtml;

    // Flash terms that changed
    for (var f = 0; f < list.length; f++) {
      var changed = detectTermChanges(list[f].name, list[f].term_hits);
      if (changed.length > 0) {
        glowTermPills(tbody, list[f].name, changed);
      } else if (prevHitsMap[list[f].name] !== undefined && prevHitsMap[list[f].name] !== list[f].hits) {
        var row = tbody.querySelector('[data-dname="' + CSS.escape(list[f].name) + '"]');
        if (row) {
          var hc = row.querySelector('.domain-hits');
          if (hc) { hc.classList.remove('text-glow'); void hc.offsetWidth; hc.classList.add('text-glow'); }
        }
      }
    }
  } else {
    // Surgical update — only update changed hits + flash specific terms
    for (var u = 0; u < list.length; u++) {
      var udom = list[u];
      var urow = tbody.querySelector('[data-dname="' + CSS.escape(udom.name) + '"]');
      if (!urow) continue;
      var uhc = urow.querySelector('[data-dhits="' + CSS.escape(udom.name) + '"]');
      if (uhc) {
        var newVal = udom.hits !== undefined ? udom.hits.toFixed(1) : '0';
        if (uhc.textContent !== newVal) {
          uhc.textContent = newVal;
          uhc.classList.remove('text-glow'); void uhc.offsetWidth;
          uhc.classList.add('text-glow');
        }
      }
      var uchanged = detectTermChanges(udom.name, udom.term_hits);
      if (uchanged.length > 0) glowTermPills(tbody, udom.name, uchanged);
    }
  }

  updateTermHitsCache(list);
  prevDomainData = domainsResult;
}

/* ── N-gram rendering with flash/glow effects ── */
function buildNgramSection(title, sorted, barClass, prefix) {
  var maxVal = sorted.length > 0 ? sorted[0][1] : 1;
  if (maxVal === 0) maxVal = 1;
  var html = '<div class="ngram-section"><div class="ngram-section-title">' + title + '</div>';
  for (var i = 0; i < sorted.length; i++) {
    var pct = (sorted[i][1] / maxVal * 100).toFixed(1);
    var key = prefix + ':' + sorted[i][0];
    html += '<div class="ngram-row" data-ngkey="' + escapeHtml(key) + '">' +
      '<span class="ngram-name">' + escapeHtml(sorted[i][0]) + '</span>' +
      '<span class="ngram-bar-track"><span class="ngram-bar-fill ' + barClass + '" style="width:' + pct + '%"></span></span>' +
      '<span class="ngram-count">' + sorted[i][1] + '</span></div>';
  }
  if (sorted.length === 0) {
    html += '<div class="ngram-row"><span class="ngram-name text-mute">no data</span></div>';
  }
  html += '</div>';
  return html;
}

function renderBigrams(bigramsResult) {
  var bg = bigramsResult.bigrams || {};
  var ckw = bigramsResult.cohit_kw_term || {};
  var ctd = bigramsResult.cohit_term_domain || {};

  var bgSorted = Object.keys(bg).map(function(k) { return [k, bg[k]]; })
    .sort(function(a, b) { return b[1] - a[1]; }).slice(0, 10);
  var ckSorted = Object.keys(ckw).map(function(k) { return [k, ckw[k]]; })
    .sort(function(a, b) { return b[1] - a[1]; }).slice(0, 5);
  var ctSorted = Object.keys(ctd).map(function(k) { return [k, ctd[k]]; })
    .sort(function(a, b) { return b[1] - a[1]; }).slice(0, 5);

  // Signature to detect structural changes
  var sig = bgSorted.map(function(e) { return e[0]; }).join(',') + '|' +
    ckSorted.map(function(e) { return e[0]; }).join(',') + '|' +
    ctSorted.map(function(e) { return e[0]; }).join(',');
  var prevSig = prevBigramData ? prevBigramData._sig : null;

  // Previous value map for pulse detection
  var prevMap = {};
  if (prevBigramData) {
    var pb = prevBigramData.bigrams || {};
    var pck = prevBigramData.cohit_kw_term || {};
    var pct = prevBigramData.cohit_term_domain || {};
    for (var k1 in pb) prevMap['bg:' + k1] = pb[k1];
    for (var k2 in pck) prevMap['ck:' + k2] = pck[k2];
    for (var k3 in pct) prevMap['ct:' + k3] = pct[k3];
  }

  // Target the three containers
  var containers = [
    { id: 'ngramBigrams', sorted: bgSorted, bar: 'bar-cyan', prefix: 'bg', title: 'BIGRAMS' },
    { id: 'ngramCohitKw', sorted: ckSorted, bar: 'bar-green', prefix: 'ck', title: 'COHITS: KW \u2192 TERM' },
    { id: 'ngramCohitTd', sorted: ctSorted, bar: 'bar-purple', prefix: 'ct', title: 'COHITS: TERM \u2192 DOMAIN' }
  ];

  if (sig !== prevSig) {
    // Full rebuild — write HTML into each container
    for (var c = 0; c < containers.length; c++) {
      var ct = containers[c];
      var el = document.getElementById(ct.id);
      if (!el) continue;
      // buildNgramSection returns a wrapping div, extract the inner rows
      var maxVal = ct.sorted.length > 0 ? ct.sorted[0][1] : 1;
      if (maxVal === 0) maxVal = 1;
      var html = '';
      for (var r = 0; r < ct.sorted.length; r++) {
        var pct2 = (ct.sorted[r][1] / maxVal * 100).toFixed(1);
        var ngkey = ct.prefix + ':' + ct.sorted[r][0];
        html += '<div class="ngram-row" data-ngkey="' + escapeHtml(ngkey) + '">' +
          '<span class="ngram-name">' + escapeHtml(ct.sorted[r][0]) + '</span>' +
          '<span class="ngram-bar-track"><span class="ngram-bar-fill ' + ct.bar + '" style="width:' + pct2 + '%"></span></span>' +
          '<span class="ngram-count">' + ct.sorted[r][1] + '</span></div>';
      }
      if (ct.sorted.length === 0) {
        html = '<div class="ngram-row"><span class="ngram-name text-mute">no data</span></div>';
      }
      el.innerHTML = html;
    }

    // Pulse changed values
    var allSorted = [bgSorted, ckSorted, ctSorted];
    var prefixes = ['bg', 'ck', 'ct'];
    var parentIds = ['ngramBigrams', 'ngramCohitKw', 'ngramCohitTd'];
    for (var si = 0; si < allSorted.length; si++) {
      var parent = document.getElementById(parentIds[si]);
      if (!parent) continue;
      for (var ri = 0; ri < allSorted[si].length; ri++) {
        var nkey = prefixes[si] + ':' + allSorted[si][ri][0];
        if (prevMap[nkey] !== undefined && prevMap[nkey] !== allSorted[si][ri][1]) {
          var nrow = parent.querySelector('[data-ngkey="' + CSS.escape(nkey) + '"]');
          if (nrow) {
            var ncnt = nrow.querySelector('.ngram-count');
            if (ncnt) { ncnt.classList.remove('soft-glow'); void ncnt.offsetWidth; ncnt.classList.add('soft-glow'); }
            var nname = nrow.querySelector('.ngram-name');
            if (nname) { nname.classList.remove('text-glow'); void nname.offsetWidth; nname.classList.add('text-glow'); }
          }
        }
      }
    }
  } else {
    // Surgical update — only update changed counts + flash
    var allSorted2 = [bgSorted, ckSorted, ctSorted];
    var prefixes2 = ['bg', 'ck', 'ct'];
    var parentIds2 = ['ngramBigrams', 'ngramCohitKw', 'ngramCohitTd'];
    for (var si2 = 0; si2 < allSorted2.length; si2++) {
      var parent2 = document.getElementById(parentIds2[si2]);
      if (!parent2) continue;
      var maxV = allSorted2[si2].length > 0 ? allSorted2[si2][0][1] : 1;
      for (var ri2 = 0; ri2 < allSorted2[si2].length; ri2++) {
        var nkey2 = prefixes2[si2] + ':' + allSorted2[si2][ri2][0];
        var nrow2 = parent2.querySelector('[data-ngkey="' + CSS.escape(nkey2) + '"]');
        if (!nrow2) continue;
        var cntEl = nrow2.querySelector('.ngram-count');
        if (cntEl && cntEl.textContent !== String(allSorted2[si2][ri2][1])) {
          cntEl.textContent = allSorted2[si2][ri2][1];
          cntEl.classList.remove('soft-glow'); void cntEl.offsetWidth; cntEl.classList.add('soft-glow');
          var nname2 = nrow2.querySelector('.ngram-name');
          if (nname2) { nname2.classList.remove('text-glow'); void nname2.offsetWidth; nname2.classList.add('text-glow'); }
        }
        var barEl = nrow2.querySelector('.ngram-bar-fill');
        if (barEl) barEl.style.width = (allSorted2[si2][ri2][1] / maxV * 100).toFixed(1) + '%';
      }
    }
  }

  bigramsResult._sig = sig;
  prevBigramData = bigramsResult;
}

/* ══════════════════════════════════════════════════════════
   RENDER: DEBRIEF TAB
   ══════════════════════════════════════════════════════════ */
function renderDebrief() {
  var cm = cache.convMetrics || {};
  var cf = cache.convFeed || {};
  var rw = cache.runway || {};
  var ct = cache.convTools || {};

  var model = rw.model || '';
  var inputTokens = cm.input_tokens || 0;
  var outputTokens = cm.output_tokens || 0;
  var cacheRead = cm.cache_read_tokens || 0;
  var turnCount = cm.turn_count || 0;

  // Pricing calculations: prefer real cost from status line hook
  var cacheSavings = calcCacheSavings(cacheRead, model);
  var sessionCost = rw.total_cost_usd > 0 ? rw.total_cost_usd : calcSessionCost(inputTokens, outputTokens, cacheRead, model);
  var costPerTurn = turnCount > 0 ? sessionCost / turnCount : 0;

  var rawTurns = (cf.turns || []);

  // Shared char counts — used by both throughput and conv speed.
  // Walk turns once: sum text chars, thinking chars, and tool result chars.
  var totalTextChars = 0;   // user text + assistant text + thinking
  var totalResultChars = 0; // tool result content
  for (var ri = 0; ri < rawTurns.length; ri++) {
    var rt = rawTurns[ri];
    if (rt.text) totalTextChars += rt.text.length;
    if (rt.thinking_text) totalTextChars += rt.thinking_text.length;
    var rActions = rt.actions || [];
    for (var ra = 0; ra < rActions.length; ra++) {
      totalResultChars += (rActions[ra].result_chars || 0);
    }
  }
  var textBasedTokens = Math.round(totalTextChars / 4);
  var resultTokens = Math.round(totalResultChars / 4);

  // Throughput — total content tokens / session wall time.
  // Uses max(API outputTokens, text estimate) so throughput >= conv speed.
  // API outputTokens is more accurate for model output; text estimate captures
  // user input that outputTokens doesn't include.
  var throughputTokens = Math.max(outputTokens, textBasedTokens) + resultTokens;
  var throughput = '-';
  var elapsedSec = 0;
  if (rw.total_duration_ms > 0) {
    elapsedSec = rw.total_duration_ms / 1000;
  } else if (cm.session_start_ts > 0) {
    elapsedSec = (Date.now() / 1000) - cm.session_start_ts;
  } else if (rawTurns.length >= 2) {
    var tsMin = Infinity, tsMax = 0;
    for (var ti = 0; ti < rawTurns.length; ti++) {
      if (rawTurns[ti].timestamp > 0) {
        if (rawTurns[ti].timestamp < tsMin) tsMin = rawTurns[ti].timestamp;
        if (rawTurns[ti].timestamp > tsMax) tsMax = rawTurns[ti].timestamp;
      }
    }
    if (tsMax > tsMin) elapsedSec = tsMax - tsMin;
  }
  if (elapsedSec > 5 && throughputTokens > 0) {
    throughput = (throughputTokens / elapsedSec).toFixed(1) + '/s';
  }

  // Conv Speed — visible conversation text (chars/4) / wall time.
  // Only user text + assistant text + thinking. Tool results are infrastructure
  // data, not conversation — they go in throughput only.
  var convSpeed = '-';
  var convChars = totalTextChars;
  var convTokens = convChars / 4;
  if (rawTurns.length >= 2) {
    var ctsMin = Infinity, ctsMax = 0;
    for (var cj = 0; cj < rawTurns.length; cj++) {
      if (rawTurns[cj].timestamp > 0) {
        if (rawTurns[cj].timestamp < ctsMin) ctsMin = rawTurns[cj].timestamp;
        if (rawTurns[cj].timestamp > ctsMax) ctsMax = rawTurns[cj].timestamp;
      }
    }
    var convElapsed = ctsMax - ctsMin;
    if (convElapsed > 5 && convTokens > 0) {
      convSpeed = (convTokens / convElapsed).toFixed(1) + '/s';
    }
  }

  var totalDurMs = 0, durCount = 0;
  for (var td = 0; td < rawTurns.length; td++) {
    if (rawTurns[td].duration_ms > 0) { totalDurMs += rawTurns[td].duration_ms; durCount++; }
  }
  var avgTurnDur = durCount > 0 ? fmtTime(Math.round(totalDurMs / durCount)) : '-';

  // Tool density
  var toolTotal = (ct.total_count || 0);
  var toolDensity = turnCount > 0 ? (toolTotal / turnCount).toFixed(1) : '-';

  // Amplification: total assistant chars / total user chars
  var totalAssistChars = 0, totalUserChars = 0;
  for (var ac = 0; ac < rawTurns.length; ac++) {
    var t = rawTurns[ac];
    if (t.role === 'assistant' && t.text) totalAssistChars += t.text.length;
    if (t.role === 'user' && t.text) totalUserChars += t.text.length;
  }
  var amplification = totalUserChars > 0 ? (totalAssistChars / totalUserChars).toFixed(1) + 'x' : '-';

  // Model mix: find most common model
  var modelCounts = {};
  for (var mc = 0; mc < rawTurns.length; mc++) {
    if (rawTurns[mc].model) {
      var m = rawTurns[mc].model;
      modelCounts[m] = (modelCounts[m] || 0) + 1;
    }
  }
  var primaryModel = '-';
  var maxModelCount = 0;
  for (var mk in modelCounts) {
    if (modelCounts[mk] > maxModelCount) { maxModelCount = modelCounts[mk]; primaryModel = mk; }
  }
  // Shorten model name for display
  var modelMix = primaryModel;
  if (modelMix.length > 12) {
    if (modelMix.indexOf('opus') !== -1) modelMix = 'opus';
    else if (modelMix.indexOf('sonnet') !== -1) modelMix = 'sonnet';
    else if (modelMix.indexOf('haiku') !== -1) modelMix = 'haiku';
    else modelMix = modelMix.substring(0, 12);
  }

  // Hero metrics
  setGlow('hm-debrief-0', fmtK(inputTokens));
  setGlow('hm-debrief-1', fmtK(outputTokens));
  setGlow('hm-debrief-2', cacheSavings > 0 ? fmtDollar(cacheSavings) : '-');
  setGlow('hm-debrief-3', costPerTurn > 0 ? fmtDollar(costPerTurn) : '-');

  // Hero support — only show parts that have data
  var totalTokens = inputTokens + outputTokens;
  var sup = [];
  if (turnCount > 0) sup.push('<span class="c">' + turnCount + '</span> exchanges');
  if (totalTokens > 0) sup.push('<span class="g">' + fmtK(totalTokens) + '</span> total tokens');
  if (cacheSavings > 0) sup.push('cache saved <span class="b">' + fmtDollar(cacheSavings) + '</span>');
  if (throughput !== '-') sup.push('pace <span class="g">' + convSpeed + '</span>');
  setHtml('heroSupport-debrief', sup.length > 0 ? sup.join(' &middot; ') : '');

  // Stats — dialogue → pace → depth → leverage → amplification → engine
  setGlow('ds-flow', throughput);
  setGlow('ds-pace', convSpeed);
  setGlow('ds-turntime', avgTurnDur);
  setGlow('ds-leverage', toolDensity);
  setGlow('ds-amplify', amplification);
  setGlow('ds-engine', modelMix);
  setText('convCount', (cf.count || 0) + ' turns');

  // Conversation Feed — reverse to chronological, pair user+assistant into exchanges
  var rawTurns = cf.turns || [];
  var turns = rawTurns.slice().reverse();

  // Check if containers are near bottom before re-rendering
  var msgContainer = document.getElementById('convMessages');
  var actContainer = document.getElementById('convActions');
  var msgWasNear = msgContainer ? isNearBottom(msgContainer) : true;
  var actWasNear = actContainer ? isNearBottom(actContainer) : true;

  // Group into exchanges: each exchange is { user: turn|null, assistant: turn|null }
  var exchanges = [];
  var i = 0;
  while (i < turns.length) {
    var ex = { user: null, assistant: null };
    if (turns[i].role === 'user') {
      ex.user = turns[i];
      i++;
      if (i < turns.length && turns[i].role === 'assistant') {
        ex.assistant = turns[i];
        i++;
      }
    } else if (turns[i].role === 'assistant') {
      ex.assistant = turns[i];
      i++;
    } else {
      i++;
      continue;
    }
    exchanges.push(ex);
  }

  setText('convCount', exchanges.length + ' turns');

  // Messages column
  var mhtml = '';
  for (var e = 0; e < exchanges.length; e++) {
    var ex = exchanges[e];
    var turnNum = e + 1;
    mhtml += '<div class="conv-turn-sep">Turn ' + turnNum + '</div>';

    // User message — estimate tokens from text length (bytes/4)
    if (ex.user) {
      var userTokEst = ex.user.text ? Math.ceil(ex.user.text.length / 4) : 0;
      var userTokTag = userTokEst > 0 ? '<span class="conv-msg-meta" style="margin-left:auto">~' + fmtK(userTokEst) + ' tok</span>' : '';
      mhtml += '<div class="conv-msg user">' +
        '<div class="conv-msg-header"><span class="conv-msg-role text-yellow">User</span>' +
        userTokTag +
        '<span class="conv-msg-meta">' + relTime(ex.user.timestamp) + '</span></div>' +
        '<div class="conv-msg-text">' + escapeHtml(truncText(ex.user.text, 2000)) + '</div></div>';
    }

    // Assistant: thinking (nested lines, always visible) + response
    if (ex.assistant) {
      if (ex.assistant.thinking_text) {
        var thinkTokEst = Math.ceil(ex.assistant.thinking_text.length / 4);
        var thoughts = ex.assistant.thinking_text.split('\n').filter(function(t) { return t.trim(); });
        mhtml += '<div class="conv-thinking-block">' +
          '<div class="conv-thinking-header"><span class="conv-msg-role text-purple">Thinking</span>' +
          '<span class="conv-msg-meta">' + thoughts.length + ' thought' + (thoughts.length !== 1 ? 's' : '') + '</span>' +
          '<span class="conv-msg-meta" style="margin-left:auto">~' + fmtK(thinkTokEst) + ' tok</span></div>';
        for (var th = 0; th < thoughts.length; th++) {
          mhtml += '<div class="conv-thought-line">' + renderMd(truncText(thoughts[th], 500)) + '</div>';
        }
        mhtml += '</div>';
      }
      var modelTag = ex.assistant.model ? '<span class="pill pill-dim" style="margin-left:6px">' + escapeHtml(ex.assistant.model) + '</span>' : '';
      var tokenTag = ex.assistant.output_tokens ? '<span class="conv-msg-meta" style="margin-left:auto">' + fmtK(ex.assistant.output_tokens) + ' tok</span>' : '';
      mhtml += '<div class="conv-msg assistant">' +
        '<div class="conv-msg-header"><span class="conv-msg-role text-green">Assistant</span>' +
        modelTag + tokenTag +
        '<span class="conv-msg-meta" style="margin-left:8px">' + relTime(ex.assistant.timestamp) + '</span></div>' +
        '<div class="conv-msg-text">' + renderMd(truncText(ex.assistant.text, 2000)) + '</div></div>';
    }
  }
  mhtml += '<div class="conv-now"><span class="conv-now-line"></span><span class="conv-now-dot"></span><span class="conv-now-text">NOW</span><span class="conv-now-line"></span></div>';
  document.getElementById('convMessages').innerHTML = mhtml;

  // Auto-scroll if user was near bottom
  if (msgWasNear && msgContainer) {
    msgContainer.scrollTop = msgContainer.scrollHeight;
  }

  // Actions column — keyed by exchange number
  var ahtml = '';
  for (var e2 = 0; e2 < exchanges.length; e2++) {
    var asTurn = exchanges[e2].assistant;
    if (!asTurn) continue;
    var actions = asTurn.actions || [];
    var toolNames = asTurn.tool_names || [];
    if (actions.length === 0 && toolNames.length === 0) continue;

    ahtml += '<div class="conv-action-group">';
    ahtml += '<div class="conv-action-header">' +
      '<span class="conv-action-turn">Turn ' + (e2 + 1) + '</span>' +
      '<span class="act-col-head" title="aOa savings">Save</span>' +
      '<span class="act-col-head" title="Estimated tokens">Tok</span>' +
      '</div>';

    if (actions.length === 0 && toolNames.length > 0) {
      for (var tn = 0; tn < toolNames.length; tn++) {
        ahtml += '<div class="conv-action-item">' +
          '<span class="act-left"><span class="conv-tool-chip ' + getToolChipClass(toolNames[tn]) + '">' + escapeHtml(toolNames[tn]) + '</span></span>' +
          '<span class="act-cell"></span><span class="act-cell"></span></div>';
      }
    }

    for (var a = 0; a < actions.length; a++) {
      var act = actions[a];
      var targetStr = act.target || '';
      if (act.range) targetStr += act.range;
      // L9.2: Build detail subtitle from pattern/command/file_path
      var detailStr = '';
      if (act.pattern) detailStr = act.pattern;
      else if (act.command) detailStr = act.command;
      else if (act.file_path && !targetStr) detailStr = act.file_path;
      // Savings cell: compact 4-char max (e.g. ↓88%)
      var saveVal = '';
      if (act.savings > 0) {
        saveVal = '<span class="text-green">\u2193' + act.savings + '%</span>';
      }
      // L9.8: Shadow savings cell
      if (act.shadow_saved > 0 && (act.shadow_chars + act.shadow_saved) > 0) {
        var pct = Math.round((1 - act.shadow_chars / (act.shadow_chars + act.shadow_saved)) * 100);
        if (isFinite(pct)) {
          saveVal += (saveVal ? ' ' : '') + '<span class="text-cyan" title="Shadow: ' + fmtK(act.shadow_chars + act.shadow_saved) + ' \u2192 ' + fmtK(act.shadow_chars) + '">\u2193' + pct + '%</span>';
        }
      }
      // Tokens cell: compact 4-char max (e.g. 1.2k, 11k, 340)
      // Fallback: estimate tokens from result_chars for tools that don't set tokens at invocation (e.g. Task)
      var tokVal = '';
      var displayTokens = act.tokens > 0 ? act.tokens : (act.result_chars > 0 ? Math.round(act.result_chars / 4) : 0);
      if (displayTokens > 0) {
        var tokCls = act.attrib === 'aOa guided' ? 'text-green' : (act.attrib === 'unguided' ? 'text-red' : 'text-dim');
        tokVal = '<span class="' + tokCls + '">' + fmtK(displayTokens) + '</span>';
      }
      var pathStyle = act.attrib === 'unguided' ? ' style="color:var(--red)"' : '';
      // L9.2: Tooltip includes detail (pattern/command) when available
      var fullTooltip = targetStr;
      if (detailStr && detailStr !== targetStr) fullTooltip += '\n' + detailStr;
      ahtml += '<div class="conv-action-item">' +
        '<span class="act-left">' +
          '<span class="conv-tool-chip ' + getToolChipClass(act.tool) + '">' + escapeHtml(act.tool) + '</span>' +
          '<span class="conv-action-path"' + pathStyle + ' title="' + escapeHtml(fullTooltip) + '">' + escapeHtml(truncPath(targetStr, 80)) + '</span>' +
        '</span>' +
        '<span class="act-cell">' + saveVal + '</span>' +
        '<span class="act-cell">' + tokVal + '</span>' +
        '</div>';
    }
    ahtml += '</div>';
  }
  ahtml += '<div class="conv-now"><span class="conv-now-line"></span><span class="conv-now-dot"></span><span class="conv-now-text">NOW</span><span class="conv-now-line"></span></div>';
  document.getElementById('convActions').innerHTML = ahtml;

  // Auto-scroll actions if user was near bottom
  if (actWasNear && actContainer) {
    actContainer.scrollTop = actContainer.scrollHeight;
  }
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
  var cm = cache.convMetrics || {};

  var sessions = ss.sessions || [];
  var sessionCount = ss.count || sessions.length;

  // Aggregate stats across all sessions
  var totalSaved = rw.tokens_saved || 0;
  var totalTimeSavedMs = rw.time_saved_ms || 0;
  var totalReads = 0, totalGuidedReads = 0, totalPrompts = 0;
  var totalInputTokens = 0, totalCacheRead = 0;

  for (var i = 0; i < sessions.length; i++) {
    var s = sessions[i];
    totalReads += (s.read_count || 0);
    totalGuidedReads += (s.guided_read_count || 0);
    totalPrompts += (s.prompt_count || 0);
    totalSaved += (s.tokens_saved || 0);
    totalTimeSavedMs += (s.time_saved_ms || 0);
    totalInputTokens += (s.input_tokens || 0);
    totalCacheRead += (s.cache_read_tokens || 0);
  }
  // Include current session from live metrics
  totalInputTokens += (cm.input_tokens || 0);
  totalCacheRead += (cm.cache_read_tokens || 0);
  totalReads += (rw.read_count || 0);
  totalGuidedReads += (rw.guided_read_count || 0);

  var overallRatio = totalReads > 0 ? totalGuidedReads / totalReads : 0;
  var totalUnguided = Math.max(0, totalReads - totalGuidedReads);
  var readVelocity = totalPrompts > 0 ? (totalReads / totalPrompts).toFixed(1) : '-';
  var avgPrompts = sessionCount > 0 ? Math.round(totalPrompts / sessionCount) : '-';

  // Cost calculations
  var pricing = getModelPricing(rw.model);
  var costAvoidance = totalSaved * pricing.input / 1000000;
  var lifetimeCacheSavings = totalCacheRead * (pricing.input - pricing.cacheRead) / 1000000;

  // Efficiency score: weighted composite
  var cacheHitRate = rw.cache_hit_rate || 0;
  var savingsRate = totalReads > 0 ? totalSaved / (totalReads * 1000) : 0; // normalize
  if (savingsRate > 1) savingsRate = 1;
  var efficiencyScore = ((overallRatio * 0.4 + cacheHitRate * 0.3 + savingsRate * 0.3) * 100).toFixed(0);

  // ROI
  var roi = '-';
  var totalSpent = calcSessionCost(totalInputTokens, 0, totalCacheRead, rw.model);
  if (totalSpent > 0) {
    roi = ((costAvoidance + lifetimeCacheSavings) / totalSpent).toFixed(1) + 'x';
  }

  // Hero metrics: cost avoidance → extended → cache savings → efficiency
  setGlow('hm-arsenal-0', costAvoidance > 0 ? fmtDollar(costAvoidance) : '-');
  setGlow('hm-arsenal-1', rw.delta_minutes ? fmtMin(rw.delta_minutes) : '-');
  setGlow('hm-arsenal-2', lifetimeCacheSavings > 0 ? fmtDollar(lifetimeCacheSavings) : '-');
  setGlow('hm-arsenal-3', efficiencyScore + '%');

  // Hero support
  var sup = [];
  sup.push('<span class="c">' + sessionCount + '</span> sessions');
  sup.push('<span class="g">' + fmtK(totalSaved) + '</span> tokens saved');
  if (totalTimeSavedMs > 0) sup.push('saved <span class="g">' + fmtTime(totalTimeSavedMs) + '</span>');
  sup.push('ROI <span class="g">' + roi + '</span>');
  setHtml('heroSupport-arsenal', sup.join(' &middot; '));

  // Stats
  setGlow('as-ratio', fmtPct(overallRatio));
  setGlow('as-cost', fmtK(totalUnguided * 200));
  setGlow('as-sessions', sessionCount);
  setGlow('as-avgprompts', avgPrompts);
  setGlow('as-saved', fmtK(totalSaved));
  setGlow('as-velocity', readVelocity);
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
      '<td class="mono text-green" style="font-size:11px">' + (sess.time_saved_ms > 0 ? fmtTime(sess.time_saved_ms) : '-') + '</td>' +
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

/* ── Now button: scroll listeners + global handler ── */
window.scrollToNow = function(id) {
  var el = document.getElementById(id);
  if (el) el.scrollTop = el.scrollHeight;
  var btnId = (id === 'convMessages') ? 'nowBtnMsg' : 'nowBtnAct';
  var btn = document.getElementById(btnId);
  if (btn) btn.style.display = 'none';
};

(function initScrollListeners() {
  var msgEl = document.getElementById('convMessages');
  var actEl = document.getElementById('convActions');
  if (msgEl) {
    msgEl.addEventListener('scroll', function() {
      var btn = document.getElementById('nowBtnMsg');
      if (btn) btn.style.display = isNearBottom(msgEl) ? 'none' : 'block';
    });
  }
  if (actEl) {
    actEl.addEventListener('scroll', function() {
      var btn = document.getElementById('nowBtnAct');
      if (btn) btn.style.display = isNearBottom(actEl) ? 'none' : 'block';
    });
  }
})();

/* ══════════════════════════════════════════════════════════
   RECON TAB: Dimensional Scanning
   ══════════════════════════════════════════════════════════ */
var RECON_TIERS = [
  {
    id: 'security', label: 'Security', color: 'red', desc: 'Injection, secrets, crypto, transport, auth',
    dimensions: [
      { id: 'injection', label: 'Injection' },
      { id: 'secrets', label: 'Secrets' },
      { id: 'crypto', label: 'Cryptography' },
      { id: 'transport', label: 'Transport' },
      { id: 'exposure', label: 'Exposure' },
      { id: 'config', label: 'Config' },
      { id: 'data', label: 'Data' },
      { id: 'denial', label: 'Denial' },
      { id: 'auth', label: 'Auth' }
    ]
  },
  {
    id: 'performance', label: 'Performance', color: 'yellow', desc: 'Resources, concurrency, query, memory, hot path',
    dimensions: [
      { id: 'resources', label: 'Resource Leaks' },
      { id: 'concurrency', label: 'Concurrency' },
      { id: 'query', label: 'Query Patterns' },
      { id: 'memory', label: 'Memory' },
      { id: 'hot_path', label: 'Hot Path' }
    ]
  },
  {
    id: 'quality', label: 'Quality', color: 'blue', desc: 'Errors, complexity, dead code, conventions',
    dimensions: [
      { id: 'errors', label: 'Error Handling' },
      { id: 'complexity', label: 'Complexity' },
      { id: 'dead_code', label: 'Dead Code' },
      { id: 'conventions', label: 'Conventions' }
    ]
  },
  {
    id: 'architecture', label: 'Architecture', color: 'cyan', desc: 'Anti-patterns, imports, API surface, coupling',
    dimensions: [
      { id: 'antipattern', label: 'Anti-patterns' },
      { id: 'imports', label: 'Import Health' },
      { id: 'api_surface', label: 'API Surface' },
      { id: 'coupling', label: 'Coupling' }
    ]
  },
  {
    id: 'observability', label: 'Observability', color: 'green', desc: 'Debug, silent failures, logging, resilience, error visibility',
    dimensions: [
      { id: 'debug', label: 'Debug Artifacts' },
      { id: 'silent_failures', label: 'Silent Failures' },
      { id: 'logging', label: 'Logging' },
      { id: 'resilience', label: 'Resilience' },
      { id: 'error_visibility', label: 'Error Visibility' }
    ]
  },
  {
    id: 'investigated', label: 'Investigated', color: 'slate', desc: 'Reviewed & accepted files',
    dimensions: [
      { id: 'investigated', label: 'Reviewed' }
    ],
    defaultOff: true
  }
];

var RECON_TIER_ABBREV = {
  security: 'Sec', performance: 'Perf', quality: 'Qual',
  architecture: 'Arch', observability: 'Obs', investigated: 'Inv'
};
var RECON_DIM_ORDER = ['security', 'performance', 'quality', 'architecture', 'observability', 'investigated'];

// Active dimension state — persisted in localStorage
var reconActiveDims = {};
var savedReconDims = JSON.parse(localStorage.getItem('aoa-recon-dims') || '{}');
RECON_TIERS.forEach(function(t) {
  t.dimensions.forEach(function(d) {
    var defaultVal = t.defaultOff ? false : true;
    reconActiveDims[d.id] = savedReconDims.hasOwnProperty(d.id) ? savedReconDims[d.id] : defaultVal;
  });
});

function saveReconDimState() {
  localStorage.setItem('aoa-recon-dims', JSON.stringify(reconActiveDims));
}

function isReconTierActive(tierId) {
  var tier = RECON_TIERS.find(function(t) { return t.id === tierId; });
  return tier ? tier.dimensions.some(function(d) { return reconActiveDims[d.id]; }) : false;
}

function toggleReconTier(tierId) {
  var tier = RECON_TIERS.find(function(t) { return t.id === tierId; });
  if (!tier) return;
  var anyActive = tier.dimensions.some(function(d) { return reconActiveDims[d.id]; });
  tier.dimensions.forEach(function(d) { reconActiveDims[d.id] = !anyActive; });
  saveReconDimState();
  renderRecon();
}

function toggleReconDim(dimId) {
  reconActiveDims[dimId] = !reconActiveDims[dimId];
  saveReconDimState();
  renderRecon();
}

function soloReconTier(tierId) {
  // Deactivate all dimensions, then activate only this tier's dimensions
  RECON_TIERS.forEach(function(t) {
    t.dimensions.forEach(function(d) {
      reconActiveDims[d.id] = (t.id === tierId);
    });
  });
  saveReconDimState();
}

// Navigation state
var reconLevel = 'root', reconFolder = null, reconFile = null;

// File-level controls
var reconFocus = 'recon';  // 'recon' | 'critical' | 'warning' | 'info' | 'all'
var reconSourceOn = false;  // false = findings detail | true = inline source code

// Source line cache: "file:line" → source text. Avoids re-fetching on every poll cycle.
var reconSourceCache = {};

// Annotate findings with investigated flag based on investigated_files from API.
// Also adds 'investigated' dim_id so the dim filter system handles show/hide.
function reconAnnotateInvestigated(data) {
  if (!data || !data.tree) return;
  var invSet = {};
  (data.investigated_files || []).forEach(function(f) { invSet[f] = true; });

  Object.keys(data.tree).forEach(function(folder) {
    var files = data.tree[folder];
    Object.keys(files).forEach(function(file) {
      var relPath = (folder === '.' ? '' : folder + '/') + file;
      var isInv = !!invSet[relPath];
      var info = files[file];
      if (info.findings) {
        info.findings.forEach(function(f) {
          f.investigated = isInv;
        });
      }
    });
  });
}

// Mark/unmark a file as investigated via API.
function reconSetInvestigated(relPath, investigated) {
  fetch('/api/recon-investigate', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ file: relPath, action: investigated ? 'add' : 'remove' })
  }).then(function() {
    // Re-fetch recon data to update annotations
    safeFetch('/api/recon').then(function(d) {
      cache.recon = d;
      reconAnnotateInvestigated(d);
      renderRecon();
    });
  });
}

function reconClearAllInvestigated() {
  fetch('/api/recon-investigate', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ action: 'clear' })
  }).then(function() {
    safeFetch('/api/recon').then(function(d) {
      cache.recon = d;
      reconAnnotateInvestigated(d);
      renderRecon();
    });
  });
}

function reconNavigateTo(level, folder, file) {
  // Reset controls when leaving file level
  if (level !== 'file') {
    reconFocus = 'recon';
    reconSourceOn = false;
  }
  // Clear source cache when navigating to a different file
  if (file !== reconFile || folder !== reconFolder) {
    reconSourceCache = {};
  }
  reconLevel = level;
  reconFolder = folder || null;
  reconFile = file || null;
  renderReconTree(cache.recon);
}

// Filter findings based on active dimensions
function reconFilterFindings(findings) {
  if (!findings) return [];
  var invActive = reconActiveDims['investigated'];
  return findings.filter(function(f) {
    // Investigated findings: only show when investigated toggle is ON
    if (f.investigated && !invActive) return false;
    // Non-investigated findings: hide when ONLY investigated toggle is on (solo mode)
    if (!f.investigated && invActive && !reconActiveDims[f.dim_id]) return false;
    // Normal dim filter for non-investigated findings
    if (!f.investigated && !reconActiveDims[f.dim_id]) return false;
    // Focus filter
    if (reconFocus === 'recon' && f.severity === 'info') return false;
    if (reconFocus === 'critical' && f.severity !== 'critical') return false;
    if (reconFocus === 'warning' && f.severity !== 'warning') return false;
    if (reconFocus === 'info' && f.severity !== 'info') return false;
    return true;
  });
}

// Aggregate findings for a tree node (folder or file)
function reconAggregate(data, prefix) {
  var byCat = {};
  var sevs = { critical: 0, warning: 0, info: 0 };
  var total = 0;
  if (!data || !data.tree) return { byCat: byCat, sevs: sevs, total: 0 };

  Object.keys(data.tree).forEach(function(folder) {
    if (prefix && folder !== prefix && folder.indexOf(prefix + '/') !== 0) return;
    var files = data.tree[folder];
    Object.keys(files).forEach(function(file) {
      if (prefix && prefix.indexOf('/') === -1) {
        // folder-level match: only check folder
      } else if (prefix) {
        // file-level: folder/file
        var checkPath = folder + '/' + file;
        if (checkPath !== prefix) return;
      }
      var filtered = reconFilterFindings(files[file].findings);
      filtered.forEach(function(f) {
        byCat[f.tier_id] = (byCat[f.tier_id] || 0) + 1;
        sevs[f.severity]++;
        total++;
      });
    });
  });

  return { byCat: byCat, sevs: sevs, total: total };
}

function reconMaxSeverity(s) {
  return s.critical > 0 ? 'critical' : s.warning > 0 ? 'warning' : s.info > 0 ? 'info' : null;
}
function reconScorePct(s) { return Math.min(100, s.critical * 25 + s.warning * 10 + s.info * 0); }
function reconScoreClass(p) { return p >= 40 ? 'high' : p >= 15 ? 'medium' : 'low'; }

function reconDimsHtml(byCat) {
  var h = '';
  RECON_DIM_ORDER.forEach(function(tid) {
    var n = byCat[tid] || 0;
    h += '<span class="dim-num ' + (n > 0 ? tid : 'zero') + '">' + n + '</span>';
  });
  return h;
}

function reconDimsHeaderHtml() {
  var h = '<span class="dims-label">Dimensions</span>' +
    '<div class="tree-dims">';
  RECON_DIM_ORDER.forEach(function(tid) {
    var active = isReconTierActive(tid);
    h += '<span class="col-dim ' + tid + (active ? '' : ' off') + '" data-tier-toggle="' + tid + '">' + RECON_TIER_ABBREV[tid] + '</span>';
  });
  h += '</div>';
  return h;
}

function reconColHeaderHtml(fileLevel) {
  var h = '<div class="tree-col-header">';
  if (fileLevel) {
    // File-level toolbar: Focus | Code | Copy Prompt | ... | Dimensions
    h += '<div class="toolbar-box">' +
      '<span class="toolbar-label">Focus</span>' +
      '<span class="toolbar-btn' + (reconFocus === 'recon' ? ' active' : '') + '" data-focus="recon">Recon</span>' +
      '<span class="toolbar-btn' + (reconFocus === 'critical' ? ' active' : '') + '" data-focus="critical">Critical</span>' +
      '<span class="toolbar-btn' + (reconFocus === 'warning' ? ' active' : '') + '" data-focus="warning">Warning</span>' +
      '<span class="toolbar-btn' + (reconFocus === 'info' ? ' active' : '') + '" data-focus="info">Info</span>' +
      '<span class="toolbar-btn' + (reconFocus === 'all' ? ' active' : '') + '" data-focus="all">All</span>' +
      '</div>';
    h += '<span class="toolbar-btn toolbar-toggle' + (reconSourceOn ? ' active' : '') + '" id="reconSourceToggle">Code</span>';
    h += '<span class="toolbar-btn toolbar-action" id="reconCopyPrompt">Copy Prompt</span>';
    // Investigate action — context-dependent label
    var filePath = (reconFolder === '.' ? '' : reconFolder + '/') + reconFile;
    var isInv = cache.recon && cache.recon.investigated_files && cache.recon.investigated_files.indexOf(filePath) >= 0;
    h += '<span class="toolbar-btn toolbar-action' + (isInv ? ' investigated' : '') + '" id="reconInvBtn">' + (isInv ? 'Uninvestigate' : 'Investigated') + '</span>';
    h += reconDimsHeaderHtml();
    // Trailing spacers to match method-row trailing elements (severity dot + score bar + chevron)
    h += '<span style="width:7px;flex-shrink:0"></span>' +
      '<span style="width:44px;flex-shrink:0"></span>' +
      '<span style="width:14px;flex-shrink:0"></span>';
  } else {
    // Root/folder level: spacer + Dimensions (right-aligned)
    h += '<span class="col-name"></span>';
    h += reconDimsHeaderHtml();
    // Trailing spacers to align with severity dot + score bar + chevron
    h += '<span style="width:7px;flex-shrink:0"></span>' +
      '<span style="width:44px;flex-shrink:0"></span>' +
      '<span style="width:14px;flex-shrink:0"></span>';
  }
  h += '</div>';
  return h;
}

function reconDimsClickableHtml(byCat, navAction) {
  var h = '';
  RECON_DIM_ORDER.forEach(function(tid) {
    var n = byCat[tid] || 0;
    if (n > 0) {
      h += '<span class="dim-num ' + tid + ' dim-clickable" data-dim-nav="' + escapeHtml(navAction) + '" data-dim-tier="' + tid + '">' + n + '</span>';
    } else {
      h += '<span class="dim-num zero">' + n + '</span>';
    }
  });
  return h;
}

function reconTreeRowHtml(type, name, agg, navAction) {
  var sev = reconMaxSeverity(agg.sevs);
  var pct = reconScorePct(agg.sevs);
  var icon = type === 'folder' ? '\uD83D\uDCC1' : '\uD83D\uDCC4';
  return '<div class="tree-row" data-nav="' + escapeHtml(navAction) + '">' +
    '<span class="tree-icon ' + type + '">' + icon + '</span>' +
    '<span class="tree-name">' + escapeHtml(name) + '</span>' +
    '<div class="tree-dims">' + reconDimsClickableHtml(agg.byCat, navAction) + '</div>' +
    (sev ? '<span class="tree-severity ' + sev + '"></span>' : '<span style="width:7px"></span>') +
    '<div class="tree-score"><div class="tree-score-fill ' + reconScoreClass(pct) + '" style="width:' + pct + '%"></div></div>' +
    '<span class="tree-chevron">\u203A</span></div>';
}

function reconMethodRowHtml(name, findings) {
  var filtered = reconFilterFindings(findings);
  var h = '';

  if (filtered.length === 0) {
    return '';
  }

  var byCat = {}; var sevs = { critical: 0, warning: 0, info: 0 };
  filtered.forEach(function(f) { byCat[f.tier_id] = (byCat[f.tier_id] || 0) + 1; sevs[f.severity]++; });
  var sev = reconMaxSeverity(sevs);
  var pct = reconScorePct(sevs);

  // Symbol header — always the same structure regardless of code toggle
  h += '<div class="tree-row" style="cursor:default"><span class="tree-icon method">\u0192</span>' +
    '<span class="tree-name">' + escapeHtml(name) + '</span>' +
    '<div class="tree-dims">' + reconDimsHtml(byCat) + '</div>' +
    '<span class="tree-severity ' + sev + '"></span>' +
    '<div class="tree-score"><div class="tree-score-fill ' + reconScoreClass(pct) + '" style="width:' + pct + '%"></div></div>' +
    '<span style="width:14px"></span></div>';

  filtered.sort(function(a, b) {
    var o = { critical: 0, warning: 1, info: 2 };
    if (o[a.severity] !== o[b.severity]) return o[a.severity] - o[b.severity];
    return a.line - b.line;
  });

  var filePath = (reconFolder === '.' ? '' : reconFolder + '/') + reconFile;

  filtered.forEach(function(f) {
    var tierAbbrev = RECON_TIER_ABBREV[f.tier_id] || f.tier_id;

    if (reconSourceOn) {
      // Code mode: tier + line + severity + source code (replaces rule_id + description)
      var cacheKey = filePath + ':' + f.line;
      var cachedSrc = reconSourceCache[cacheKey];
      var srcText = cachedSrc || '';
      var needsFetch = !cachedSrc;
      h += '<div class="finding-row finding-code-mode"' +
        (needsFetch ? ' data-code-file="' + escapeHtml(filePath) + '" data-code-line="' + f.line + '"' : '') + '>' +
        '<span class="finding-tier ' + f.tier_id + '">' + tierAbbrev + '</span>' +
        '<span class="finding-line">L' + f.line + '</span>' +
        '<span class="finding-sev ' + f.severity + '">' + f.severity + '</span>' +
        '<span class="finding-src">' + escapeHtml(srcText) + '</span></div>';
    } else {
      // Detail mode: tier + line + severity + rule_id + description
      h += '<div class="finding-row finding-peek" data-file="' + escapeHtml(filePath) + '" data-line="' + f.line + '">' +
        '<span class="finding-tier ' + f.tier_id + '">' + tierAbbrev + '</span>' +
        '<span class="finding-line">L' + f.line + '</span>' +
        '<span class="finding-sev ' + f.severity + '">' + f.severity + '</span>' +
        '<span class="finding-id">' + escapeHtml(f.id) + '</span>' +
        '<span class="finding-desc">' + escapeHtml(f.label) + '</span>' +
        '<span class="finding-code" style="display:none"></span></div>';
    }
  });

  return h;
}

function reconCopyInvestigatePrompt() {
  var data = cache.recon;
  if (!data || !data.tree || !reconFolder || !reconFile) return;

  var fileData = data.tree[reconFolder] && data.tree[reconFolder][reconFile];
  if (!fileData) return;

  var filePath = (reconFolder === '.' ? '' : reconFolder + '/') + reconFile;
  var findings = reconFilterFindings(fileData.findings);

  if (findings.length === 0) return;

  // Collect unique tiers and their severities for the taxonomy header
  var tierSevs = {};
  findings.forEach(function(f) {
    if (!tierSevs[f.tier_id]) tierSevs[f.tier_id] = {};
    tierSevs[f.tier_id][f.severity] = true;
  });

  // Collect affected symbols for caller investigation
  var symbolSet = {};
  findings.forEach(function(f) {
    if (f.symbol && f.symbol !== '(package-level)') symbolSet[f.symbol] = true;
  });

  var lines = [];
  lines.push('Investigate `' + filePath + '`.');
  lines.push('');

  // Taxonomy: define each tier+severity combo once
  var taxParts = [];
  Object.keys(tierSevs).forEach(function(tid) {
    var abbrev = (RECON_TIER_ABBREV[tid] || tid).toUpperCase();
    var sevList = Object.keys(tierSevs[tid]).sort();
    taxParts.push(abbrev + ' = ' + tid + ' (' + sevList.join(', ') + ')');
  });
  lines.push('Dimensions: ' + taxParts.join('; '));
  lines.push('');

  // Findings: file:line [TIER severity] actual_code_or_label
  findings.sort(function(a, b) { return a.line - b.line; });
  findings.forEach(function(f) {
    var abbrev = (RECON_TIER_ABBREV[f.tier_id] || f.tier_id).toUpperCase();
    var cacheKey = filePath + ':' + f.line;
    var code = reconSourceCache[cacheKey];
    var content = code ? code.trim() : f.label;
    lines.push(filePath + ':' + f.line + ' [' + abbrev + ' ' + f.severity + '] ' + content);
  });
  lines.push('');
  lines.push('For each line: real issue \u2192 provide the fix, or acceptable in context \u2192 explain why.');

  var namedSymbols = Object.keys(symbolSet);
  if (namedSymbols.length > 0) {
    lines.push('Also check callers of ' + namedSymbols.map(function(s) { return '`' + s + '`'; }).join(', ') + ' for propagation.');
  }

  var prompt = lines.join('\n');

  navigator.clipboard.writeText(prompt).then(function() {
    var btn = document.getElementById('reconCopyPrompt');
    if (btn) {
      btn.textContent = 'Paste in Claude Code \u2197';
      btn.classList.add('copied');
      setTimeout(function() {
        btn.textContent = 'Copy Prompt';
        btn.classList.remove('copied');
      }, 3000);
    }
  });
}

function renderReconInstallPrompt() {
  var wrap = document.getElementById('reconTreeWrap');
  if (!wrap) return;
  wrap.innerHTML =
    '<div style="text-align:center;padding:40px 20px;color:#aaa;font-size:14px;line-height:1.8">' +
    '<div style="font-size:24px;margin-bottom:12px;opacity:0.6">&#x1F50D;</div>' +
    '<div style="color:#e0e0e0;font-size:16px;margin-bottom:8px">aoa-recon not installed</div>' +
    '<div>Install the companion binary to unlock full scanning:</div>' +
    '<div style="margin:16px auto;max-width:400px;background:#1a1a2e;border:1px solid #333;border-radius:6px;padding:12px 16px;font-family:monospace;font-size:13px;color:#4fc3f7;text-align:left">' +
    'npm install -g aoa-recon</div>' +
    '<div style="color:#888;font-size:12px;margin-top:8px">Adds tree-sitter parsing + security scanning. Restart the daemon after installing.</div>' +
    '</div>';
}

function renderRecon() {
  var data = cache.recon;
  if (!data) return;

  // Show install prompt when aoa-recon is not available and no scan data
  if (!data.recon_available && (data.total_findings || 0) === 0 && (data.files_scanned || 0) === 0) {
    renderReconInstallPrompt();
    return;
  }

  var filesScanned = data.files_scanned || 0;
  var totalFindings = data.total_findings || 0;
  var cleanFiles = data.clean_files || 0;
  var cleanPct = filesScanned > 0 ? (cleanFiles / filesScanned * 100).toFixed(0) + '%' : '-';
  var findingsPerFile = filesScanned > 0 ? (totalFindings / filesScanned).toFixed(1) : '-';

  // Hero metrics
  setGlow('hm-recon-0', filesScanned);
  setGlow('hm-recon-1', totalFindings);
  setGlow('hm-recon-2', data.critical || 0);
  setGlow('hm-recon-3', cleanPct);

  // Hero support
  var activeDimCount = Object.values(reconActiveDims).filter(function(v) { return v; }).length;
  var sup = [];
  sup.push('<span class="g">' + filesScanned + '</span> files');
  sup.push('<span class="r">' + findingsPerFile + '</span> findings/file');
  sup.push('<span class="g">' + cleanPct + '</span> clean');
  sup.push('<span class="p">' + activeDimCount + '</span> dims active');
  if (!data.recon_available) {
    sup.push('<span class="p" title="Install aoa-recon for symbol-level scanning">lite mode</span>');
  }
  if (data.scanned_at) {
    var ago = Math.max(0, Math.floor(Date.now() / 1000 - data.scanned_at));
    var agoStr = ago < 5 ? 'just now' : ago < 60 ? ago + 's ago' : Math.floor(ago / 60) + 'm ago';
    sup.push('<span class="g" title="Cache snapshot age">scanned ' + agoStr + '</span>');
  }
  setHtml('heroSupport-recon', sup.join(' &middot; '));

  // Stats grid
  setGlow('rs-density', findingsPerFile);
  setGlow('rs-critical', data.critical || 0);
  setGlow('rs-warnings', data.warnings || 0);
  setGlow('rs-cleanpct', cleanPct);
  setGlow('rs-dims', activeDimCount);
  setText('rs-active-count', activeDimCount);

  // Sidebar
  renderReconSidebar(data);

  // Tree
  renderReconTree(data);
}

function renderReconSidebar(data) {
  var sb = document.getElementById('reconSidebar');
  if (!sb) return;
  var footer = sb.querySelector('.recon-sidebar-footer');

  // Remove old tier elements
  sb.querySelectorAll('.recon-tier').forEach(function(el) { el.remove(); });
  sb.querySelectorAll('.recon-tier-pills').forEach(function(el) { el.remove(); });

  var totalActive = 0;
  var totalDims = 0;

  RECON_TIERS.forEach(function(tier) {
    var tierEl = document.createElement('div');
    tierEl.className = 'recon-tier ' + tier.id;

    var tierTotal = 0;
    tier.dimensions.forEach(function(d) {
      tierTotal += (data.dim_counts && data.dim_counts[d.id]) || 0;
    });

    var tierActive = isReconTierActive(tier.id);
    var hdr = document.createElement('div');
    hdr.className = 'recon-tier-header' + (tierActive ? '' : ' tier-off');
    hdr.innerHTML =
      '<span class="recon-tier-toggle ' + (tierActive ? 'on' : 'off') + '"></span>' +
      '<span class="recon-tier-label-group">' +
        '<div class="recon-tier-label">' + escapeHtml(tier.label) + '</div>' +
        '<div class="recon-tier-desc">' + escapeHtml(tier.desc || '') + '</div>' +
      '</span>' +
      '<span class="recon-tier-count">' + tierTotal + '</span>';
    hdr.setAttribute('data-tier-id', tier.id);
    hdr.addEventListener('click', function() { toggleReconTier(tier.id); });
    tierEl.appendChild(hdr);

    var pillsWrap = document.createElement('div');
    pillsWrap.className = 'recon-tier-pills';

    tier.dimensions.forEach(function(dim) {
      totalDims++;
      var isActive = reconActiveDims[dim.id];
      if (isActive) totalActive++;
      var count = (data.dim_counts && data.dim_counts[dim.id]) || 0;

      var pill = document.createElement('span');
      pill.className = 'recon-dim-pill ' + (isActive ? 'active' : 'inactive');
      pill.innerHTML = escapeHtml(dim.label) + (count > 0 ? ' <span class="pill-count">' + count + '</span>' : '');
      pill.addEventListener('click', function(e) {
        e.stopPropagation();
        toggleReconDim(dim.id);
      });
      pillsWrap.appendChild(pill);
    });

    tierEl.appendChild(pillsWrap);
    sb.insertBefore(tierEl, footer);
  });

  setText('rs-sidebar-active', totalActive);
  setText('rs-sidebar-total', totalDims);
}

function renderReconTree(data) {
  if (!data || !data.tree) return;
  var wrap = document.getElementById('reconTreeWrap');
  if (!wrap) return;
  var h = '';

  // Update breadcrumb
  var bcEl = document.getElementById('reconBreadcrumb');
  if (bcEl) {
    var bc = '<span class="bc-link" data-nav="root">Root</span>';
    if (reconFolder) {
      bc += '<span class="bc-sep">\u203A</span>';
      bc += '<span class="bc-link" data-nav="folder:' + escapeHtml(reconFolder) + '">' + escapeHtml(reconFolder) + '</span>';
    }
    if (reconFile) {
      bc += '<span class="bc-sep">\u203A</span>';
      bc += '<span class="bc-link" data-nav="file:' + escapeHtml(reconFolder) + ':' + escapeHtml(reconFile) + '">' + escapeHtml(reconFile) + '</span>';
    }
    bcEl.innerHTML = bc;
  }

  if (reconLevel === 'root') {
    // Show folders sorted by finding count
    var folders = Object.keys(data.tree);
    var items = folders.map(function(f) {
      var agg = reconAggregateFolder(data, f);
      return { name: f, agg: agg };
    });
    items.sort(function(a, b) { return b.agg.total - a.agg.total; });
    document.getElementById('reconTreeBadge').textContent = items.length + ' directories';
    h += reconColHeaderHtml();
    items.forEach(function(item) {
      h += reconTreeRowHtml('folder', item.name + '/', item.agg, 'folder:' + item.name);
    });
  } else if (reconLevel === 'folder') {
    var files = data.tree[reconFolder] || {};
    var fileItems = Object.keys(files).map(function(f) {
      var agg = reconAggregateFile(data, reconFolder, f);
      return { name: f, agg: agg };
    });
    fileItems.sort(function(a, b) { return b.agg.total - a.agg.total; });
    document.getElementById('reconTreeBadge').textContent = fileItems.length + ' files';
    h += reconColHeaderHtml();
    fileItems.forEach(function(item) {
      h += reconTreeRowHtml('file', item.name, item.agg, 'file:' + reconFolder + ':' + item.name);
    });
  } else if (reconLevel === 'file') {
    var fileData = data.tree[reconFolder] && data.tree[reconFolder][reconFile];
    if (fileData) {
      var symbols = fileData.symbols || [];
      document.getElementById('reconTreeBadge').textContent = symbols.length + ' symbols';
      h += reconColHeaderHtml(true);

      // Filter findings by active dimensions
      var elevatedFindings = reconFilterFindings(fileData.findings);

      // Group findings by symbol
      var findingsBySymbol = {};
      elevatedFindings.forEach(function(f) {
        var sym = f.symbol || '(package-level)';
        if (!findingsBySymbol[sym]) findingsBySymbol[sym] = [];
        findingsBySymbol[sym].push(f);
      });

      symbols.forEach(function(sym) {
        h += reconMethodRowHtml(sym, findingsBySymbol[sym] || []);
      });

      // Package-level findings (no symbol)
      if (findingsBySymbol['(package-level)'] || findingsBySymbol['']) {
        var pkgFindings = (findingsBySymbol['(package-level)'] || []).concat(findingsBySymbol[''] || []);
        if (pkgFindings.length > 0) {
          h += reconMethodRowHtml('(package-level)', pkgFindings);
        }
      }
    }
  }

  if (!h) h = '<div style="padding:40px;text-align:center;color:var(--mute)">No findings match active dimensions.</div>';
  wrap.innerHTML = h;

  // Attach click handlers for tree navigation
  wrap.querySelectorAll('[data-nav]').forEach(function(el) {
    el.addEventListener('click', function() {
      var nav = this.getAttribute('data-nav');
      var parts = nav.split(':');
      if (parts[0] === 'root') reconNavigateTo('root');
      else if (parts[0] === 'folder') reconNavigateTo('folder', parts[1]);
      else if (parts[0] === 'file') reconNavigateTo('file', parts[1], parts[2]);
    });
  });

  // Attach click handlers for column header tier toggles
  wrap.querySelectorAll('[data-tier-toggle]').forEach(function(el) {
    el.addEventListener('click', function(e) {
      e.stopPropagation();
      toggleReconTier(this.getAttribute('data-tier-toggle'));
    });
  });

  // Attach click handlers for dimension count numbers (solo tier + navigate)
  wrap.querySelectorAll('[data-dim-nav]').forEach(function(el) {
    el.addEventListener('click', function(e) {
      e.stopPropagation();
      var tierId = this.getAttribute('data-dim-tier');
      var nav = this.getAttribute('data-dim-nav');
      var parts = nav.split(':');
      soloReconTier(tierId);
      if (parts[0] === 'root') reconNavigateTo('root');
      else if (parts[0] === 'folder') reconNavigateTo('folder', parts[1]);
      else if (parts[0] === 'file') reconNavigateTo('file', parts[1], parts[2]);
      renderReconSidebar(cache.recon);
    });
  });

  // Breadcrumb navigation
  if (bcEl) {
    bcEl.querySelectorAll('[data-nav]').forEach(function(el) {
      el.addEventListener('click', function() {
        var nav = this.getAttribute('data-nav');
        var parts = nav.split(':');
        if (parts[0] === 'root') reconNavigateTo('root');
        else if (parts[0] === 'folder') reconNavigateTo('folder', parts[1]);
        else if (parts[0] === 'file') reconNavigateTo('file', parts[1], parts[2]);
      });
    });
  }

  // Focus buttons (file level)
  wrap.querySelectorAll('[data-focus]').forEach(function(el) {
    el.addEventListener('click', function(e) {
      e.stopPropagation();
      reconFocus = this.getAttribute('data-focus');
      renderReconTree(cache.recon);
    });
  });

  // Source toggle (file level)
  var srcToggle = document.getElementById('reconSourceToggle');
  if (srcToggle) {
    srcToggle.addEventListener('click', function(e) {
      e.stopPropagation();
      reconSourceOn = !reconSourceOn;
      renderReconTree(cache.recon);
    });
  }

  // Copy Prompt button (file level)
  var cpBtn = document.getElementById('reconCopyPrompt');
  if (cpBtn) {
    cpBtn.addEventListener('click', function(e) {
      e.stopPropagation();
      reconCopyInvestigatePrompt();
    });
  }

  // Investigate button (file level)
  var invBtn = document.getElementById('reconInvBtn');
  if (invBtn) {
    invBtn.addEventListener('click', function(e) {
      e.stopPropagation();
      if (!reconFolder || !reconFile) return;
      var filePath = (reconFolder === '.' ? '' : reconFolder + '/') + reconFile;
      var isInv = cache.recon && cache.recon.investigated_files && cache.recon.investigated_files.indexOf(filePath) >= 0;
      reconSetInvestigated(filePath, !isInv);
    });
  }

  // Code mode: fetch source lines for finding rows not yet cached
  if (reconSourceOn) {
    wrap.querySelectorAll('.finding-code-mode[data-code-file]').forEach(function(el) {
      var file = el.getAttribute('data-code-file');
      var line = el.getAttribute('data-code-line');
      var srcEl = el.querySelector('.finding-src');
      if (!srcEl) return;
      var cacheKey = file + ':' + line;

      fetch('/api/source-line?file=' + encodeURIComponent(file) + '&line=' + line + '&context=0')
        .then(function(r) { return r.json(); })
        .then(function(lines) {
          var text = (lines && lines.length > 0) ? lines[0].content : '(line not in cache)';
          reconSourceCache[cacheKey] = text;
          srcEl.textContent = text;
        })
        .catch(function() {
          reconSourceCache[cacheKey] = '(unavailable)';
          srcEl.textContent = '(unavailable)';
        });
    });
  }

  // Finding peek: click toggles between description and source line
  wrap.querySelectorAll('.finding-peek').forEach(function(el) {
    el.addEventListener('click', function() {
      var row = this;
      var descEl = row.querySelector('.finding-desc');
      var codeEl = row.querySelector('.finding-code');
      if (!descEl || !codeEl) return;

      // Toggle: if code is visible, swap back to description
      if (codeEl.style.display !== 'none') {
        codeEl.style.display = 'none';
        descEl.style.display = '';
        row.classList.remove('finding-peek-active');
        return;
      }

      // If code already fetched, just show it
      if (codeEl.getAttribute('data-loaded')) {
        descEl.style.display = 'none';
        codeEl.style.display = '';
        row.classList.add('finding-peek-active');
        return;
      }

      // Fetch source line from cache
      var file = row.getAttribute('data-file');
      var line = row.getAttribute('data-line');
      codeEl.textContent = 'loading...';
      descEl.style.display = 'none';
      codeEl.style.display = '';

      fetch('/api/source-line?file=' + encodeURIComponent(file) + '&line=' + line + '&context=0')
        .then(function(r) { return r.json(); })
        .then(function(lines) {
          if (lines && lines.length > 0) {
            codeEl.textContent = lines[0].content;
          } else {
            codeEl.textContent = '(line not in cache)';
          }
          codeEl.setAttribute('data-loaded', '1');
          row.classList.add('finding-peek-active');
        })
        .catch(function() {
          codeEl.textContent = '(unavailable)';
          codeEl.setAttribute('data-loaded', '1');
          row.classList.add('finding-peek-active');
        });
    });
  });
}

function reconAggregateFolder(data, folder) {
  var byCat = {};
  var sevs = { critical: 0, warning: 0, info: 0 };
  var total = 0;
  var files = data.tree[folder] || {};
  Object.keys(files).forEach(function(file) {
    var filtered = reconFilterFindings(files[file].findings);
    filtered.forEach(function(f) {
      byCat[f.tier_id] = (byCat[f.tier_id] || 0) + 1;
      if (f.investigated) byCat['investigated'] = (byCat['investigated'] || 0) + 1;
      sevs[f.severity]++;
      total++;
    });
  });
  // Also count investigated findings that are currently hidden (for INV column display)
  if (!reconActiveDims['investigated']) {
    Object.keys(files).forEach(function(file) {
      var allFindings = files[file].findings || [];
      allFindings.forEach(function(f) {
        if (f.investigated) byCat['investigated'] = (byCat['investigated'] || 0) + 1;
      });
    });
  }
  return { byCat: byCat, sevs: sevs, total: total };
}

function reconAggregateFile(data, folder, file) {
  var byCat = {};
  var sevs = { critical: 0, warning: 0, info: 0 };
  var total = 0;
  var fileData = data.tree[folder] && data.tree[folder][file];
  if (fileData) {
    var filtered = reconFilterFindings(fileData.findings);
    filtered.forEach(function(f) {
      byCat[f.tier_id] = (byCat[f.tier_id] || 0) + 1;
      if (f.investigated) byCat['investigated'] = (byCat['investigated'] || 0) + 1;
      sevs[f.severity]++;
      total++;
    });
    // Also count investigated findings that are currently hidden
    if (!reconActiveDims['investigated']) {
      (fileData.findings || []).forEach(function(f) {
        if (f.investigated) byCat['investigated'] = (byCat['investigated'] || 0) + 1;
      });
    }
  }
  return { byCat: byCat, sevs: sevs, total: total };
}

/* ── Start ── */
startPolling();

})();
