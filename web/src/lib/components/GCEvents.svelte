<script>
  import { fetchGCEvents } from '../api.js';

  let events = $state([]);
  let loading = $state(true);
  let error = $state(null);

  async function load() {
    loading = true;
    error = null;
    try {
      events = await fetchGCEvents();
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function formatTime(ts) {
    if (!ts) return 'Unknown';
    return new Date(ts).toLocaleString();
  }

  function reasonColor(reason) {
    if (reason.includes('Failed') || reason.includes('Pressure')) return 'critical';
    if (reason.includes('Succeeded') || reason.includes('NoDisk')) return 'success';
    return '';
  }

  load();
</script>

<div class="gc-events">
  <div class="header">
    <h2>Garbage Collection Events</h2>
    <button class="refresh" onclick={load}>↻ Refresh</button>
  </div>

  {#if loading}
    <p class="status">Loading events...</p>
  {:else if error}
    <p class="status error">{error}</p>
  {:else if events.length === 0}
    <p class="status">No GC events found. This means kubelet hasn't needed to garbage collect recently.</p>
  {:else}
    <div class="events-list">
      {#each events as event}
        <div class="event-card">
          <div class="event-time">{formatTime(event.Timestamp)}</div>
          <div class="event-body">
            <span class="event-reason {reasonColor(event.Reason)}">{event.Reason}</span>
            <span class="event-message">{event.Message}</span>
          </div>
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

  .event-card {
    background: #16181c;
    border: 1px solid #2f3336;
    border-radius: 8px;
    padding: 0.75rem 1rem;
    margin-bottom: 0.5rem;
  }

  .event-time {
    font-size: 0.75rem;
    color: #71767b;
    margin-bottom: 0.25rem;
  }

  .event-body { display: flex; gap: 0.75rem; align-items: baseline; }

  .event-reason {
    font-size: 0.8rem;
    font-weight: 600;
    padding: 0.1rem 0.4rem;
    border-radius: 4px;
    background: #1d2125;
    white-space: nowrap;
  }

  .event-reason.critical { background: #f4212e33; color: #f4212e; }
  .event-reason.success { background: #00ba7c33; color: #00ba7c; }

  .event-message {
    font-size: 0.85rem;
    color: #e7e9ea;
  }
</style>
