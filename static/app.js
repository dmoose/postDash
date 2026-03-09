const feed = document.getElementById('feed');
const countBadge = document.getElementById('count');
const emptyMsg = document.getElementById('empty-msg');
const btnPause = document.getElementById('btn-pause');
const btnClear = document.getElementById('btn-clear');
const fSource = document.getElementById('f-source');
const fChannel = document.getElementById('f-channel');
const fAction = document.getElementById('f-action');
const fLevel = document.getElementById('f-level');
const fAgent = document.getElementById('f-agent');
const channelBar = document.getElementById('channel-bar');
const sparklineSvg = document.getElementById('sparkline');
const topBar = document.querySelector('.top-bar');
const eventsView = document.getElementById('events-view');
const logsView = document.getElementById('logs-view');
const pageView = document.getElementById('page-view');

const sRate = document.getElementById('s-rate');
const sTotal = document.getElementById('s-total');
const sClients = document.getElementById('s-clients');
const sErrors = document.getElementById('s-errors');
const sErrorsWrap = document.getElementById('s-errors-wrap');
const sWarns = document.getElementById('s-warns');
const sWarnsWrap = document.getElementById('s-warns-wrap');
const sSources = document.getElementById('s-sources');

let count = 0;
let paused = false;
let es = null;
let activeChannel = '';
let currentView = 'events';

const seen = { source: new Set(), channel: new Set(), action: new Set(), level: new Set(), agent_id: new Set() };

// --- Navigation ---

function switchView(view, slug) {
  currentView = view;
  topBar.querySelectorAll('.nav-tab').forEach(t => {
    t.classList.toggle('active', t.dataset.view === view && t.dataset.slug === (slug || ''));
  });

  eventsView.style.display = view === 'events' ? 'block' : 'none';
  logsView.style.display = view === 'logs' ? 'block' : 'none';
  pageView.style.display = (view !== 'events' && view !== 'logs') ? 'block' : 'none';
  countBadge.style.display = view === 'events' ? '' : 'none';

  if (view === 'logs') {
    if (!logSSEConnected) connectLogSSE();
  } else if (view !== 'events' && view !== 'logs') {
    loadPage(view, slug);
  }
}

function loadPage(view, slug) {
  pageView.innerHTML = '<div style="padding:40px;color:#8b949e;">Loading...</div>';

  if (view === 'status') {
    fetch('/api/status')
      .then(r => r.json())
      .then(sections => renderStatus(sections))
      .catch(err => { pageView.innerHTML = `<div class="page-error">Failed to load: ${esc(err.message)}</div>`; });
    return;
  }

  fetch(`/api/pages/${slug}`)
    .then(r => r.json())
    .then(result => renderPage(slug, result))
    .catch(err => { pageView.innerHTML = `<div class="page-error">Failed to load: ${esc(err.message)}</div>`; });
}

function renderStatus(sections) {
  const cards = sections.map(s => {
    const rows = s.items.map(([k, v]) =>
      `<div class="status-row"><span class="status-key">${esc(k)}</span><span class="status-val">${esc(v)}</span></div>`
    ).join('');
    return `<div class="status-section"><h3>${esc(s.title)}</h3>${rows}</div>`;
  }).join('');

  pageView.innerHTML = `
    <div class="page-header">
      <span class="page-title">Status</span>
      <button class="page-refresh" onclick="loadPage('status')">Refresh</button>
    </div>
    <div class="status-grid">${cards}</div>
  `;
}

