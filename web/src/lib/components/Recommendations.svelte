<script>
  import { dataStore } from '../stores/data.svelte.js';
  import { toastStore } from '../stores/toast.svelte.js';
  import { removeImage, formatBytes } from '../api.js';

  let removing = $state(null); // imageRef currently being removed

  async function handleRemove(rec) {
    const imageName = rec.Image.Names?.[0] || rec.Image.Names?.[1] || 'unnamed';
    if (!confirm(`Remove image ${imageName} from ${rec.NodeName}?\n\nThis will free ${formatBytes(rec.SavingsBytes)}.`)) {
      return;
    }

    removing = imageName;
    try {
      const result = await removeImage(imageName, rec.NodeName);
      if (result.Success) {
        toastStore.success(`Removed ${imageName} — freed ${formatBytes(result.FreedBytes)}`);
        await dataStore.refreshRecommendations();
        await dataStore.refreshNodes();
      } else {
        toastStore.error(`Failed: ${result.Error}`);
      }
    } catch (e) {
      toastStore.error(`Error: ${e.message}`);
    } finally {
      removing = null;
    }
  }

  function totalSavings(recs) {
    return recs.reduce((sum, r) => sum + r.SavingsBytes, 0);
  }
</script>

<div class="recommendations">
  {#if dataStore.loading.recommendations && dataStore.recommendations.length === 0}
    <div class="skeleton-list">
      {#each [1,2,3] as _}
        <div class="skeleton-card">
          <div class="skeleton" style="width: 60%; height: 1rem;"></div>
          <div class="skeleton" style="width: 40%; height: 0.8rem; margin-top: 0.5rem;"></div>
        </div>
      {/each}
    </div>
  {:else if dataStore.errors.recommendations}
    <div class="error-state">
      <span class="error-icon">⚠️</span>
      <p class="error-msg">{dataStore.errors.recommendations}</p>
      <button class="retry-btn" onclick={() => dataStore.refreshRecommendations()}>Retry</button>
    </div>
  {:else if dataStore.recommendations.length === 0}
    <div class="empty-state">
      <span class="empty-icon">✨</span>
      <p>All images are in use — nothing to clean up!</p>
    </div>
  {:else}
    <div class="summary-banner">
      <div class="summary-stat">
        <span class="stat-number">{dataStore.recommendations.length}</span>
        <span class="stat-label">unused images</span>
      </div>
      <div class="summary-divider"></div>
      <div class="summary-stat">
        <span class="stat-number">{formatBytes(totalSavings(dataStore.recommendations))}</span>
        <span class="stat-label">reclaimable</span>
      </div>
    </div>

    <div class="rec-list">
      {#each dataStore.recommendations as rec, i}
        <div class="rec-card" style="animation-delay: {i * 40}ms">
          <div class="rec-info">
            <code class="rec-image">{rec.Image.Names?.[0] || 'unnamed'}</code>
            <div class="rec-meta">
              <span class="meta-node">🖥️ {rec.NodeName}</span>
              <span class="meta-size">📦 {formatBytes(rec.SavingsBytes)}</span>
              <span class="meta-reason">💡 {rec.Reason}</span>
            </div>
          </div>
          <button
            class="remove-btn"
            onclick={() => handleRemove(rec)}
            disabled={removing === (rec.Image.Names?.[0] || '')}
          >
            {#if removing === (rec.Image.Names?.[0] || '')}
              <span class="btn-spinner">↻</span>
            {:else}
              Remove
            {/if}
          </button>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .skeleton-list { display: grid; gap: var(--space-md); }

  .skeleton-card {
    background: var(--bg-surface);
    border: 1px solid var(--border-muted);
    border-radius: var(--radius-md);
    padding: var(--space-md);
  }

  .error-state, .empty-state {
    text-align: center;
    padding: var(--space-2xl);
    color: var(--text-muted);
  }

  .error-icon, .empty-icon { font-size: 2.5rem; display: block; margin-bottom: var(--space-md); }
  .error-msg { color: var(--danger); }

  .retry-btn {
    background: var(--danger-muted);
    border: 1px solid var(--danger);
    color: var(--danger);
    padding: var(--space-sm) var(--space-md);
    border-radius: var(--radius-md);
    cursor: pointer;
  }

  /* Summary */
  .summary-banner {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-lg);
    padding: var(--space-md) var(--space-lg);
    background: var(--warning-muted);
    border: 1px solid var(--warning);
    border-radius: var(--radius-lg);
    margin-bottom: var(--space-lg);
  }

  .summary-stat {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
  }

  .stat-number {
    font-size: 1.3rem;
    font-weight: 700;
    color: var(--warning);
  }

  .stat-label {
    font-size: 0.72rem;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .summary-divider {
    width: 1px;
    height: 2rem;
    background: var(--border-default);
  }

  /* List */
  .rec-list {
    display: grid;
    gap: var(--space-sm);
  }

  .rec-card {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-md);
    background: var(--bg-surface);
    border: 1px solid var(--border-muted);
    border-radius: var(--radius-md);
    padding: var(--space-sm) var(--space-md);
    animation: fadeIn var(--transition-slow) both;
    transition: border-color var(--transition-fast);
  }

  .rec-card:hover {
    border-color: var(--border-default);
  }

  .rec-info {
    min-width: 0;
    flex: 1;
  }

  .rec-image {
    font-family: var(--font-mono);
    font-size: 0.82rem;
    color: var(--text-primary);
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .rec-meta {
    display: flex;
    gap: var(--space-md);
    margin-top: var(--space-xs);
    font-size: 0.72rem;
    color: var(--text-muted);
    flex-wrap: wrap;
  }

  .remove-btn {
    flex-shrink: 0;
    background: var(--danger-muted);
    border: 1px solid var(--danger);
    color: var(--danger);
    padding: var(--space-xs) var(--space-md);
    border-radius: var(--radius-md);
    cursor: pointer;
    font-size: 0.8rem;
    font-weight: 600;
    transition: all var(--transition-fast);
    min-width: 5rem;
    text-align: center;
  }

  .remove-btn:hover:not(:disabled) {
    background: var(--danger);
    color: var(--text-inverse);
  }

  .remove-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .btn-spinner {
    display: inline-block;
    animation: spin 1s linear infinite;
  }

  @media (max-width: 640px) {
    .rec-card { flex-direction: column; align-items: stretch; }
    .remove-btn { align-self: flex-end; }
    .summary-banner { gap: var(--space-md); }
  }
</style>
