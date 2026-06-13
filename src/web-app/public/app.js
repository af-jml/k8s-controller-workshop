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
  updateHidden(); // ensure hidden field is current
  const data = Object.fromEntries(new FormData(form).entries());
  if (!data.dataset || !data.dataset.trim()) {
    alert('Please add at least one financial data row (label + value).');
    return;
  }
  submitBtn.disabled = true;
  try {
    const res = await fetch('/api/reports', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) throw new Error(await res.text());
    form.reset();
    resetDatasetEditor();
  } catch (err) {
    alert('Failed to create report: ' + err.message);
  } finally {
    submitBtn.disabled = false;
  }
});

// ─── Dataset row editor ───────────────────────────────────────────────────────
const presets = {
  pl: {
    title: 'Q2 P&L Summary',
    instructions: 'Highlight the largest cost centres and margin opportunities',
    rows: [
      ['Revenue', '1.20M'], ['Cost of goods', '0.55M'], ['Gross profit', '0.65M'],
      ['Operating expenses', '0.25M'], ['EBITDA', '0.40M'], ['Net profit', '0.33M'],
    ],
  },
  cashflow: {
    title: 'Q2 Cash Flow Statement',
    instructions: 'Focus on operating cash generation and working capital trends',
    rows: [
      ['Operating activities', '420K'], ['Capital expenditure', '-180K'],
      ['Free cash flow', '240K'], ['Financing activities', '-50K'],
      ['Net change in cash', '190K'],
    ],
  },
  budget: {
    title: 'Budget vs Actual Review',
    instructions: 'Identify the largest variances and their root causes',
    rows: [
      ['Revenue (budget)', '1.00M'], ['Revenue (actual)', '1.15M'],
      ['Costs (budget)', '0.70M'], ['Costs (actual)', '0.68M'],
      ['Variance (revenue)', '0.15M'], ['Variance (costs)', '0.02M'],
    ],
  },
};

const dsEditor = document.getElementById('dataset-editor');

function updateHidden() {
  const lines = [...dsEditor.querySelectorAll('.ds-row')]
    .map((row) => {
      const lbl = row.querySelector('.ds-label').value.trim();
      const val = row.querySelector('.ds-value').value.trim();
      return lbl && val ? `${lbl}: ${val}` : null;
    })
    .filter(Boolean);
  document.getElementById('dataset-hidden').value = lines.join('\n');
}

function createDsRow(label = '', value = '') {
  const row = document.createElement('div');
  row.className = 'ds-row';
  row.innerHTML =
    `<input class="ds-label" placeholder="e.g. Revenue" value="${escapeHtml(label)}" />` +
    `<span class="ds-sep">:</span>` +
    `<input class="ds-value" placeholder="e.g. 1.2M" value="${escapeHtml(value)}" />` +
    `<button type="button" class="ds-remove" aria-label="Remove row">&times;</button>`;
  row.querySelector('.ds-remove').addEventListener('click', () => { row.remove(); updateHidden(); });
  row.querySelectorAll('input').forEach((inp) => inp.addEventListener('input', updateHidden));
  return row;
}

function resetDatasetEditor() {
  dsEditor.innerHTML = '';
  for (let i = 0; i < 3; i++) dsEditor.appendChild(createDsRow());
  updateHidden();
}

document.getElementById('add-row-btn').addEventListener('click', () => {
  dsEditor.appendChild(createDsRow());
  updateHidden();
});

document.querySelectorAll('.preset-btn').forEach((btn) => {
  btn.addEventListener('click', () => {
    const p = presets[btn.dataset.preset];
    if (!p) return;
    form.querySelector('[name=title]').value = p.title;
    form.querySelector('[name=instructions]').value = p.instructions;
    dsEditor.innerHTML = '';
    p.rows.forEach(([l, v]) => dsEditor.appendChild(createDsRow(l, v)));
    updateHidden();
  });
});

// Initialise with three blank rows
resetDatasetEditor();

// ─────────────────────────────────────────────────────────────────────────────
loadInitial();
subscribe();
