<script>
  import { fetchRecommendations, removeImage, formatBytes } from '../api.js';

  let recommendations = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let actionResult = $state(null);

  async function load() {
    loading = true;
    error = null;
    try {
      recommendations = await fetchRecommendations();
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function handleRemove(rec) {
    if (!confirm(`Remove image ${rec.Image.Names?.[0]} from ${rec.NodeName}?\n\nThis will free ${formatBytes(rec.SavingsBytes)}.`)) {
      return;
    }
    actionResult = null;
    try {
      const result = await removeImage(rec.Image.Names[0], rec.NodeName);
      actionResult = result;
      if (result.Success) {
        await load(); // refresh
      }
    } catch (e) {
      actionResult = { Success: false, Error: e.message };
    }
  }

  load();
</script>

<div class="recommendations">
  <div class="header">
    <h2>Image Recommendations</h2>
    <button class="refresh" onclick={load}>↻ Refresh</button>
  </div>

  {#if actionResult}
    <div class="result" class:success={actionResult.Success} class:failure={!actionResult.Success}>
      {#if actionResult.Success}
        ✓ Removed — freed {formatBytes(actionResult.FreedBytes)}
      {:else}
        ✗ Failed: {actionResult.Error}
      {/if}
    </div>
  {/if}

  {#if loading}
    <p class="status">Analyzing images...</p>
  {:else if error}
    <p class="status error">{error}</p>
  {:else if recommendations.length === 0}
    <p class="status">No recommendations — all images are in use 🎉</p>
  {:else}
    <p class="summary">
      {recommendations.length} images can be removed, saving {formatBytes(recommendations.reduce((sum, r) => sum + r.SavingsBytes, 0))}
    </p>
    <div class="rec-list">
      {#each recommendations as rec}
        <div class="rec-card">
          <div class="rec-info">
            <span class="image-name">{rec.Image.Names?.[0] || 'unnamed'}</span>
            <span class="rec-meta">
              {rec.NodeName} · {formatBytes(rec.SavingsBytes)} · {rec.Reason}
            </span>
          </div>
          <button class="remove-btn" onclick={() => handleRemove(rec)}>
            Remove
          </button>
        </div>
      {/each}
    </div>
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

  .summary { color: #f59e0b; margin-bottom: 1rem; }

  .result {
    padding: 0.5rem 1rem;
    border-radius: 4px;
    margin-bottom: 1rem;
  }

  .result.success { background: #00ba7c22; color: #00ba7c; }
  .result.failure { background: #f4212e22; color: #f4212e; }

  .rec-card {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: #16181c;
    border: 1px solid #2f3336;
    border-radius: 8px;
    padding: 0.75rem 1rem;
    margin-bottom: 0.5rem;
  }

  .rec-info { display: flex; flex-direction: column; gap: 0.25rem; overflow: hidden; }

  .image-name {
    font-family: monospace;
    font-size: 0.85rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .rec-meta { font-size: 0.75rem; color: #71767b; }

  .remove-btn {
    background: #f4212e22;
    border: 1px solid #f4212e44;
    color: #f4212e;
    padding: 0.3rem 0.8rem;
    border-radius: 4px;
    cursor: pointer;
    white-space: nowrap;
  }

  .remove-btn:hover { background: #f4212e44; }
</style>
