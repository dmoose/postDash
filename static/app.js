const feed = document.getElementById('feed');
const countBadge = document.getElementById('count');
const emptyMsg = document.getElementById('empty-msg');
const btnPause = document.getElementById('btn-pause');
const btnClear = document.getElementById('btn-clear');
const fSource = document.getElementById('f-source');
const fAction = document.getElementById('f-action');
const fLevel = document.getElementById('f-level');
const fAgent = document.getElementById('f-agent');
const channelBar = document.getElementById('channel-bar');
const sparklineSvg = document.getElementById('sparkline');

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

const seen = { source: new Set(), channel: new Set(), action: new Set(), level: new Set(), agent_id: new Set() };

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
  div.onclick = () => div.classList.toggle('expanded');
  div.dataset.source = evt.source || '';
  div.dataset.channel = evt.channel || '';
  div.dataset.action = evt.action || '';
  div.dataset.level = level;
  div.dataset.agent = evt.agent_id || '';

  const ts = new Date(evt.ts).toLocaleTimeString();
  const parts = [`<span class="source">${esc(evt.source || '?')}</span>`];
  if (evt.channel) parts.push(`<span class="channel">${esc(evt.channel)}</span>`);
  if (evt.action) parts.push(`<span class="action">${esc(evt.action)}</span>`);
  parts.push(`<span class="level">${esc(level)}</span>`);
  if (evt.agent_id) parts.push(`<span class="agent">${esc(evt.agent_id)}</span>`);
  if (evt.duration_ms != null) parts.push(`<span class="duration">${evt.duration_ms}ms</span>`);
  parts.push(`<span class="time">#${evt.seq} ${ts}</span>`);

  div.innerHTML = `
    <div class="meta">${parts.join(' ')}</div>
    <div class="detail">${esc(JSON.stringify(evt.data || {}, null, 2))}</div>
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

// --- Filtering (client-side, includes channel tab) ---

function getFilters() {
  return {
    source: fSource.value.toLowerCase(),
    channel: activeChannel.toLowerCase(),
    action: fAction.value.toLowerCase(),
    level: fLevel.value.toLowerCase(),
    agent: fAgent.value.toLowerCase(),
  };
}

function applyFilterToElement(el) {
  const f = getFilters();
  const show =
    (!f.source || el.dataset.source.toLowerCase().includes(f.source)) &&
    (!f.channel || el.dataset.channel.toLowerCase() === f.channel) &&
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
  channelBar.querySelectorAll('.channel-tab').forEach(tab => {
    tab.classList.toggle('active', tab.dataset.channel === ch);
  });
  applyFilters();
}

// "all" tab handler
channelBar.querySelector('.channel-tab').onclick = () => selectChannel('');

// --- Datalists ---

function updateDataLists() {
  updateDataList('dl-source', seen.source);
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

[fSource, fAction, fLevel, fAgent].forEach(input => {
  input.addEventListener('input', applyFilters);
});

// --- Bootstrap ---

fetch('/events/recent?n=50')
  .then(r => r.json())
  .then(events => {
    if (events && events.length) events.forEach(addEvent);
    connectSSE();
    pollStats();
    pollSparkline();
    setInterval(pollStats, 3000);
    setInterval(pollSparkline, 5000);
  })
  .catch(() => {
    connectSSE();
    setInterval(pollStats, 3000);
    setInterval(pollSparkline, 5000);
  });
