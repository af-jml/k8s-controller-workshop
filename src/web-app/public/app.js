// Frontend logic: load the current requests, subscribe to live updates, and submit new ones.

const reports = new Map();

const body = document.getElementById('reports-body');
const form = document.getElementById('report-form');
const submitBtn = document.getElementById('submit-btn');
const connection = document.getElementById('connection');

function phaseBadge(phase) {
  const cls = {
    Pending: 'badge-pending',
    Processing: 'badge-processing',
    Completed: 'badge-completed',
    Failed: 'badge-failed',
  }[phase] || 'badge-pending';
  return `<span class="badge ${cls}">${phase}</span>`;
}

function render() {
  if (reports.size === 0) {
    body.innerHTML = '<tr class="empty"><td colspan="5">No requests yet. Create one above.</td></tr>';
    return;
  }
  const rows = [...reports.values()]
    .sort((a, b) => (b.creationTimestamp || '').localeCompare(a.creationTimestamp || ''))
    .map((r) => {
      const pdf = r.phase === 'Completed'
        ? `<a class="download" href="/api/reports/${r.name}/pdf" target="_blank" rel="noopener">Download</a>`
        : '—';
      return `<tr>
        <td>${escapeHtml(r.title || r.name)}</td>
        <td>${phaseBadge(r.phase)}</td>
        <td>${escapeHtml(r.jobName || '—')}</td>
        <td>${escapeHtml(r.message || '')}</td>
        <td>${pdf}</td>
      </tr>`;
    });
  body.innerHTML = rows.join('');
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[c]));
}

async function loadInitial() {
  try {
    const res = await fetch('/api/reports');
    const items = await res.json();
    items.forEach((r) => reports.set(r.name, r));
    render();
  } catch (err) {
    console.error('failed to load reports', err);
  }
}

function subscribe() {
  const source = new EventSource('/api/reports/stream');
  source.addEventListener('open', () => {
    connection.textContent = 'live';
    connection.className = 'badge badge-completed';
  });
  source.addEventListener('change', (e) => {
    const { type, object } = JSON.parse(e.data);
    if (type === 'DELETED') {
      reports.delete(object.name);
    } else {
      reports.set(object.name, object);
    }
    render();
  });
  source.addEventListener('error', () => {
    connection.textContent = 'reconnecting…';
    connection.className = 'badge badge-pending';
  });
}

form.addEventListener('submit', async (e) => {
  e.preventDefault();
  submitBtn.disabled = true;
  const data = Object.fromEntries(new FormData(form).entries());
  try {
    const res = await fetch('/api/reports', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) throw new Error(await res.text());
    form.reset();
  } catch (err) {
    alert('Failed to create report: ' + err.message);
  } finally {
    submitBtn.disabled = false;
  }
});

loadInitial();
subscribe();
