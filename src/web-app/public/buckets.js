// Frontend logic for the Buckets tab: load buckets, subscribe to live updates, create new
// ones, and switch tabs. Mirrors app.js but for the storage.workshop.io/Bucket resource.

const buckets = new Map();

const bucketsBody = document.getElementById('buckets-body');
const bucketForm = document.getElementById('bucket-form');
const bucketSubmitBtn = document.getElementById('bucket-submit-btn');
const bucketConnection = document.getElementById('bucket-connection');

function bucketPhaseBadge(phase) {
  const cls = {
    Pending: 'badge-pending',
    Ready: 'badge-completed',
    Failed: 'badge-failed',
    Terminating: 'badge-processing',
  }[phase] || 'badge-pending';
  return `<span class="badge ${cls}">${phase}</span>`;
}

function bucketEsc(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[c]));
}

function renderBuckets() {
  if (buckets.size === 0) {
    bucketsBody.innerHTML = '<tr class="empty"><td colspan="5">No buckets yet. Create one above.</td></tr>';
    return;
  }
  const rows = [...buckets.values()]
    .sort((a, b) => (b.creationTimestamp || '').localeCompare(a.creationTimestamp || ''))
    .map((b) => {
      const policy = b.accessPolicy === 'public-read'
        ? '<span class="tag tag-public">public-read</span>'
        : '<span class="tag">private</span>';
      return `<tr>
        <td><code>${bucketEsc(b.bucketName || b.name)}</code></td>
        <td>${bucketPhaseBadge(b.phase)}</td>
        <td>${policy}</td>
        <td>${bucketEsc(b.quota || '—')}</td>
        <td>${bucketEsc(b.message || '')}</td>
      </tr>`;
    });
  bucketsBody.innerHTML = rows.join('');
}

async function loadBuckets() {
  try {
    const res = await fetch('/api/buckets');
    const items = await res.json();
    items.forEach((b) => buckets.set(b.name, b));
    renderBuckets();
  } catch (err) {
    console.error('failed to load buckets', err);
  }
}

function subscribeBuckets() {
  const source = new EventSource('/api/buckets/stream');
  source.addEventListener('open', () => {
    bucketConnection.textContent = 'live';
    bucketConnection.className = 'badge badge-completed';
  });
  source.addEventListener('change', (e) => {
    const { type, object } = JSON.parse(e.data);
    if (type === 'DELETED') {
      buckets.delete(object.name);
    } else {
      buckets.set(object.name, object);
    }
    renderBuckets();
  });
  source.addEventListener('error', () => {
    bucketConnection.textContent = 'reconnecting…';
    bucketConnection.className = 'badge badge-pending';
  });
}

bucketForm.addEventListener('submit', async (e) => {
  e.preventDefault();
  const data = Object.fromEntries(new FormData(bucketForm).entries());
  bucketSubmitBtn.disabled = true;
  try {
    const res = await fetch('/api/buckets', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || (await res.text()));
    }
    bucketForm.reset();
  } catch (err) {
    alert('Failed to create bucket: ' + err.message);
  } finally {
    bucketSubmitBtn.disabled = false;
  }
});

// ─── Tab switching ────────────────────────────────────────────────────────────
document.querySelectorAll('.tab-btn').forEach((btn) => {
  btn.addEventListener('click', () => {
    const tab = btn.dataset.tab;
    document.querySelectorAll('.tab-btn').forEach((b) => b.classList.toggle('active', b === btn));
    document.getElementById('tab-reports').hidden = tab !== 'reports';
    document.getElementById('tab-buckets').hidden = tab !== 'buckets';
  });
});

// ──────────────────────────────────────────────────────────────────────────────
loadBuckets();
subscribeBuckets();