function renderPage(name, result) {
  const updated = result.updated_at ? new Date(result.updated_at).toLocaleTimeString() : '';
  const errorHtml = result.error ? `<div class="page-error">Command error: ${esc(result.error)}</div>` : '';

  let contentHtml;
  switch (result.format) {
    case 'json':
      contentHtml = `<div class="page-content"><pre>${syntaxHighlightJSON(result.content)}</pre></div>`;
      break;
    case 'markdown':
      contentHtml = `<div class="page-content md">${renderMarkdown(result.content)}</div>`;
      break;
    case 'yaml':
      contentHtml = `<div class="page-content"><pre>${esc(result.content)}</pre></div>`;
      break;
    default: // text
      contentHtml = `<div class="page-content"><pre>${esc(result.content)}</pre></div>`;
  }

  pageView.innerHTML = `
    <div class="page-header">
      <span class="page-title">${esc(name)}</span>
      <span class="page-meta">updated ${updated}</span>
      <button class="page-refresh" onclick="loadPage('${currentView}', '${esc(name)}')">Refresh</button>
    </div>
    ${errorHtml}
    ${contentHtml}
  `;
}

function syntaxHighlightJSON(raw) {
  try {
    const obj = typeof raw === 'string' ? JSON.parse(raw) : raw;
    const formatted = JSON.stringify(obj, null, 2);
    // Escape HTML FIRST to prevent XSS, then apply highlighting
    const safe = esc(formatted);
    return safe
      .replace(/(&quot;.*?&quot;)(\s*:\s*)/g, '<span class="json-key">$1</span>$2')
      .replace(/:\s*(&quot;.*?&quot;)/g, ': <span class="json-string">$1</span>')
      .replace(/:\s*(\d+\.?\d*)/g, ': <span class="json-number">$1</span>')
      .replace(/:\s*(true|false)/g, ': <span class="json-bool">$1</span>')
      .replace(/:\s*(null)/g, ': <span class="json-null">$1</span>');
  } catch {
    return esc(raw);
  }
}

