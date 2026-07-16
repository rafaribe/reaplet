<script>
  import { dataStore } from '../stores/data.svelte.js';
  import { formatBytes, formatPercent } from '../api.js';

  let expandedNode = $state(null);

  function storagePercent(node) {
    if (!node.EphemeralStorage?.CapacityBytes) return 0;
    return (node.EphemeralStorage.AllocatedBytes / node.EphemeralStorage.CapacityBytes) * 100;
  }

  function storageLevel(node) {
    const pct = storagePercent(node);
    if (pct >= 90) return 'critical';
    if (pct >= 75) return 'warning';
    return 'normal';
  }
</script>

<div class="node-list">
  {#if dataStore.loading.nodes && dataStore.nodes.length === 0}
    <div class="skeleton-grid">
      {#each [1,2,3] as _}
        <div class="skeleton-card">
          <div class="skeleton" style="width: 40%; height: 1.2rem;"></div>
          <div class="skeleton" style="width: 100%; height: 8px; margin-top: 1rem;"></div>
          <div class="skeleton" style="width: 60%; height: 0.8rem; margin-top: 0.5rem;"></div>
        </div>
      {/each}
    </div>
  {:else if dataStore.errors.nodes}
    <div class="error-state">
      <span class="error-icon">⚠️</span>
      <p class="error-msg">{dataStore.errors.nodes}</p>
      <button class="retry-btn" onclick={() => dataStore.refreshNodes()}>Retry</button>
    </div>
  {:else if dataStore.nodes.length === 0}
    <div class="empty-state">
      <span class="empty-icon">🖥️</span>
      <p>No nodes found</p>
    </div>
  {:else}
    <div class="node-grid">
      {#each dataStore.nodes as node, i}
        <div
          class="node-card"
          class:expanded={expandedNode === node.Name}
          style="animation-delay: {i * 50}ms"
        >
          <button
            class="node-header"
            onclick={() => expandedNode = expandedNode === node.Name ? null : node.Name}
            aria-expanded={expandedNode === node.Name}
          >
            <div class="node-identity">
              <span class="node-indicator" class:critical={storageLevel(node) === 'critical'} class:warning={storageLevel(node) === 'warning'}></span>
              <h3 class="node-name">{node.Name}</h3>
            </div>
            <div class="node-badges">
              <span class="badge badge-images">
                📦 {node.Images?.length || 0}
              </span>
              <span class="badge badge-size">
                {formatBytes(node.TotalImageSize)}
              </span>
            </div>
          </button>

          <div class="storage-section">
            <div class="progress-bar">
              <div
                class="progress-fill progress-{storageLevel(node)}"
                style="width: {storagePercent(node)}%"
              ></div>
            </div>
            <div class="storage-meta">
              <span>{formatBytes(node.EphemeralStorage?.AllocatedBytes || 0)} used</span>
              <span class="storage-pct">{storagePercent(node).toFixed(1)}%</span>
              <span>{formatBytes(node.EphemeralStorage?.CapacityBytes || 0)} total</span>
            </div>
          </div>

          {#if expandedNode === node.Name}
            <div class="image-table-wrap">
              <table class="image-table">
                <thead>
                  <tr>
                    <th>Image</th>
                    <th>Size</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {#each (node.Images || []).sort((a, b) => b.SizeBytes - a.SizeBytes) as img, j}
                    <tr style="animation-delay: {j * 30}ms" class="animate-in">
                      <td class="col-image">
                        <code>{img.Names?.[0] || 'unnamed'}</code>
                      </td>
                      <td class="col-size">{formatBytes(img.SizeBytes)}</td>
                      <td>
                        <span class="status-pill" class:in-use={img.InUse}>
                          {img.InUse ? 'In Use' : 'Unused'}
                        </span>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .node-grid {
    display: grid;
    gap: var(--space-md);
  }

  .skeleton-grid {
    display: grid;
    gap: var(--space-md);
  }

  .skeleton-card {
    background: var(--bg-surface);
    border: 1px solid var(--border-muted);
    border-radius: var(--radius-lg);
    padding: var(--space-lg);
  }

  .error-state, .empty-state {
    text-align: center;
    padding: var(--space-2xl);
    color: var(--text-muted);
  }

  .error-icon, .empty-icon {
    font-size: 2.5rem;
    display: block;
    margin-bottom: var(--space-md);
  }

  .error-msg { color: var(--danger); margin: var(--space-sm) 0; }

  .retry-btn {
    background: var(--danger-muted);
    border: 1px solid var(--danger);
    color: var(--danger);
    padding: var(--space-sm) var(--space-md);
    border-radius: var(--radius-md);
    cursor: pointer;
    font-weight: 500;
    transition: all var(--transition-fast);
  }

  .retry-btn:hover {
    background: var(--danger);
    color: var(--text-inverse);
  }

  /* Card */
  .node-card {
    background: var(--bg-surface);
    border: 1px solid var(--border-muted);
    border-radius: var(--radius-lg);
    padding: var(--space-md) var(--space-lg);
    animation: fadeIn var(--transition-slow) both;
    transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
  }

  .node-card:hover {
    border-color: var(--border-default);
  }

  .node-card.expanded {
    border-color: var(--accent);
    box-shadow: var(--shadow-glow);
  }

  .node-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    width: 100%;
    background: none;
    border: none;
    cursor: pointer;
    padding: 0;
    color: inherit;
    text-align: left;
  }

  .node-identity {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
  }

  .node-indicator {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--success);
    flex-shrink: 0;
  }

  .node-indicator.warning { background: var(--warning); }
  .node-indicator.critical { background: var(--danger); animation: pulse 1.5s infinite; }

  .node-name {
    margin: 0;
    font-size: 1rem;
    font-weight: 600;
  }

  .node-badges {
    display: flex;
    gap: var(--space-sm);
  }

  .badge {
    font-size: 0.75rem;
    padding: 2px 8px;
    border-radius: var(--radius-full);
    background: var(--bg-overlay);
    color: var(--text-secondary);
    font-weight: 500;
  }

  /* Progress */
  .storage-section {
    margin-top: var(--space-md);
  }

  .progress-bar {
    height: 6px;
    background: var(--progress-bg);
    border-radius: var(--radius-full);
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    border-radius: var(--radius-full);
    transition: width var(--transition-slow);
  }

  .progress-normal { background: var(--accent); }
  .progress-warning { background: var(--warning); }
  .progress-critical { background: var(--danger); }

  .storage-meta {
    display: flex;
    justify-content: space-between;
    font-size: 0.72rem;
    color: var(--text-muted);
    margin-top: var(--space-xs);
  }

  .storage-pct {
    font-weight: 600;
    color: var(--text-secondary);
  }

  /* Image table */
  .image-table-wrap {
    margin-top: var(--space-md);
    padding-top: var(--space-md);
    border-top: 1px solid var(--border-muted);
    overflow-x: auto;
  }

  .image-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.82rem;
  }

  .image-table th {
    text-align: left;
    padding: var(--space-sm);
    color: var(--text-muted);
    font-weight: 500;
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    border-bottom: 1px solid var(--border-muted);
  }

  .image-table td {
    padding: var(--space-sm);
    border-bottom: 1px solid var(--border-muted);
    vertical-align: middle;
  }

  .image-table tbody tr:last-child td {
    border-bottom: none;
  }

  .col-image code {
    font-family: var(--font-mono);
    font-size: 0.78rem;
    max-width: 400px;
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-secondary);
  }

  .col-size {
    white-space: nowrap;
    color: var(--text-secondary);
  }

  .status-pill {
    display: inline-block;
    padding: 2px 8px;
    border-radius: var(--radius-full);
    font-size: 0.72rem;
    font-weight: 600;
    background: var(--danger-muted);
    color: var(--danger);
  }

  .status-pill.in-use {
    background: var(--success-muted);
    color: var(--success);
  }

  @media (max-width: 640px) {
    .node-badges { display: none; }
    .col-image code { max-width: 180px; }
  }
</style>
