// ============================================================
// Reaplet — Vanilla JS Application (~200 lines, no deps)
// ============================================================

const THEMES = ['dark', 'light', 'catppuccin', 'nord', 'dracula'];
const THEME_LABELS = { dark: '🌙 Dark', light: '☀️ Light', catppuccin: '🐱 Catppuccin', nord: '❄️ Nord', dracula: '🧛 Dracula' };
const REFRESH_MS = 30000;

let state = { nodes: [], gcEvents: [], recommendations: [], loading: true, tab: 'nodes', autoRefresh: true, timer: null };

// --- Util ---
function formatBytes(b) {
  if (!b || b === 0) return '0 B';
  const u = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(b) / Math.log(1024));
  return `${(b / Math.pow(1024, i)).toFixed(1)} ${u[i]}`;
}
function pct(used, total) { return total ? ((used / total) * 100).toFixed(1) : '0.0'; }
function timeAgo(ts) {
  if (!ts) return '';
  const d = new Date(ts);
  return d.toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
}

// --- API ---
async function api(path) {
  const r = await fetch(`/api${path}`);
  if (!r.ok) throw new Error(`${r.status} ${r.statusText}`);
  return r.json();
}

async function loadAll() {
  state.loading = true;
  try {
    const [nodes, events, recs] = await Promise.all([api('/nodes'), api('/gc-events'), api('/recommendations')]);
    state.nodes = nodes || [];
    state.gcEvents = events || [];
    state.recommendations = recs || [];
  } catch (e) {
    toast(`Failed to load: ${e.message}`, 'error');
  }
  state.loading = false;
  render();
}

async function removeImage(imageRef, nodeName) {
  const r = await fetch('/api/remove-image', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ NodeName: nodeName, ImageRef: imageRef })
  });
  return r.json();
}

// --- Theme ---
function getTheme() { return localStorage.getItem('reaplet-theme') || 'dark'; }
function setTheme(t) { localStorage.setItem('reaplet-theme', t); document.documentElement.setAttribute('data-theme', t); render(); }

// --- Toast ---
function toast(msg, type = 'success') {
  const c = document.getElementById('toasts');
  const el = document.createElement('div');
  el.className = `toast toast-${type}`;
  el.innerHTML = `<span class="toast-icon">${type === 'success' ? '✓' : '✕'}</span><span class="toast-msg">${msg}</span><button class="toast-close" onclick="this.parentElement.remove()">×</button>`;
  c.appendChild(el);
  setTimeout(() => el.remove(), 5000);
}

// --- Auto refresh ---
function startPolling() { state.timer = setInterval(loadAll, REFRESH_MS); }
function stopPolling() { clearInterval(state.timer); state.timer = null; }
function toggleAutoRefresh() {
  state.autoRefresh = !state.autoRefresh;
  state.autoRefresh ? startPolling() : stopPolling();
  render();
}

// --- Render ---
function render() {
  document.getElementById('content').innerHTML = renderPanel();
  document.getElementById('tabs').innerHTML = renderTabs();
  document.getElementById('theme-dropdown').innerHTML = renderThemeDropdown();
  document.getElementById('auto-toggle').className = `auto-toggle${state.autoRefresh ? ' active' : ''}`;
  document.getElementById('auto-toggle').textContent = state.autoRefresh ? 'On' : 'Off';
  const ts = document.getElementById('last-updated');
  ts.innerHTML = state.loading ? '<span class="pulse-dot"></span> Loading...' : `<span class="pulse-dot"></span> ${new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}`;
}

function renderTabs() {
  const tabs = [['nodes', '🖥️', 'Nodes'], ['recommendations', '💡', 'Recommendations'], ['gc-events', '🗑️', 'GC Events']];
  return tabs.map(([id, icon, label]) =>
    `<button class="tab${state.tab === id ? ' active' : ''}" onclick="switchTab('${id}')"><span class="tab-icon">${icon}</span><span class="tab-label">${label}</span></button>`
  ).join('');
}

