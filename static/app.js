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

// Stats elements
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

// Track seen values for datalist suggestions
const seen = { source: new Set(), channel: new Set(), action: new Set(), level: new Set(), agent_id: new Set() };

function connectSSE() {
  if (es) es.close();
  // SSE stream is unfiltered — we filter client-side
  es = new EventSource('/events/stream');
  es.onmessage = (e) => {
    if (paused) return;
    const evt = JSON.parse(e.data);
    addEvent(evt);
  };
  es.onerror = () => {
    setTimeout(connectSSE, 2000);
  };
}

function addEvent(evt) {
  emptyMsg.style.display = 'none';
  count++;
  countBadge.textContent = count;

  // Track values for datalists
  if (evt.source) seen.source.add(evt.source);
  if (evt.channel) seen.channel.add(evt.channel);
  if (evt.action) seen.action.add(evt.action);
  if (evt.level) seen.level.add(evt.level);
  if (evt.agent_id) seen.agent_id.add(evt.agent_id);
  updateDataLists();

  const div = document.createElement('div');
  const level = evt.level || 'info';
  div.className = 'event level-' + level;
  div.onclick = () => div.classList.toggle('expanded');

  // Store filter-relevant data on the element
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

  // Apply current filter to new element
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

// --- Client-side filtering ---

function getFilters() {
  return {
    source: fSource.value.toLowerCase(),
    channel: fChannel.value.toLowerCase(),
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

// --- Datalist suggestions ---

function updateDataLists() {
  updateDataList('dl-source', seen.source);
  updateDataList('dl-channel', seen.channel);
  updateDataList('dl-action', seen.action);
  updateDataList('dl-level', seen.level);
  updateDataList('dl-agent', seen.agent_id);
}

function updateDataList(id, values) {
  let dl = document.getElementById(id);
  if (!dl) {
    dl = document.createElement('datalist');
    dl.id = id;
    document.body.appendChild(dl);
  }
  // Only update if values changed
  if (dl.children.length === values.size) return;
  dl.innerHTML = '';
  for (const v of [...values].sort()) {
    const opt = document.createElement('option');
    opt.value = v;
    dl.appendChild(opt);
  }
}

// --- Stats polling ---

function pollStats() {
  fetch('/events/stats')
    .then(r => r.json())
    .then(stats => {
      sRate.textContent = stats.events_per_second.toFixed(1) + '/s';
      sTotal.textContent = stats.total_events.toLocaleString();
      sClients.textContent = stats.client_count;

      const errors = stats.by_level?.error || 0;
      if (errors > 0) {
        sErrorsWrap.style.display = '';
        sErrors.textContent = errors + ' error' + (errors !== 1 ? 's' : '');
      } else {
        sErrorsWrap.style.display = 'none';
      }

      const warns = stats.by_level?.warn || 0;
      if (warns > 0) {
        sWarnsWrap.style.display = '';
        sWarns.textContent = warns + ' warn' + (warns !== 1 ? 's' : '');
      } else {
        sWarnsWrap.style.display = 'none';
      }

      sSources.innerHTML = '';
      for (const [src, cnt] of Object.entries(stats.by_source || {})) {
        const tag = document.createElement('span');
        tag.className = 'source-tag';
        tag.textContent = `${src}: ${cnt}`;
        tag.onclick = () => { fSource.value = src; applyFilters(); };
        tag.style.cursor = 'pointer';
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

// Filter inputs — client-side only, no SSE reconnect
[fSource, fChannel, fAction, fLevel, fAgent].forEach(input => {
  input.addEventListener('input', applyFilters);
});

// --- Bootstrap ---

// Wire up datalist attributes on inputs
fSource.setAttribute('list', 'dl-source');
fChannel.setAttribute('list', 'dl-channel');
fAction.setAttribute('list', 'dl-action');
fLevel.setAttribute('list', 'dl-level');
fAgent.setAttribute('list', 'dl-agent');

fetch('/events/recent?n=50')
  .then(r => r.json())
  .then(events => {
    if (events && events.length) events.forEach(addEvent);
    connectSSE();
    pollStats();
    setInterval(pollStats, 3000);
  })
  .catch(() => {
    connectSSE();
    setInterval(pollStats, 3000);
  });
