<script>
  import { dataStore } from '../stores/data.svelte.js';

  function formatTime(ts) {
    if (!ts) return 'Unknown';
    const d = new Date(ts);
    return d.toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
  }

  function reasonType(reason) {
    if (reason.includes('Failed') || reason.includes('Pressure')) return 'critical';
    if (reason.includes('Succeeded') || reason.includes('NoDisk')) return 'success';
    return 'neutral';
  }

  function reasonIcon(reason) {
    if (reason.includes('Failed')) return '❌';
    if (reason.includes('Pressure')) return '⚠️';
    if (reason.includes('Succeeded')) return '✅';
    if (reason.includes('NoDisk')) return '🎉';
    return '📋';
  }
</script>

<div class="gc-events">
  {#if dataStore.loading.gcEvents && dataStore.gcEvents.length === 0}
    <div class="skeleton-list">
      {#each [1,2,3,4] as _}
        <div class="skeleton-event">
          <div class="skeleton" style="width: 20%; height: 0.8rem;"></div>
          <div class="skeleton" style="width: 70%; height: 1rem; margin-top: 0.5rem;"></div>
        </div>
      {/each}
    </div>
  {:else if dataStore.errors.gcEvents}
    <div class="error-state">
      <span class="error-icon">⚠️</span>
      <p class="error-msg">{dataStore.errors.gcEvents}</p>
      <button class="retry-btn" onclick={() => dataStore.refreshGCEvents()}>Retry</button>
    </div>
  {:else if dataStore.gcEvents.length === 0}
    <div class="empty-state">
      <span class="empty-icon">🎉</span>
      <p>No GC events — kubelet hasn't needed to garbage collect recently.</p>
    </div>
  {:else}
    <div class="timeline">
      {#each dataStore.gcEvents as event, i}
        <div class="timeline-item" style="animation-delay: {i * 60}ms">
          <div class="timeline-marker marker-{reasonType(event.Reason)}">
            <span class="marker-icon">{reasonIcon(event.Reason)}</span>
          </div>
          <div class="timeline-content">
            <div class="event-header">
              <span class="event-reason reason-{reasonType(event.Reason)}">{event.Reason}</span>
              <span class="event-time">{formatTime(event.Timestamp)}</span>
            </div>
            <p class="event-message">{event.Message}</p>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .skeleton-list { display: grid; gap: var(--space-md); }

  .skeleton-event {
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

  /* Timeline */
  .timeline {
    position: relative;
    padding-left: 2.5rem;
  }

  .timeline::before {
    content: '';
    position: absolute;
    left: 1rem;
    top: 0.5rem;
    bottom: 0.5rem;
    width: 2px;
    background: var(--border-muted);
    border-radius: 1px;
  }

  .timeline-item {
    position: relative;
    padding-bottom: var(--space-md);
    animation: fadeIn var(--transition-slow) both;
  }

  .timeline-marker {
    position: absolute;
    left: -1.75rem;
    top: 0.2rem;
    width: 1.5rem;
    height: 1.5rem;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    background: var(--bg-surface);
    border: 2px solid var(--border-default);
    font-size: 0.7rem;
    z-index: 1;
  }

  .marker-critical { border-color: var(--danger); }
  .marker-success { border-color: var(--success); }

  .timeline-content {
    background: var(--bg-surface);
    border: 1px solid var(--border-muted);
    border-radius: var(--radius-md);
    padding: var(--space-sm) var(--space-md);
    transition: border-color var(--transition-fast);
  }

  .timeline-content:hover {
    border-color: var(--border-default);
  }

  .event-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-sm);
    flex-wrap: wrap;
  }

  .event-reason {
    font-size: 0.78rem;
    font-weight: 600;
    padding: 2px 8px;
    border-radius: var(--radius-full);
    background: var(--bg-overlay);
    color: var(--text-secondary);
  }

  .reason-critical { background: var(--danger-muted); color: var(--danger); }
  .reason-success { background: var(--success-muted); color: var(--success); }

  .event-time {
    font-size: 0.72rem;
    color: var(--text-muted);
  }

  .event-message {
    margin: var(--space-xs) 0 0;
    font-size: 0.85rem;
    color: var(--text-secondary);
    line-height: 1.4;
  }

  @media (max-width: 640px) {
    .timeline { padding-left: 2rem; }
    .timeline-marker { left: -1.5rem; width: 1.25rem; height: 1.25rem; }
  }
</style>
