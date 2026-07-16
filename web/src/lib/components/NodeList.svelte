<script>
  import { fetchNodes, formatBytes, formatPercent } from '../api.js';

  let nodes = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let expandedNode = $state(null);

  async function load() {
    loading = true;
    error = null;
    try {
      nodes = await fetchNodes();
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  load();
</script>

<div class="node-list">
  <div class="header">
    <h2>Nodes</h2>
    <button class="refresh" onclick={load}>↻ Refresh</button>
  </div>

  {#if loading}
    <p class="status">Loading...</p>
  {:else if error}
    <p class="status error">{error}</p>
  {:else if nodes.length === 0}
    <p class="status">No nodes found</p>
  {:else}
    {#each nodes as node}
      <div class="node-card">
        <div class="node-header" onclick={() => expandedNode = expandedNode === node.Name ? null : node.Name}>
          <h3>{node.Name}</h3>
          <div class="node-summary">
            <span class="badge">
              {formatBytes(node.TotalImageSize)} images
            </span>
            <span class="badge" class:warning={node.EphemeralStorage.AvailableBytes < node.EphemeralStorage.CapacityBytes * 0.2}>
              {formatPercent(node.EphemeralStorage.AllocatedBytes, node.EphemeralStorage.CapacityBytes)} used
            </span>
          </div>
        </div>

        <div class="storage-bar">
          <div
            class="storage-fill"
            class:critical={node.EphemeralStorage.AvailableBytes < node.EphemeralStorage.CapacityBytes * 0.1}
            style="width: {formatPercent(node.EphemeralStorage.AllocatedBytes, node.EphemeralStorage.CapacityBytes)}"
          ></div>
        </div>
        <div class="storage-labels">
          <span>{formatBytes(node.EphemeralStorage.AllocatedBytes)} used</span>
          <span>{formatBytes(node.EphemeralStorage.CapacityBytes)} total</span>
        </div>

        {#if expandedNode === node.Name}
          <div class="image-list">
            <h4>Container Images ({node.Images?.length || 0})</h4>
            <table>
              <thead>
                <tr>
                  <th>Image</th>
                  <th>Size</th>
                  <th>Status</th>
                </tr>
              </thead>
              <tbody>
                {#each (node.Images || []).sort((a, b) => b.SizeBytes - a.SizeBytes) as img}
                  <tr>
                    <td class="image-name">{img.Names?.[0] || 'unnamed'}</td>
                    <td>{formatBytes(img.SizeBytes)}</td>
                    <td>
                      <span class="status-badge" class:in-use={img.InUse}>
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
  {/if}
</div>

<style>
  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .refresh {
    background: #1d2125;
    border: 1px solid #2f3336;
    color: #e7e9ea;
    padding: 0.4rem 0.8rem;
    border-radius: 4px;
    cursor: pointer;
  }

  .refresh:hover { background: #2f3336; }

  .status { color: #71767b; }
  .status.error { color: #f4212e; }

  .node-card {
    background: #16181c;
    border: 1px solid #2f3336;
    border-radius: 8px;
    padding: 1rem;
    margin-bottom: 1rem;
  }

  .node-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    cursor: pointer;
  }

  .node-header h3 { margin: 0; }

  .node-summary { display: flex; gap: 0.5rem; }

  .badge {
    background: #1d2125;
    padding: 0.2rem 0.6rem;
    border-radius: 12px;
    font-size: 0.8rem;
  }

  .badge.warning { background: #f59e0b33; color: #f59e0b; }

  .storage-bar {
    height: 6px;
    background: #2f3336;
    border-radius: 3px;
    margin: 0.75rem 0 0.25rem;
    overflow: hidden;
  }

  .storage-fill {
    height: 100%;
    background: #1d9bf0;
    border-radius: 3px;
    transition: width 0.3s;
  }

  .storage-fill.critical { background: #f4212e; }

  .storage-labels {
    display: flex;
    justify-content: space-between;
    font-size: 0.75rem;
    color: #71767b;
  }

  .image-list {
    margin-top: 1rem;
    border-top: 1px solid #2f3336;
    padding-top: 1rem;
  }

  .image-list h4 { margin: 0 0 0.5rem; }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
  }

  th, td { padding: 0.4rem 0.5rem; text-align: left; }
  th { color: #71767b; border-bottom: 1px solid #2f3336; }
  td { border-bottom: 1px solid #1d2125; }

  .image-name {
    max-width: 400px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: monospace;
    font-size: 0.8rem;
  }

  .status-badge {
    padding: 0.1rem 0.4rem;
    border-radius: 4px;
    font-size: 0.75rem;
    background: #f4212e33;
    color: #f4212e;
  }

  .status-badge.in-use {
    background: #00ba7c33;
    color: #00ba7c;
  }
</style>
