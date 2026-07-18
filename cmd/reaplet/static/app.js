// ============================================================
// Reaplet — Vanilla JS Application (full API parity)
// ============================================================

const THEMES = ['dark', 'light', 'catppuccin', 'nord', 'dracula'];
const THEME_LABELS = { dark: '🌙 Dark', light: '☀️ Light', catppuccin: '🐱 Catppuccin', nord: '❄️ Nord', dracula: '🧛 Dracula' };
const REFRESH_MS = 30000;

let state = {
  nodes: [], gcEvents: [], recommendations: [], clusterSummary: null,
  alertConfig: null, alertHistory: [], cleanupConfig: null,
  dedup: [], warmList: null, upgradeCheck: [],
  loading: true, tab: 'nodes', autoRefresh: true, timer: null
};

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
async function api(path, opts) {
  const r = await fetch(`/api${path}`, opts);
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: `HTTP ${r.status}` }));
    throw new Error(err.error || `HTTP ${r.status}`);
  }
  return r.json();
}

async function loadAll() {
  state.loading = true;
  try {
    const [nodes, events, recs, summary] = await Promise.all([
      api('/nodes'), api('/gc-events'), api('/recommendations'), api('/cluster/summary')
    ]);
    state.nodes = nodes || [];
    state.gcEvents = events || [];
    state.recommendations = recs || [];
    state.clusterSummary = summary;
  } catch (e) {
    toast(`Failed to load: ${e.message}`, 'error');
  }
  state.loading = false;
  render();
}

async function loadAlerts() {
  try {
    const [cfg, hist] = await Promise.all([api('/alerts/config'), api('/alerts/history')]);
    state.alertConfig = cfg;
    state.alertHistory = hist || [];
  } catch (e) { toast(`Failed to load alerts: ${e.message}`, 'error'); }
  render();
}

async function loadCleanup() {
  try { state.cleanupConfig = await api('/cleanup/config'); } catch (e) { toast(e.message, 'error'); }
  render();
}

async function loadDedup() {
  try { state.dedup = await api('/dedup') || []; } catch (e) { toast(e.message, 'error'); }
  render();
}

async function loadWarmList() {
  try { state.warmList = await api('/warm-list'); } catch (e) { toast(e.message, 'error'); }
  render();
}

async function loadUpgradeCheck() {
  try { state.upgradeCheck = await api('/upgrade-check') || []; } catch (e) { toast(e.message, 'error'); }
  render();
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
  const tabs = [
    ['nodes', '🖥️', 'Nodes'], ['cluster', '📊', 'Cluster'],
    ['recommendations', '💡', 'Cleanup'], ['gc-events', '🗑️', 'GC Events'],
    ['alerts', '🔔', 'Alerts'], ['settings', '⚙️', 'Settings']
  ];
  return tabs.map(([id, icon, label]) =>
    `<button class="tab${state.tab === id ? ' active' : ''}" onclick="switchTab('${id}')"><span class="tab-icon">${icon}</span><span class="tab-label">${label}</span></button>`
  ).join('');
}

function switchTab(t) {
  state.tab = t;
  if (t === 'alerts') loadAlerts();
  else if (t === 'settings') loadCleanup();
  else if (t === 'cluster') { loadDedup(); loadUpgradeCheck(); }
  render();
}

function renderPanel() {
  if (state.loading && state.nodes.length === 0) return renderSkeleton();
  switch (state.tab) {
    case 'nodes': return renderNodes();
    case 'cluster': return renderCluster();
    case 'recommendations': return renderRecommendations();
    case 'gc-events': return renderGCEvents();
    case 'alerts': return renderAlerts();
    case 'settings': return renderSettings();
    default: return '';
  }
}

function renderSkeleton() {
  return Array(3).fill('<div class="card"><div class="skeleton" style="width:40%;height:1.2rem"></div><div class="skeleton" style="width:100%;height:6px;margin-top:1rem"></div><div class="skeleton" style="width:60%;height:0.8rem;margin-top:0.5rem"></div></div>').join('');
}

