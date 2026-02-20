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
  if (ms >= 60000) return (ms / 60000).toFixed(1) + 'min';
  if (ms >= 1000) return (ms / 1000).toFixed(1) + 's';
  return ms + 'ms';
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
        safeFetch('/api/conversation/feed').then(function(d) { cache.convFeed = d; }).catch(function() {})
      ]).then(function() { renderDebrief(); });
      break;
    case 'arsenal':
      Promise.all([
        safeFetch('/api/sessions').then(function(d) { cache.sessions = d; }).catch(function() {}),
        safeFetch('/api/config').then(function(d) { cache.config = d; }).catch(function() {}),
        safeFetch('/api/runway').then(function(d) { cache.runway = d; }).catch(function() {})
      ]).then(function() { renderArsenal(); });
      break;
    case 'recon':
      // No data endpoints yet
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
  totalSaved += (rw.tokens_saved || 0);
  totalTimeSavedMs += (rw.time_saved_ms || 0);
  var ratio = totalReads > 0 ? totalGuided / totalReads : 0;

  // Hero metrics — all-time totals
  setGlow('hm-live-0', totalTimeSavedMs > 0 ? fmtTime(totalTimeSavedMs) : '-');
  setGlow('hm-live-1', totalSaved > 0 ? fmtK(totalSaved) : '-');
  setGlow('hm-live-2', fmtPct(ratio));
  setGlow('hm-live-3', rw.delta_minutes ? fmtMin(rw.delta_minutes) : '-');

  // Hero support line
  var parts = [];
  parts.push('<span class="g">' + fmtMin(rw.runway_minutes) + '</span> runway');
  parts.push('<span class="c">' + (st.domain_count || 0) + '</span> domains');
  parts.push('<span class="c">' + (st.prompt_count || 0) + '</span> prompts');
  if (rw.counterfact_minutes) parts.push('without aOa: <span class="r">' + fmtMin(rw.counterfact_minutes) + '</span>');
  setHtml('heroSupport-live', parts.join(' &middot; '));

  // Stats grid
  var promptN = st.prompt_count || 0;
  var autotuneProgress = promptN % 50;
  setGlow('ls-searches', st.prompt_count || 0);
  setGlow('ls-files', st.index_files || 0);
  setGlow('ls-autotune', autotuneProgress + '/50');
  setGlow('ls-burn', rw.burn_rate_per_min ? fmtK(Math.round(rw.burn_rate_per_min)) + '/min' : '-');
  setGlow('ls-guided', fmtPct(ratio));
  if (totalGuided > 0) {
    setGlow('ls-savings', fmtK(Math.round(totalSaved / totalGuided)));
  }
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

  // Hero metrics
  setGlow('hm-intel-0', st.domain_count || 0);
  setGlow('hm-intel-1', st.core_count || 0);
  setGlow('hm-intel-2', st.term_count || 0);
  setGlow('hm-intel-3', st.bigram_count || 0);

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
  setGlow('is-domains', st.domain_count || 0);
  setGlow('is-core', st.core_count || 0);
  setGlow('is-terms', st.term_count || 0);
  setGlow('is-keywords', st.keyword_count || 0);
  setGlow('is-bigrams', st.bigram_count || 0);
  setGlow('is-totalhits', totalHits ? totalHits.toFixed(1) : '0');

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
        if (uhc.textContent !== newVal) uhc.textContent = newVal;
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

  // Hero metrics
  setGlow('hm-debrief-0', fmtK(cm.input_tokens || 0));
  setGlow('hm-debrief-1', fmtK(cm.output_tokens || 0));
  setGlow('hm-debrief-2', fmtK(cm.cache_read_tokens || 0));
  setGlow('hm-debrief-3', fmtK(cm.cache_read_tokens || 0));

  // Hero support
  var totalTokens = (cm.input_tokens || 0) + (cm.output_tokens || 0);
  var sup = [];
  sup.push('<span class="c">' + (cm.turn_count || 0) + '</span> turns');
  sup.push('<span class="g">' + fmtK(totalTokens) + '</span> total tokens');
  sup.push('cache hit <span class="b">' + fmtPct(cm.cache_hit_rate || 0) + '</span>');
  sup.push('saved <span class="g">' + fmtK(cm.cache_read_tokens || 0) + '</span>');
  setHtml('heroSupport-debrief', sup.join(' &middot; '));

  // Stats
  setGlow('ds-input', fmtK(cm.input_tokens || 0));
  setGlow('ds-output', fmtK(cm.output_tokens || 0));
  setGlow('ds-cread', fmtK(cm.cache_read_tokens || 0));
  setGlow('ds-cwrite', fmtK(cm.cache_write_tokens || 0));
  setGlow('ds-hitrate', fmtPct(cm.cache_hit_rate || 0));
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
        '<div class="conv-msg-text">' + escapeHtml(truncText(ex.user.text, 500)) + '</div></div>';
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
        '<div class="conv-msg-text">' + renderMd(truncText(ex.assistant.text, 500)) + '</div></div>';
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
      // Savings cell: compact 4-char max (e.g. ↓88%)
      var saveVal = '';
      if (act.savings > 0) {
        saveVal = '<span class="text-green">\u2193' + act.savings + '%</span>';
      }
      // Tokens cell: compact 4-char max (e.g. 1.2k, 11k, 340)
      var tokVal = '';
      if (act.tokens > 0) {
        var tokCls = act.attrib === 'aOa guided' ? 'text-green' : (act.attrib === 'unguided' ? 'text-red' : 'text-dim');
        tokVal = '<span class="' + tokCls + '">' + fmtK(act.tokens) + '</span>';
      }
      var pathStyle = act.attrib === 'unguided' ? ' style="color:var(--red)"' : '';
      ahtml += '<div class="conv-action-item">' +
        '<span class="act-left">' +
          '<span class="conv-tool-chip ' + getToolChipClass(act.tool) + '">' + escapeHtml(act.tool) + '</span>' +
          '<span class="conv-action-path"' + pathStyle + ' title="' + escapeHtml(targetStr) + '">' + escapeHtml(truncPath(targetStr, 80)) + '</span>' +
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

  var sessions = ss.sessions || [];
  var sessionCount = ss.count || sessions.length;

  // Aggregate stats
  var totalSaved = rw.tokens_saved || 0;
  var totalTimeSavedMs = rw.time_saved_ms || 0;
  var totalReads = 0, totalGuidedReads = 0, totalPrompts = 0;

  for (var i = 0; i < sessions.length; i++) {
    var s = sessions[i];
    totalReads += (s.read_count || 0);
    totalGuidedReads += (s.guided_read_count || 0);
    totalPrompts += (s.prompt_count || 0);
    totalSaved += (s.tokens_saved || 0);
    totalTimeSavedMs += (s.time_saved_ms || 0);
  }

  var overallRatio = totalReads > 0 ? totalGuidedReads / totalReads : 0;
  var totalUnguided = Math.max(0, totalReads - totalGuidedReads);
  var readVelocity = totalPrompts > 0 ? (totalReads / totalPrompts).toFixed(1) : '-';

  // Hero metrics
  setGlow('hm-arsenal-0', fmtK(totalSaved));
  setGlow('hm-arsenal-1', rw.delta_minutes ? fmtMin(rw.delta_minutes) : '-');
  setGlow('hm-arsenal-2', fmtK(totalUnguided * 200));
  setGlow('hm-arsenal-3', fmtPct(overallRatio));

  // Hero support
  var sup = [];
  sup.push('<span class="g">' + fmtK(totalSaved) + '</span> tokens saved');
  if (totalTimeSavedMs > 0) sup.push('saved <span class="g">' + fmtTime(totalTimeSavedMs) + '</span>');
  if (rw.delta_minutes) sup.push('<span class="b">' + fmtMin(rw.delta_minutes) + '</span> extended');
  sup.push('<span class="c">' + sessionCount + '</span> sessions');
  sup.push('guided <span class="g">' + fmtPct(overallRatio) + '</span>');
  if (rw.counterfact_minutes) sup.push('without aOa: <span class="r">' + fmtMin(rw.counterfact_minutes) + '</span>');
  setHtml('heroSupport-arsenal', sup.join(' &middot; '));

  // Stats
  setGlow('as-saved', fmtK(totalSaved));
  setGlow('as-cost', fmtK(totalUnguided * 200));
  setGlow('as-sessions', sessionCount);
  setGlow('as-extended', rw.delta_minutes ? fmtMin(rw.delta_minutes) : '-');
  setGlow('as-ratio', fmtPct(overallRatio));
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

/* ── Start ── */
startPolling();

})();