function switchTab(t) { state.tab = t; render(); }

function renderPanel() {
  if (state.loading && state.nodes.length === 0) return renderSkeleton();
  if (state.tab === 'nodes') return renderNodes();
  if (state.tab === 'recommendations') return renderRecommendations();
  if (state.tab === 'gc-events') return renderGCEvents();
}

function renderSkeleton() {
  return Array(3).fill('<div class="card"><div class="skeleton" style="width:40%;height:1.2rem"></div><div class="skeleton" style="width:100%;height:6px;margin-top:1rem"></div><div class="skeleton" style="width:60%;height:0.8rem;margin-top:0.5rem"></div></div>').join('');
}

// --- Nodes ---
function renderNodes() {
  if (!state.nodes.length) return '<div class="empty-state"><span class="icon">🖥️</span><p>No nodes found</p></div>';
  return state.nodes.map((n, i) => {
    const p = pct(n.EphemeralStorage?.AllocatedBytes || 0, n.EphemeralStorage?.CapacityBytes || 1);
    const level = p >= 90 ? 'critical' : p >= 75 ? 'warning' : '';
    return `<div class="card" id="node-${i}">
      <div class="card-header" onclick="toggleNode(${i})">
        <div class="node-identity"><span class="node-dot ${level}"></span><h3>${n.Name}</h3></div>
        <div class="badges"><span class="badge">📦 ${(n.Images || []).length}</span><span class="badge">${formatBytes(n.TotalImageSize)}</span></div>
      </div>
      <div class="progress-bar"><div class="progress-fill ${level}" style="width:${p}%"></div></div>
      <div class="storage-meta"><span>${formatBytes(n.EphemeralStorage?.AllocatedBytes || 0)} used</span><span class="storage-pct">${p}%</span><span>${formatBytes(n.EphemeralStorage?.CapacityBytes || 0)} total</span></div>
      <div class="image-table-wrap" id="images-${i}" style="display:none">${renderImageTable(n.Images)}</div>
    </div>`;
  }).join('');
}

function toggleNode(i) {
  const wrap = document.getElementById(`images-${i}`);
  const card = document.getElementById(`node-${i}`);
  const show = wrap.style.display === 'none';
  wrap.style.display = show ? 'block' : 'none';
  card.classList.toggle('expanded', show);
}

function renderImageTable(images) {
  if (!images || !images.length) return '<p style="color:var(--text-muted);font-size:0.85rem">No images</p>';
  const sorted = [...images].sort((a, b) => b.SizeBytes - a.SizeBytes);
  const rows = sorted.map(img => `<tr>
    <td><code>${img.Names?.[0] || 'unnamed'}</code></td>
    <td>${formatBytes(img.SizeBytes)}</td>
    <td><span class="pill ${img.InUse ? 'pill-success' : 'pill-danger'}">${img.InUse ? 'In Use' : 'Unused'}</span></td>
  </tr>`).join('');
  return `<table class="image-table"><thead><tr><th>Image</th><th>Size</th><th>Status</th></tr></thead><tbody>${rows}</tbody></table>`;
}

// --- GC Events ---
function renderGCEvents() {
  if (!state.gcEvents.length) return '<div class="empty-state"><span class="icon">🎉</span><p>No GC events — kubelet hasn\'t needed to garbage collect.</p></div>';
  const items = state.gcEvents.map(ev => {
    const type = (ev.Reason.includes('Failed') || ev.Reason.includes('Pressure')) ? 'critical' : (ev.Reason.includes('Succeeded') || ev.Reason.includes('NoDisk')) ? 'success' : '';
    const icon = ev.Reason.includes('Failed') ? '❌' : ev.Reason.includes('Pressure') ? '⚠️' : ev.Reason.includes('Succeeded') ? '✅' : '📋';
    return `<div class="timeline-item">
      <div class="timeline-marker ${type}">${icon}</div>
      <div class="timeline-content">
        <div class="event-header"><span class="event-reason ${type}">${ev.Reason}</span><span class="event-time">${timeAgo(ev.Timestamp)}</span></div>
        <p class="event-message">${ev.Message}</p>
      </div>
    </div>`;
  }).join('');
  return `<div class="timeline">${items}</div>`;
}