function renderMarkdown(text) {
  // Split into code blocks and non-code sections to protect code from formatting
  const parts = text.split(/(```[\s\S]*?```)/g);
  let inTable = false;
  let tableRows = [];

  function flushTable() {
    if (!tableRows.length) return '';
    const headerCells = tableRows[0].map(c => `<th>${formatInline(esc(c))}</th>`).join('');
    const bodyRows = tableRows.slice(1).map(row =>
      '<tr>' + row.map(c => `<td>${formatInline(esc(c))}</td>`).join('') + '</tr>'
    ).join('');
    tableRows = [];
    inTable = false;
    return `<table><thead><tr>${headerCells}</tr></thead><tbody>${bodyRows}</tbody></table>`;
  }

  return parts.map(part => {
    if (part.startsWith('```')) {
      const code = part.replace(/^```\w*\n?/, '').replace(/```$/, '');
      return '<pre><code>' + esc(code) + '</code></pre>';
    }
    let result = '';
    const lines = part.split('\n');
    for (const line of lines) {
      const trimmed = line.trim();

      // Table rows: | col | col |
      if (/^\|(.+)\|$/.test(trimmed)) {
        const cells = trimmed.slice(1, -1).split('|').map(c => c.trim());
        // Skip separator rows (|---|---|)
        if (cells.every(c => /^[-:]+$/.test(c))) continue;
        if (!inTable) inTable = true;
        tableRows.push(cells);
        continue;
      }

      // End of table
      if (inTable) result += flushTable();

      const escaped = esc(line);
      // Headers
      if (/^#### /.test(line)) { result += '<h4>' + formatInline(escaped.slice(5)) + '</h4>\n'; continue; }
      if (/^### /.test(line)) { result += '<h3>' + formatInline(escaped.slice(4)) + '</h3>\n'; continue; }
      if (/^## /.test(line)) { result += '<h2>' + formatInline(escaped.slice(3)) + '</h2>\n'; continue; }
      if (/^# /.test(line)) { result += '<h1>' + formatInline(escaped.slice(2)) + '</h1>\n'; continue; }
      // Horizontal rule
      if (/^---+$/.test(trimmed)) { result += '<hr>\n'; continue; }
      // Blockquote
      if (/^> /.test(line)) { result += '<blockquote><p>' + formatInline(esc(line.slice(2))) + '</p></blockquote>\n'; continue; }
      // Ordered list
      if (/^\d+\. /.test(line)) { result += '<li class="ol">' + formatInline(esc(line.replace(/^\d+\.\s*/, ''))) + '</li>\n'; continue; }
      // Unordered list
      if (/^- /.test(line)) { result += '<li>' + formatInline(escaped.slice(2)) + '</li>\n'; continue; }
      // Empty line
      if (trimmed === '') { result += '\n'; continue; }
      // Paragraph
      result += '<p>' + formatInline(escaped) + '</p>\n';
    }
    if (inTable) result += flushTable();
    // Wrap consecutive <li> in <ul>, <li class="ol"> in <ol>
    result = result.replace(/(<li class="ol">[\s\S]*?<\/li>\n?)+/g, m => '<ol>' + m.replace(/ class="ol"/g, '') + '</ol>');
    result = result.replace(/(<li>(?:(?!<li class)[\s\S])*?<\/li>\n?)+/g, '<ul>$&</ul>');
    return result;
  }).join('');
}

function formatInline(escaped) {
  return escaped
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/`([^`]+)`/g, '<code>$1</code>');
}

// Load nav tabs from server
function loadNav() {
  fetch('/api/pages')
    .then(r => r.json())
    .then(pages => {
      for (const page of pages) {
        const tab = document.createElement('div');
        tab.className = 'nav-tab';
        tab.dataset.view = 'page';
        tab.dataset.slug = page.slug;
        tab.textContent = page.name;
        tab.onclick = () => switchView('page', page.slug);
        topBar.appendChild(tab);
      }
    })
    .catch(() => {});
}

// Wire up static nav tabs
topBar.querySelector('[data-view="events"]').onclick = () => switchView('events');
topBar.querySelector('[data-view="logs"]').onclick = () => switchView('logs');
topBar.querySelector('[data-view="status"]').onclick = () => switchView('status');

// --- SSE ---

function connectSSE() {
  if (es) es.close();
  es = new EventSource('/events/stream');
  es.onmessage = (e) => {
    if (paused) return;
    addEvent(JSON.parse(e.data));
  };
  es.onerror = () => setTimeout(connectSSE, 2000);
}

function addEvent(evt) {
  emptyMsg.style.display = 'none';
  count++;
  countBadge.textContent = count;

  if (evt.source) seen.source.add(evt.source);
  if (evt.channel) seen.channel.add(evt.channel);
  if (evt.action) seen.action.add(evt.action);
  if (evt.level) seen.level.add(evt.level);
  if (evt.agent_id) seen.agent_id.add(evt.agent_id);
  updateDataLists();
  updateChannelTabs();

  const div = document.createElement('div');
  const level = evt.level || 'info';
  div.className = 'event level-' + level;
  div.onclick = (e) => { if (e.target.classList.contains('inspect-btn')) { e.stopPropagation(); div.classList.toggle('expanded'); } };
  div.dataset.source = evt.source || '';
  div.dataset.channel = evt.channel || '';
  div.dataset.action = evt.action || '';
  div.dataset.level = level;
  div.dataset.agent = evt.agent_id || '';

  const ts = new Date(evt.ts).toLocaleTimeString();

  const data = evt.data || {};
  const dataStr = Object.keys(data).length > 0 ? JSON.stringify(data) : '';
  const detail = dataStr ? syntaxHighlightJSON(data) : '';

  div.innerHTML = `
    <div class="row">
      <span class="level">${esc(level)}</span>
      <span class="source" title="${esc(evt.source || '')}">${esc(evt.source || '')}</span>
      <span class="channel" title="${esc(evt.channel || '')}">${esc(evt.channel || '')}</span>
      <span class="agent" title="${esc(evt.agent_id || '')}">${esc(evt.agent_id || '')}</span>
      <span class="action" title="${esc(evt.action || '')}">${esc(evt.action || '')}</span>
      <span class="duration">${evt.duration_ms != null ? (evt.duration_ms >= 500 ? (evt.duration_ms / 1000).toFixed(1) + 's' : evt.duration_ms + 'ms') : ''}</span>
      <span class="inline-data" title="${esc(dataStr)}">${esc(dataStr)}</span>
      <span class="time">${ts}</span>
      <span class="seq">${evt.seq}</span>
      <button class="inspect-btn" title="Inspect">&#9776;</button>
    </div>
    ${detail ? `<div class="detail">${detail}</div>` : ''}
  `;

  feed.insertBefore(div, emptyMsg.nextSibling);
  applyFilterToElement(div);

  while (feed.querySelectorAll('.event').length > 500) {
    const events = feed.querySelectorAll('.event');
    events[events.length - 1].remove();
  }
}

function esc(s) {
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}

// --- Filtering ---

function getFilters() {
  return {
    source: fSource.value.toLowerCase(),
    channel: activeChannel.toLowerCase() || fChannel.value.toLowerCase(),
    action: fAction.value.toLowerCase(),
    level: fLevel.value.toLowerCase(),
    agent: fAgent.value.toLowerCase(),
  };
}

function applyFilterToElement(el) {
  const f = getFilters();
  const show =
    (!f.source || el.dataset.source.toLowerCase().includes(f.source)) &&
    (!f.channel || el.dataset.channel.toLowerCase().includes(f.channel)) &&
    (!f.action || el.dataset.action.toLowerCase().includes(f.action)) &&
    (!f.level || el.dataset.level.toLowerCase() === f.level) &&
    (!f.agent || el.dataset.agent.toLowerCase().includes(f.agent));
  el.style.display = show ? '' : 'none';
}

function applyFilters() {
  feed.querySelectorAll('.event').forEach(applyFilterToElement);
}

// --- Channel tabs ---

function updateChannelTabs() {
  const channels = [...seen.channel].sort();
  const existing = new Set([...channelBar.querySelectorAll('.channel-tab')].map(t => t.dataset.channel));
  for (const ch of channels) {
    if (!existing.has(ch)) {
      const tab = document.createElement('div');
      tab.className = 'channel-tab';
      tab.dataset.channel = ch;
      tab.textContent = ch;
      tab.onclick = () => selectChannel(ch);
      channelBar.appendChild(tab);
    }
  }
}

function selectChannel(ch) {
  activeChannel = ch;
  fChannel.value = '';
  channelBar.querySelectorAll('.channel-tab').forEach(tab => {
    tab.classList.toggle('active', tab.dataset.channel === ch);
  });
  applyFilters();
}

channelBar.querySelector('.channel-tab').onclick = () => selectChannel('');

// --- Datalists ---

function updateDataLists() {
  updateDataList('dl-source', seen.source);
  updateDataList('dl-channel', seen.channel);
  updateDataList('dl-action', seen.action);
  updateDataList('dl-level', seen.level);
  updateDataList('dl-agent', seen.agent_id);
}

function updateDataList(id, values) {
  let dl = document.getElementById(id);
  if (!dl) { dl = document.createElement('datalist'); dl.id = id; document.body.appendChild(dl); }
  if (dl.children.length === values.size) return;
  dl.innerHTML = '';
  for (const v of [...values].sort()) {
    const opt = document.createElement('option');
    opt.value = v;
    dl.appendChild(opt);
  }
}

// --- Sparkline ---

function renderSparkline(data) {
  const w = 120, h = 20;
  const max = Math.max(...data, 1);
  const step = w / (data.length - 1 || 1);
  const points = data.map((v, i) => `${i * step},${h - (v / max) * (h - 2) - 1}`).join(' ');
  sparklineSvg.innerHTML = `
    <polyline points="${points}" fill="none" stroke="#58a6ff" stroke-width="1.5" stroke-linejoin="round"/>
    <polyline points="0,${h} ${points} ${w},${h}" fill="#58a6ff10" stroke="none"/>
  `;
}

function pollSparkline() {
  fetch('/events/rate?buckets=30&minutes=5')
    .then(r => r.json())
    .then(data => { if (data && data.length) renderSparkline(data); })
    .catch(() => {});
}

// --- Stats ---

function pollStats() {
  fetch('/events/stats')
    .then(r => r.json())
    .then(stats => {
      sRate.textContent = stats.events_per_second.toFixed(1) + '/s';
      sTotal.textContent = stats.total_events.toLocaleString();
      sClients.textContent = stats.client_count;

      const errors = stats.by_level?.error || 0;
      sErrorsWrap.style.display = errors > 0 ? '' : 'none';
      if (errors > 0) sErrors.textContent = errors + ' error' + (errors !== 1 ? 's' : '');

      const warns = stats.by_level?.warn || 0;
      sWarnsWrap.style.display = warns > 0 ? '' : 'none';
      if (warns > 0) sWarns.textContent = warns + ' warn' + (warns !== 1 ? 's' : '');

      sSources.innerHTML = '';
      for (const [src, cnt] of Object.entries(stats.by_source || {})) {
        const tag = document.createElement('span');
        tag.className = 'source-tag';
        tag.textContent = `${src}: ${cnt}`;
        tag.onclick = () => { fSource.value = src; applyFilters(); };
        sSources.appendChild(tag);
      }
    })
    .catch(() => {});
}

// --- Controls ---

btnPause.onclick = () => {
  paused = !paused;
  btnPause.textContent = paused ? 'Resume' : 'Pause';
  btnPause.classList.toggle('active', paused);
};

btnClear.onclick = () => {
  feed.querySelectorAll('.event').forEach(e => e.remove());
  count = 0;
  countBadge.textContent = 0;
};

[fSource, fChannel, fAction, fLevel, fAgent].forEach(input => {
  input.addEventListener('input', applyFilters);
});

// --- Logs tab ---

const logFeed = document.getElementById('log-feed');
const logEmptyMsg = document.getElementById('log-empty-msg');
const logBtnPause = document.getElementById('log-btn-pause');
const logBtnClear = document.getElementById('log-btn-clear');
const lfLevel = document.getElementById('lf-level');
const lfLogger = document.getElementById('lf-logger');
const lfSearch = document.getElementById('lf-search');
const lsAccepted = document.getElementById('ls-accepted');
const lsGated = document.getElementById('ls-gated');
const lsMinLevel = document.getElementById('ls-min-level');
const lsErrors = document.getElementById('ls-errors');
const lsErrorsWrap = document.getElementById('ls-errors-wrap');

const logSparklineSvg = document.getElementById('log-sparkline');
const lsRate = document.getElementById('ls-rate');

let logCount = 0;
let logPaused = false;
let logES = null;
let logSSEConnected = false;
const logSeen = { level: new Set(), logger: new Set() };

function connectLogSSE() {
  if (logES) logES.close();
  logES = new EventSource('/logs/stream');
  logES.onmessage = (e) => {
    if (logPaused) return;
    addLog(JSON.parse(e.data));
  };
  logES.onerror = () => setTimeout(connectLogSSE, 2000);
  logSSEConnected = true;
}

function addLog(entry) {
  logEmptyMsg.style.display = 'none';
  logCount++;

  if (entry.level) logSeen.level.add(entry.level);
  if (entry.logger) logSeen.logger.add(entry.logger);
  updateLogDataLists();

  const div = document.createElement('div');
  const level = entry.level || 'info';
  div.className = 'log-entry level-' + level;
  div.dataset.level = level;
  div.dataset.logger = entry.logger || '';
  div.dataset.message = (entry.message || '').toLowerCase();

  const ts = new Date(entry.ts).toLocaleTimeString();
  const fields = entry.fields || {};
  const fieldsStr = Object.keys(fields).length > 0 ? JSON.stringify(fields) : '';

  let detailParts = [];
  if (entry.message) detailParts.push(`<div class="detail-message">${esc(entry.message)}</div>`);
  if (entry.caller) detailParts.push(`<div class="detail-caller">${esc(entry.caller)}</div>`);
  if (fieldsStr) detailParts.push(`<pre>${syntaxHighlightJSON(fields)}</pre>`);

  div.innerHTML = `
    <div class="row">
      <span class="level">${esc(level)}</span>
      <span class="logger" title="${esc(entry.logger || '')}">${esc(entry.logger || '')}</span>
      <span class="message" title="${esc(entry.message || '')}">${esc(entry.message || '')}</span>
      <span class="time">${ts}</span>
      <span class="seq">${entry.seq}</span>
      <button class="inspect-btn" title="Inspect">&#9776;</button>
    </div>
    ${detailParts.length ? `<div class="detail">${detailParts.join('')}</div>` : ''}
  `;

  div.onclick = (e) => { if (e.target.classList.contains('inspect-btn')) { e.stopPropagation(); div.classList.toggle('expanded'); } };

  logFeed.insertBefore(div, logEmptyMsg.nextSibling);
  applyLogFilterToElement(div);

  while (logFeed.querySelectorAll('.log-entry').length > 500) {
    const entries = logFeed.querySelectorAll('.log-entry');
    entries[entries.length - 1].remove();
  }
}

function getLogFilters() {
  return {
    level: lfLevel.value.toLowerCase(),
    logger: lfLogger.value.toLowerCase(),
    search: lfSearch.value.toLowerCase(),
  };
}

function applyLogFilterToElement(el) {
  const f = getLogFilters();
  const show =
    (!f.level || el.dataset.level === f.level) &&
    (!f.logger || el.dataset.logger.toLowerCase().includes(f.logger)) &&
    (!f.search || el.dataset.message.includes(f.search));
  el.style.display = show ? '' : 'none';
}

function applyLogFilters() {
  logFeed.querySelectorAll('.log-entry').forEach(applyLogFilterToElement);
}

[lfLevel, lfLogger, lfSearch].forEach(input => {
  input.addEventListener('input', applyLogFilters);
});

logBtnPause.onclick = () => {
  logPaused = !logPaused;
  logBtnPause.textContent = logPaused ? 'Resume' : 'Pause';
  logBtnPause.classList.toggle('active', logPaused);
};

logBtnClear.onclick = () => {
  logFeed.querySelectorAll('.log-entry').forEach(e => e.remove());
  logCount = 0;
};

function updateLogDataLists() {
  updateDataList('dl-log-level', logSeen.level);
  updateDataList('dl-log-logger', logSeen.logger);
}

function renderLogSparkline(data) {
  const w = 120, h = 20;
  const max = Math.max(...data, 1);
  const step = w / (data.length - 1 || 1);
  const points = data.map((v, i) => `${i * step},${h - (v / max) * (h - 2) - 1}`).join(' ');
  logSparklineSvg.innerHTML = `
    <polyline points="${points}" fill="none" stroke="#58a6ff" stroke-width="1.5" stroke-linejoin="round"/>
    <polyline points="0,${h} ${points} ${w},${h}" fill="#58a6ff10" stroke="none"/>
  `;
}

function pollLogSparkline() {
  fetch('/logs/rate?buckets=30&minutes=5')
    .then(r => r.json())
    .then(data => { if (data && data.length) renderLogSparkline(data); })
    .catch(() => {});
}

function pollLogStats() {
  fetch('/logs/stats')
    .then(r => r.json())
    .then(stats => {
      lsRate.textContent = stats.logs_per_second.toFixed(1) + '/s';
      lsAccepted.textContent = stats.accepted.toLocaleString();
      lsGated.textContent = stats.gated.toLocaleString();
      lsMinLevel.textContent = stats.min_level;
      const errors = stats.by_level?.error || 0;
      lsErrorsWrap.style.display = errors > 0 ? '' : 'none';
      if (errors > 0) lsErrors.textContent = errors + ' error' + (errors !== 1 ? 's' : '');
    })
    .catch(() => {});
}

// --- Export ---

// Toggle menus
document.getElementById('btn-export').onclick = () => {
  document.getElementById('export-menu').classList.toggle('open');
  document.getElementById('log-export-menu').classList.remove('open');
};
document.getElementById('log-btn-export').onclick = () => {
  document.getElementById('log-export-menu').classList.toggle('open');
  document.getElementById('export-menu').classList.remove('open');
};
// Close on outside click
document.addEventListener('click', (e) => {
  if (!e.target.closest('.export-wrap')) {
    document.querySelectorAll('.export-menu').forEach(m => m.classList.remove('open'));
  }
});

function doExport(type, action) {
  const isLogs = type === 'logs';
  const format = document.getElementById(isLogs ? 'log-exp-format' : 'exp-format').value;
  const count = document.getElementById(isLogs ? 'log-exp-count' : 'exp-count').value;
  const copiedEl = document.getElementById(isLogs ? 'log-exp-copied' : 'exp-copied');

  // Build URL with current filters
  const params = new URLSearchParams({ n: count });
  if (isLogs) {
    if (lfLevel.value) params.set('level', lfLevel.value);
  } else {
    if (fSource.value) params.set('source', fSource.value);
    if (fChannel.value || activeChannel) params.set('channel', fChannel.value || activeChannel);
    if (fAction.value) params.set('action', fAction.value);
    if (fLevel.value) params.set('level', fLevel.value);
    if (fAgent.value) params.set('agent_id', fAgent.value);
  }

  const url = isLogs ? '/logs/recent' : '/events/recent';
  fetch(`${url}?${params}`)
    .then(r => r.json())
    .then(data => {
      const output = formatExport(data, format);
      const ext = format === 'csv' ? 'csv' : (format === 'jsonl' ? 'jsonl' : 'json');
      const mime = format === 'csv' ? 'text/csv' : 'application/json';
      const filename = `${type}-export-${new Date().toISOString().slice(0,19).replace(/:/g,'-')}.${ext}`;

      if (action === 'copy') {
        navigator.clipboard.writeText(output).then(() => {
          copiedEl.style.display = 'block';
          copiedEl.textContent = `Copied ${data.length} ${type}!`;
          setTimeout(() => { copiedEl.style.display = 'none'; }, 2000);
        });
      } else {
        const blob = new Blob([output], { type: mime });
        const a = document.createElement('a');
        a.href = URL.createObjectURL(blob);
        a.download = filename;
        a.click();
        URL.revokeObjectURL(a.href);
        // Close menu after download
        document.querySelectorAll('.export-menu').forEach(m => m.classList.remove('open'));
      }
    })
    .catch(err => {
      copiedEl.style.display = 'block';
      copiedEl.textContent = 'Export failed';
      copiedEl.style.color = '#f85149';
      setTimeout(() => { copiedEl.style.display = 'none'; copiedEl.style.color = ''; }, 2000);
    });
}

function formatExport(data, format) {
  if (format === 'json') return JSON.stringify(data, null, 2);
  if (format === 'jsonl') return data.map(d => JSON.stringify(d)).join('\n');
  // CSV
  if (!data.length) return '';
  const keys = Object.keys(data[0]);
  const rows = [keys.join(',')];
  for (const item of data) {
    rows.push(keys.map(k => {
      const v = item[k];
      if (v == null) return '';
      if (typeof v === 'object') return '"' + JSON.stringify(v).replace(/"/g, '""') + '"';
      const s = String(v);
      return s.includes(',') || s.includes('"') || s.includes('\n') ? '"' + s.replace(/"/g, '""') + '"' : s;
    }).join(','));
  }
  return rows.join('\n');
}

// --- Bootstrap ---

loadNav();

// Load recent logs in background
fetch('/logs/recent?n=50')
  .then(r => r.json())
  .then(entries => { if (entries && entries.length) entries.forEach(addLog); })
  .catch(() => {});

fetch('/events/recent?n=50')
  .then(r => r.json())
  .then(events => {
    if (events && events.length) events.forEach(addEvent);
    connectSSE();
    pollStats();
    pollSparkline();
    pollLogStats();
    pollLogSparkline();
    setInterval(pollStats, 3000);
    setInterval(pollSparkline, 5000);
    setInterval(pollLogStats, 3000);
    setInterval(pollLogSparkline, 5000);
  })
  .catch(() => {
    connectSSE();
    setInterval(pollStats, 3000);
    setInterval(pollSparkline, 5000);
    setInterval(pollLogStats, 3000);
    setInterval(pollLogSparkline, 5000);
  });
