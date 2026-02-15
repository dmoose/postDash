const feed = document.getElementById('feed');
const countBadge = document.getElementById('count');
const emptyMsg = document.getElementById('empty-msg');
const btnPause = document.getElementById('btn-pause');
const btnClear = document.getElementById('btn-clear');
const fSource = document.getElementById('f-source');
const fAction = document.getElementById('f-action');
const fAgent = document.getElementById('f-agent');

let count = 0;
let paused = false;
let es = null;

function buildStreamURL() {
  const params = new URLSearchParams();
  if (fSource.value) params.set('source', fSource.value);
  if (fAction.value) params.set('action', fAction.value);
  if (fAgent.value) params.set('agent_id', fAgent.value);
  const qs = params.toString();
  return '/events/stream' + (qs ? '?' + qs : '');
}

function connect() {
  if (es) es.close();
  es = new EventSource(buildStreamURL());
  es.onmessage = (e) => {
    if (paused) return;
    const evt = JSON.parse(e.data);
    addEvent(evt);
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
  div.className = 'event';
  div.onclick = () => div.classList.toggle('expanded');

  const ts = new Date(evt.ts).toLocaleTimeString();
  const agent = evt.agent_id ? `<span class="agent">${esc(evt.agent_id)}</span>` : '';
  const action = evt.action ? `<span class="action">${esc(evt.action)}</span>` : '';

  div.innerHTML = `
    <div class="meta">
      <span class="source">${esc(evt.source || '?')}</span>
      ${action}
      ${agent}
      <span class="time">#${evt.seq} ${ts}</span>
    </div>
    <div class="detail">${esc(JSON.stringify(evt.data || {}, null, 2))}</div>
  `;

  feed.insertBefore(div, feed.firstChild);

  // Cap displayed events
  while (feed.children.length > 500) {
    feed.removeChild(feed.lastChild);
  }
}

function esc(s) {
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}

// Pause/resume
btnPause.onclick = () => {
  paused = !paused;
  btnPause.textContent = paused ? 'Resume' : 'Pause';
  btnPause.classList.toggle('active', paused);
};

// Clear feed
btnClear.onclick = () => {
  feed.innerHTML = '';
  count = 0;
  countBadge.textContent = 0;
};

// Reconnect on filter change
let filterTimer;
[fSource, fAction, fAgent].forEach(input => {
  input.addEventListener('input', () => {
    clearTimeout(filterTimer);
    filterTimer = setTimeout(connect, 300);
  });
});

// Load recent events then connect to stream
fetch('/events/recent?n=50')
  .then(r => r.json())
  .then(events => {
    if (events && events.length) {
      events.forEach(addEvent);
    }
    connect();
  })
  .catch(() => connect());