// --- Recommendations ---
function renderRecommendations() {
  const recs = state.recommendations;
  if (!recs.length) return '<div class="empty-state"><span class="icon">✨</span><p>All images are in use — nothing to clean up!</p></div>';
  const total = recs.reduce((s, r) => s + r.SavingsBytes, 0);
  const banner = `<div class="summary-banner">
    <div class="summary-stat"><span class="stat-number">${recs.length}</span><span class="stat-label">unused images</span></div>
    <div class="summary-divider"></div>
    <div class="summary-stat"><span class="stat-number">${formatBytes(total)}</span><span class="stat-label">reclaimable</span></div>
  </div>`;
  const cards = recs.map((r, i) => {
    const name = r.Image.Names?.[0] || 'unnamed';
    return `<div class="rec-card">
      <div class="rec-info"><code class="rec-image">${name}</code>
        <div class="rec-meta"><span>🖥️ ${r.NodeName}</span><span>📦 ${formatBytes(r.SavingsBytes)}</span><span>💡 ${r.Reason}</span></div>
      </div>
      <button class="btn-remove" onclick="handleRemove(${i})">Remove</button>
    </div>`;
  }).join('');
  return banner + cards;
}

async function handleRemove(i) {
  const rec = state.recommendations[i];
  const name = rec.Image.Names?.[0] || rec.Image.Names?.[1] || 'unnamed';
  showModal(
    'Remove Image',
    `<p>Remove <code>${name}</code> from <strong>${rec.NodeName}</strong>?</p><p class="modal-detail">This will free ${formatBytes(rec.SavingsBytes)}. The image can be re-pulled if needed.</p>`,
    'Remove',
    async () => {
      try {
        const result = await removeImage(name, rec.NodeName);
        if (result.Success) {
          const freed = result.FreedBytes > 0 ? result.FreedBytes : rec.SavingsBytes;
          toast(`Removed ${name} — freed ${formatBytes(freed)}`);
          loadAll();
        }
        else toast(`Failed: ${result.Error}`, 'error');
      } catch (e) { toast(e.message, 'error'); }
    }
  );
}

// --- Modal ---
let modalCallback = null;
function showModal(title, bodyHtml, confirmText, onConfirm) {
  document.getElementById('modal-title').textContent = title;
  document.getElementById('modal-body').innerHTML = bodyHtml;
  document.getElementById('modal-confirm').textContent = confirmText;
  modalCallback = onConfirm;
  document.getElementById('modal').classList.add('open');
  document.getElementById('modal-confirm').onclick = async () => { hideModal(); if (modalCallback) await modalCallback(); };
}
function modalCancel() { hideModal(); }
function hideModal() { document.getElementById('modal').classList.remove('open'); modalCallback = null; }

// --- Theme dropdown ---
function renderThemeDropdown() {
  const current = getTheme();
  return THEMES.map(t => `<button class="${t === current ? 'active' : ''}" onclick="setTheme('${t}');closeThemeDropdown()">${THEME_LABELS[t]}</button>`).join('');
}
function toggleThemeDropdown() {
  const dd = document.getElementById('theme-dropdown');
  dd.style.display = dd.style.display === 'none' ? 'block' : 'none';
}
function closeThemeDropdown() { document.getElementById('theme-dropdown').style.display = 'none'; }

// --- Init ---
document.addEventListener('DOMContentLoaded', () => {
  document.addEventListener('click', (e) => {
    if (!e.target.closest('.theme-toggle')) closeThemeDropdown();
  });
  loadAll();
  startPolling();
});
