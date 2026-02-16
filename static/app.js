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

let count = 0;
let paused = false;
let es = null;

function buildStreamURL() {
  const params = new URLSearchParams();
  if (fSource.value) params.set('source', fSource.value);
  if (fChannel.value) params.set('channel', fChannel.value);
  if (fAction.value) params.set('action', fAction.value);
  if (fLevel.value) params.set('level', fLevel.value);
  if (fAgent.value) params.set('agent_id', fAgent.value);
  const qs = params.toString();
  return '/events/stream' + (qs ? '?' + qs : '');
}

function connect() {
  if (es) es.close();
  es = new EventSource(buildStreamURL());
  es.onmessage = (e) => {
    if (paused) return;
    addEvent(JSON.parse(e.data));
  };
  es.onerror = () => {
    setTimeout(connect, 2000);
  };
}

function addEvent(evt) {
  emptyMsg.style.display = 'none';
  count++;
  countBadge.textContent = count;

  const div = document.createElement('div');
  const level = evt.level || 'info';
  div.className = 'event level-' + level;
  div.onclick = () => div.classList.toggle('expanded');

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

  while (feed.children.length > 501) {
    feed.removeChild(feed.lastChild);
  }
}

function esc(s) {
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}

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

let filterTimer;
[fSource, fChannel, fAction, fLevel, fAgent].forEach(input => {
  input.addEventListener('input', () => {
    clearTimeout(filterTimer);
    filterTimer = setTimeout(connect, 300);
  });
});

fetch('/events/recent?n=50')
  .then(r => r.json())
  .then(events => {
    if (events && events.length) {
      events.forEach(addEvent);
    }
    connect();
  })
  .catch(() => connect());