// ===== NODES TAB =====
function renderNodes() {
  if (!state.nodes.length) return '<div class="empty-state"><span class="icon">🖥️</span><p>No nodes found</p></div>';
  return state.nodes.map((n, i) => {
    const p = pct(n.EphemeralStorage?.AllocatedBytes || 0, n.EphemeralStorage?.CapacityBytes || 1);
    const level = p >= 90 ? 'critical' : p >= 75 ? 'warning' : '';
    return `<div class="card" id="node-${i}">
      <div class="card-header" onclick="toggleNode(${i})">
        <div class="node-identity"><span class="node-dot ${level}"></span><h3>${n.Name}</h3></div>
        <div class="badges">
          <span class="badge">📦 ${(n.Images || []).length}</span>
          <span class="badge">${formatBytes(n.TotalImageSize)}</span>
          <button class="btn-small" onclick="event.stopPropagation();loadNodeForecast('${n.Name}')">📈 Forecast</button>
          <button class="btn-small" onclick="event.stopPropagation();loadNodePods('${n.Name}')">🔍 Pods</button>
        </div>
      </div>
      <div class="progress-bar"><div class="progress-fill ${level}" style="width:${p}%"></div></div>
      <div class="storage-meta"><span>${formatBytes(n.EphemeralStorage?.AllocatedBytes || 0)} used</span><span class="storage-pct">${p}%</span><span>${formatBytes(n.EphemeralStorage?.CapacityBytes || 0)} total</span></div>
      <div class="node-detail-wrap" id="detail-${i}" style="display:none"></div>
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

async function loadNodeForecast(name) {
  try {
    const forecast = await api(`/nodes/${name}/forecast`);
    const msg = forecast.TrendBytesPerDay > 0
      ? `📈 Growing ${formatBytes(Math.abs(forecast.TrendBytesPerDay))}/day — warning in ~${Math.round(forecast.ProjectedDaysToWarning)}d, critical in ~${Math.round(forecast.ProjectedDaysToCritical)}d`
      : forecast.TrendBytesPerDay < 0
        ? `📉 Shrinking ${formatBytes(Math.abs(forecast.TrendBytesPerDay))}/day — usage decreasing`
        : `➡️ Stable — no significant trend`;
    toast(msg);
  } catch (e) { toast(`Forecast: ${e.message}`, 'error'); }
}

async function loadNodePods(name) {
  try {
    const pods = await api(`/nodes/${name}/pods`);
    if (!pods || !pods.length) { toast(`No pods with ephemeral usage on ${name}`); return; }
    const sorted = pods.sort((a, b) => b.ephemeralUsageBytes - a.ephemeralUsageBytes);
    const rows = sorted.slice(0, 15).map(p =>
      `<tr><td>${p.namespace}</td><td><code>${p.podName}</code></td><td>${formatBytes(p.ephemeralUsageBytes)}</td><td>${p.containerCount}</td><td><button class="btn-remove btn-xs" onclick="evictPod('${p.podName}','${p.namespace}','${name}')">Evict</button></td></tr>`
    ).join('');
    showModal('Pod Storage Breakdown — ' + name,
      `<table class="image-table"><thead><tr><th>Namespace</th><th>Pod</th><th>Ephemeral</th><th>Containers</th><th></th></tr></thead><tbody>${rows}</tbody></table>
      <p class="text-muted" style="margin-top:0.5rem">⚠️ Eviction is graceful but disruptive — pod will be rescheduled.</p>`,
      'Close', () => {});
  } catch (e) { toast(`Pods: ${e.message}`, 'error'); }
}

async function evictPod(podName, namespace, nodeName) {
  showModal('Evict Pod', `<p>Evict <code>${podName}</code> from <strong>${nodeName}</strong>?</p><p class="modal-detail">The pod will be gracefully terminated and rescheduled by its controller.</p>`,
    'Evict', async () => {
      try {
        const result = await api('/evict', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ PodName: podName, Namespace: namespace, NodeName: nodeName, Reason: 'storage pressure' }) });
        if (result.Success) toast(`Evicted ${podName}`);
        else toast(`Eviction failed: ${result.Error}`, 'error');
      } catch (e) { toast(e.message, 'error'); }
    });
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

// ===== CLUSTER TAB =====
function renderCluster() {
  let html = '';

  // Cluster summary
  const s = state.clusterSummary;
  if (s) {
    html += `<div class="section-header"><h2>Cluster Overview</h2></div>
    <div class="stats-grid">
      <div class="stat-card"><span class="stat-number">${s.totalNodes}</span><span class="stat-label">Nodes</span></div>
      <div class="stat-card"><span class="stat-number">${formatBytes(s.totalCapacity)}</span><span class="stat-label">Total Capacity</span></div>
      <div class="stat-card"><span class="stat-number">${formatBytes(s.totalAllocated)}</span><span class="stat-label">Used</span></div>
      <div class="stat-card"><span class="stat-number">${formatBytes(s.reclaimableBytes)}</span><span class="stat-label">Reclaimable</span></div>
      <div class="stat-card"><span class="stat-number">${s.totalImages}</span><span class="stat-label">Total Images</span></div>
      <div class="stat-card"><span class="stat-number">${s.totalUnused}</span><span class="stat-label">Unused Images</span></div>
    </div>`;

    // Node comparison bars
    if (s.nodes && s.nodes.length) {
      const bars = s.nodes.map(n => {
        const lvl = n.usagePct >= 90 ? 'critical' : n.usagePct >= 75 ? 'warning' : '';
        return `<div class="comparison-row">
          <span class="comp-name">${n.name}</span>
          <div class="progress-bar flex-1"><div class="progress-fill ${lvl}" style="width:${n.usagePct.toFixed(1)}%"></div></div>
          <span class="comp-pct">${n.usagePct.toFixed(1)}%</span>
          <span class="comp-reclaimable">${formatBytes(n.reclaimable)} free</span>
        </div>`;
      }).join('');
      html += `<div class="section-header"><h2>Node Comparison</h2></div><div class="card">${bars}</div>`;
    }
  }

  // Upgrade check
  if (state.upgradeCheck.length) {
    const rows = state.upgradeCheck.map(u => {
      const icon = u.safe ? '✅' : '❌';
      return `<tr><td>${icon} ${u.nodeName}</td><td>${formatBytes(u.availableBytes)}</td><td>${formatBytes(u.estimatedNeededBytes)}</td><td>${u.message}</td></tr>`;
    }).join('');
    html += `<div class="section-header"><h2>🔄 Talos Upgrade Readiness</h2></div>
    <div class="card"><table class="image-table"><thead><tr><th>Node</th><th>Available</th><th>Needed</th><th>Status</th></tr></thead><tbody>${rows}</tbody></table></div>`;
  }

  // Image deduplication
  if (state.dedup.length) {
    const totalWasted = state.dedup.reduce((s, d) => s + d.wastedBytes, 0);
    const rows = state.dedup.map(d =>
      `<tr><td><code>${d.names[0]}</code></td><td>${d.names.length} tags</td><td>${d.nodes.length} nodes</td><td>${formatBytes(d.sizeBytes)}</td><td class="text-warning">${formatBytes(d.wastedBytes)}</td></tr>`
    ).join('');
    html += `<div class="section-header"><h2>🔁 Image Deduplication</h2><span class="badge badge-warning">${formatBytes(totalWasted)} wasted</span></div>
    <div class="card"><table class="image-table"><thead><tr><th>Image</th><th>Tags</th><th>Nodes</th><th>Size</th><th>Wasted</th></tr></thead><tbody>${rows}</tbody></table></div>`;
  } else {
    html += `<div class="section-header"><h2>🔁 Image Deduplication</h2></div><div class="empty-state"><span class="icon">✨</span><p>No duplicate images found</p></div>`;
  }

  return html || '<div class="empty-state"><span class="icon">📊</span><p>Loading cluster data...</p></div>';
}

// ===== GC EVENTS TAB =====
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

// ===== RECOMMENDATIONS TAB =====
function renderRecommendations() {
  const recs = state.recommendations;
  if (!recs.length) return '<div class="empty-state"><span class="icon">✨</span><p>All images are in use — nothing to clean up!</p></div>';
  const total = recs.reduce((s, r) => s + r.SavingsBytes, 0);
  const banner = `<div class="summary-banner">
    <div class="summary-stat"><span class="stat-number">${recs.length}</span><span class="stat-label">unused images</span></div>
    <div class="summary-divider"></div>
    <div class="summary-stat"><span class="stat-number">${formatBytes(total)}</span><span class="stat-label">reclaimable</span></div>
    <div class="summary-divider"></div>
    <button class="btn-remove-all" onclick="handleRemoveAll()">🗑️ Remove All Unused</button>
  </div>`;
  const cards = recs.map((r, i) => {
    const name = r.Image.Names?.[0] || 'unnamed';
    const stale = r.UnusedDays > 0 ? `<span class="rec-stale">⏱️ ${r.UnusedDays}d unused</span>` : '';
    return `<div class="rec-card" id="rec-${i}">
      <div class="rec-info"><code class="rec-image">${name}</code>
        <div class="rec-meta"><span>🖥️ ${r.NodeName}</span><span>📦 ${formatBytes(r.SavingsBytes)}</span><span>💡 ${r.Reason}</span>${stale}</div>
      </div>
      <button class="btn-remove" onclick="handleRemove(${i})">Remove</button>
    </div>`;
  }).join('');
  return banner + cards;
}

async function handleRemove(i) {
  const rec = state.recommendations[i];
  const name = rec.Image.Names?.[0] || 'unnamed';
  showModal('Remove Image',
    `<p>Remove <code>${name}</code> from <strong>${rec.NodeName}</strong>?</p><p class="modal-detail">This will free ${formatBytes(rec.SavingsBytes)}. The image can be re-pulled if needed.</p>`,
    'Remove', async () => {
      try {
        const result = await api('/remove-image', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ NodeName: rec.NodeName, ImageRef: name }) });
        if (result.Success) { toast(`Removed ${name} — freed ${formatBytes(result.FreedBytes || rec.SavingsBytes)}`); loadAll(); }
        else toast(`Failed: ${result.Error}`, 'error');
      } catch (e) { toast(e.message, 'error'); }
    });
}

async function handleRemoveAll() {
  const recs = state.recommendations;
  const total = recs.reduce((s, r) => s + r.SavingsBytes, 0);
  showModal('Remove All Unused Images',
    `<p>Remove <strong>${recs.length}</strong> unused images across all nodes?</p><p class="modal-detail">This will free approximately ${formatBytes(total)}.</p>`,
    `Remove All (${recs.length})`, async () => {
      let ok = 0, fail = 0, freed = 0;
      for (const rec of recs) {
        const name = rec.Image.Names?.[0] || 'unnamed';
        try {
          const r = await api('/remove-image', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ NodeName: rec.NodeName, ImageRef: name }) });
          if (r.Success) { ok++; freed += r.FreedBytes || rec.SavingsBytes; } else fail++;
        } catch { fail++; }
      }
      if (ok) toast(`Removed ${ok} images — freed ${formatBytes(freed)}`);
      if (fail) toast(`${fail} removals failed`, 'error');
      loadAll();
    });
}

// ===== ALERTS TAB =====
function renderAlerts() {
  const cfg = state.alertConfig;
  if (!cfg) return '<div class="empty-state"><span class="icon">🔔</span><p>Loading alerts configuration...</p></div>';

  const history = state.alertHistory.map(ev => {
    const icon = ev.level === 'critical' ? '🔴' : ev.level === 'warning' ? '🟡' : '🟢';
    return `<div class="alert-event"><span>${icon}</span><span class="alert-node">${ev.nodeName}</span><span class="alert-msg">${ev.message}</span><span class="alert-time">${timeAgo(ev.timestamp)}</span></div>`;
  }).join('') || '<p class="text-muted">No alerts fired yet.</p>';

  return `
  <div class="section-header"><h2>⚙️ Thresholds</h2></div>
  <div class="card form-card">
    <div class="form-row">
      <label>Warning threshold</label>
      <div class="input-group"><input type="range" id="alert-warning" min="50" max="99" value="${cfg.warningPct}" oninput="document.getElementById('warn-val').textContent=this.value+'%'"><span id="warn-val">${cfg.warningPct}%</span></div>
    </div>
    <div class="form-row">
      <label>Critical threshold</label>
      <div class="input-group"><input type="range" id="alert-critical" min="50" max="99" value="${cfg.criticalPct}" oninput="document.getElementById('crit-val').textContent=this.value+'%'"><span id="crit-val">${cfg.criticalPct}%</span></div>
    </div>
    <div class="form-row">
      <label>Cooldown (min)</label>
      <input type="number" id="alert-cooldown" value="${cfg.cooldownMin}" min="1" max="120" class="input-sm">
    </div>
  </div>

  <div class="section-header"><h2>📡 Discord</h2></div>
  <div class="card form-card">
    <div class="form-row">
      <label>Webhook URL</label>
      <input type="text" id="discord-url" value="${cfg.discord?.webhookUrl || ''}" placeholder="https://discord.com/api/webhooks/..." class="input-full">
    </div>
    <div class="form-row">
      <label><input type="checkbox" id="discord-enabled" ${cfg.discord?.enabled ? 'checked' : ''}> Enabled</label>
    </div>
  </div>

  <div class="section-header"><h2>📱 Pushover</h2></div>
  <div class="card form-card">
    <div class="form-row">
      <label>App Token</label>
      <input type="text" id="pushover-token" value="${cfg.pushover?.appToken || ''}" placeholder="App token" class="input-full">
    </div>
    <div class="form-row">
      <label>User Key</label>
      <input type="text" id="pushover-user" value="${cfg.pushover?.userKey || ''}" placeholder="User key" class="input-full">
    </div>
    <div class="form-row">
      <label><input type="checkbox" id="pushover-enabled" ${cfg.pushover?.enabled ? 'checked' : ''}> Enabled</label>
    </div>
  </div>

  <div class="form-actions">
    <button class="btn-primary" onclick="saveAlertConfig()">💾 Save Configuration</button>
    <button class="btn-secondary" onclick="testAlerts()">🧪 Send Test Alert</button>
  </div>

  <div class="section-header"><h2>📜 Alert History</h2></div>
  <div class="card">${history}</div>`;
}

async function saveAlertConfig() {
  const cfg = {
    warningPct: parseInt(document.getElementById('alert-warning').value),
    criticalPct: parseInt(document.getElementById('alert-critical').value),
    cooldownMin: parseInt(document.getElementById('alert-cooldown').value),
    discord: {
      enabled: document.getElementById('discord-enabled').checked,
      webhookUrl: document.getElementById('discord-url').value
    },
    pushover: {
      enabled: document.getElementById('pushover-enabled').checked,
      appToken: document.getElementById('pushover-token').value,
      userKey: document.getElementById('pushover-user').value
    }
  };
  try {
    await api('/alerts/config', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(cfg) });
    toast('Alert configuration saved');
    loadAlerts();
  } catch (e) { toast(e.message, 'error'); }
}

async function testAlerts() {
  try {
    await api('/alerts/test', { method: 'POST' });
    toast('Test alert sent to all enabled channels');
  } catch (e) { toast(`Test failed: ${e.message}`, 'error'); }
}

// ===== SETTINGS TAB (Cleanup + Warm List) =====
function renderSettings() {
  let html = '';

  // Cleanup config
  const cfg = state.cleanupConfig;
  if (cfg) {
    html += `<div class="section-header"><h2>🧹 Automatic Cleanup</h2></div>
    <div class="card form-card">
      <div class="form-row"><label><input type="checkbox" id="cleanup-enabled" ${cfg.enabled ? 'checked' : ''}> Enable automatic cleanup</label></div>
      <div class="form-row"><label><input type="checkbox" id="cleanup-dryrun" ${cfg.dryRun ? 'checked' : ''}> Dry run (log only, don't remove)</label></div>
      <div class="form-row"><label>Interval (hours)</label><input type="number" id="cleanup-interval" value="${cfg.intervalHours}" min="1" max="168" class="input-sm"></div>
      <div class="form-row"><label>Max image age (days)</label><input type="number" id="cleanup-maxage" value="${cfg.maxAgeDays}" min="1" max="365" class="input-sm"></div>
      <div class="form-row"><label>Max image size (MB)</label><input type="number" id="cleanup-maxsize" value="${cfg.maxSizeMB}" min="50" max="10000" class="input-sm"></div>
      <div class="form-row"><label>Max removals per cycle</label><input type="number" id="cleanup-maxper" value="${cfg.maxPerCycle}" min="1" max="50" class="input-sm"></div>
      <div class="form-row"><label>Keep patterns (one per line)</label><textarea id="cleanup-patterns" class="input-full" rows="3">${(cfg.keepPatterns || []).join('\n')}</textarea></div>
      <div class="form-actions">
        <button class="btn-primary" onclick="saveCleanupConfig()">💾 Save</button>
        <button class="btn-secondary" onclick="runCleanup()">▶️ Run Now</button>
      </div>
    </div>`;
  }

  // Warm list
  html += `<div class="section-header"><h2>🔥 Image Warm List (Pre-Pull)</h2>
    <button class="btn-small" onclick="addWarmListPrompt()">+ Add Image</button>
  </div>`;

  if (state.warmList) {
    const wl = state.warmList;
    if (wl.entries && wl.entries.length) {
      const rows = wl.entries.map(e => {
        const missing = wl.missingOnNodes?.[e.imageRef] || [];
        const status = missing.length ? `<span class="pill pill-danger">Missing on ${missing.length} node(s)</span>` : '<span class="pill pill-success">Present</span>';
        return `<tr><td><code>${e.imageRef}</code></td><td>${status}</td><td>${timeAgo(e.addedAt)}</td><td><button class="btn-remove btn-xs" onclick="deleteWarmEntry(${e.id})">✕</button></td></tr>`;
      }).join('');
      html += `<div class="card"><table class="image-table"><thead><tr><th>Image</th><th>Status</th><th>Added</th><th></th></tr></thead><tbody>${rows}</tbody></table></div>`;
    } else {
      html += '<div class="card"><p class="text-muted">No images in warm list. Add images that should always be pre-pulled on all nodes.</p></div>';
    }
  } else {
    loadWarmList();
    html += '<div class="card"><p class="text-muted">Loading warm list...</p></div>';
  }

  return html;
}

async function saveCleanupConfig() {
  const cfg = {
    enabled: document.getElementById('cleanup-enabled').checked,
    dryRun: document.getElementById('cleanup-dryrun').checked,
    intervalHours: parseInt(document.getElementById('cleanup-interval').value),
    maxAgeDays: parseInt(document.getElementById('cleanup-maxage').value),
    maxSizeMB: parseInt(document.getElementById('cleanup-maxsize').value),
    maxPerCycle: parseInt(document.getElementById('cleanup-maxper').value),
    keepPatterns: document.getElementById('cleanup-patterns').value.split('\n').map(s => s.trim()).filter(Boolean)
  };
  try {
    await api('/cleanup/config', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(cfg) });
    toast('Cleanup configuration saved');
  } catch (e) { toast(e.message, 'error'); }
}

async function runCleanup() {
  showModal('Run Cleanup', '<p>Execute cleanup now with current configuration?</p>', 'Run', async () => {
    try {
      const result = await api('/cleanup/run', { method: 'POST' });
      const removed = result.removed?.length || 0;
      const skipped = result.skipped?.length || 0;
      toast(`Cleanup complete: ${removed} removed, ${skipped} skipped${result.dryRun ? ' (dry run)' : ''}`);
    } catch (e) { toast(e.message, 'error'); }
  });
}

function addWarmListPrompt() {
  showModal('Add to Warm List',
    '<p>Image reference to keep pre-pulled on all nodes:</p><input type="text" id="warm-input" placeholder="docker.io/library/nginx:latest" class="input-full" style="margin-top:0.5rem">',
    'Add', async () => {
      const imageRef = document.getElementById('warm-input')?.value?.trim();
      if (!imageRef) { toast('Image reference required', 'error'); return; }
      try {
        await api('/warm-list', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ imageRef }) });
        toast(`Added ${imageRef} to warm list`);
        loadWarmList();
      } catch (e) { toast(e.message, 'error'); }
    });
}

async function deleteWarmEntry(id) {
  try {
    await api(`/warm-list/${id}`, { method: 'DELETE' });
    toast('Removed from warm list');
    loadWarmList();
  } catch (e) { toast(e.message, 'error'); }
}

// --- Modal ---
let modalCallback = null;
function showModal(title, bodyHtml, confirmText, onConfirm) {
  document.getElementById('modal-title').textContent = title;
  document.getElementById('modal-body').innerHTML = bodyHtml;
  document.getElementById('modal-confirm').textContent = confirmText;
  modalCallback = onConfirm;
  document.getElementById('modal').classList.add('open');
  document.getElementById('modal-confirm').onclick = async () => {
    const cb = modalCallback;
    hideModal();
    if (cb) await cb();
  };
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
