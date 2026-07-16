<script>
  import { dataStore } from './lib/stores/data.svelte.js';
  import ThemeToggle from './lib/components/ThemeToggle.svelte';
  import Toast from './lib/components/Toast.svelte';
  import NodeList from './lib/components/NodeList.svelte';
  import Recommendations from './lib/components/Recommendations.svelte';
  import GCEvents from './lib/components/GCEvents.svelte';

  let activeTab = $state('nodes');

  // Initialize data store
  dataStore.init();

  const tabs = [
    { id: 'nodes', label: 'Nodes', icon: '🖥️' },
    { id: 'recommendations', label: 'Recommendations', icon: '💡' },
    { id: 'gc-events', label: 'GC Events', icon: '🗑️' },
  ];

  function formatTime(date) {
    if (!date) return '';
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  }
</script>

<div class="app">
  <header class="app-header">
    <div class="header-left">
      <h1 class="brand">
        <span class="brand-icon">🌾</span>
        <span class="brand-text">Reaplet</span>
      </h1>
      <span class="brand-subtitle">Storage Monitor</span>
    </div>

    <div class="header-right">
      {#if dataStore.lastUpdated}
        <span class="last-updated" title="Last updated">
          <span class="pulse-dot"></span>
          {formatTime(dataStore.lastUpdated)}
        </span>
      {/if}

      <button
        class="refresh-btn"
        onclick={() => dataStore.refreshAll()}
        aria-label="Refresh data"
        disabled={dataStore.loading.nodes}
      >
        <span class="refresh-icon" class:spinning={dataStore.loading.nodes}>↻</span>
      </button>

      <ThemeToggle />
    </div>
  </header>

  <div class="tab-bar" role="tablist">
    {#each tabs as tab}
      <button
        class="tab"
        class:active={activeTab === tab.id}
        onclick={() => activeTab = tab.id}
        role="tab"
        aria-selected={activeTab === tab.id}
      >
        <span class="tab-icon">{tab.icon}</span>
        <span class="tab-label">{tab.label}</span>
      </button>
    {/each}
  </div>

  <main class="content">
    {#if activeTab === 'nodes'}
      <div class="panel animate-in">
        <NodeList />
      </div>
    {:else if activeTab === 'recommendations'}
      <div class="panel animate-in">
        <Recommendations />
      </div>
    {:else if activeTab === 'gc-events'}
      <div class="panel animate-in">
        <GCEvents />
      </div>
    {/if}
  </main>

  <footer class="app-footer">
    <span class="footer-text">
      Auto-refresh
      <button
        class="auto-toggle"
        class:active={dataStore.autoRefreshEnabled}
        onclick={() => dataStore.setAutoRefresh(!dataStore.autoRefreshEnabled)}
      >
        {dataStore.autoRefreshEnabled ? 'On' : 'Off'}
      </button>
    </span>
  </footer>
</div>

<Toast />

<style>
  .app {
    max-width: 1280px;
    margin: 0 auto;
    padding: var(--space-md) var(--space-lg);
    min-height: 100vh;
    display: flex;
    flex-direction: column;
  }

  /* Header */
  .app-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: var(--space-md) 0;
    margin-bottom: var(--space-md);
  }

  .header-left {
    display: flex;
    align-items: baseline;
    gap: var(--space-md);
  }

  .brand {
    margin: 0;
    font-size: 1.5rem;
    font-weight: 700;
    display: flex;
    align-items: center;
    gap: var(--space-sm);
  }

  .brand-icon {
    font-size: 1.4rem;
  }

  .brand-text {
    background: var(--gradient-accent);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
  }

  .brand-subtitle {
    color: var(--text-muted);
    font-size: 0.8rem;
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
  }

  .last-updated {
    display: flex;
    align-items: center;
    gap: var(--space-xs);
    font-size: 0.75rem;
    color: var(--text-muted);
    padding: var(--space-xs) var(--space-sm);
    background: var(--bg-surface);
    border-radius: var(--radius-full);
  }

  .pulse-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--success);
    animation: pulse 2s infinite;
  }

  .refresh-btn {
    background: var(--bg-surface);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md);
    width: 38px;
    height: 38px;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    color: var(--text-secondary);
    font-size: 1.1rem;
    transition: all var(--transition-fast);
  }

  .refresh-btn:hover:not(:disabled) {
    border-color: var(--accent);
    color: var(--accent);
    box-shadow: var(--shadow-glow);
  }

  .refresh-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .refresh-icon {
    display: inline-block;
    transition: transform var(--transition-normal);
  }

  .refresh-icon.spinning {
    animation: spin 1s linear infinite;
  }

  /* Tabs */
  .tab-bar {
    display: flex;
    gap: var(--space-xs);
    padding: var(--space-xs);
    background: var(--bg-surface);
    border-radius: var(--radius-lg);
    border: 1px solid var(--border-muted);
    margin-bottom: var(--space-lg);
  }

  .tab {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-xs);
    padding: var(--space-sm) var(--space-md);
    background: none;
    border: none;
    border-radius: var(--radius-md);
    color: var(--text-secondary);
    cursor: pointer;
    font-size: 0.85rem;
    font-weight: 500;
    transition: all var(--transition-fast);
  }

  .tab:hover {
    background: var(--bg-elevated);
    color: var(--text-primary);
  }

  .tab.active {
    background: var(--accent);
    color: var(--text-inverse);
    box-shadow: var(--shadow-sm);
  }

  .tab-icon {
    font-size: 1rem;
  }

  /* Content */
  .content {
    flex: 1;
  }

  .panel {
    animation: fadeIn var(--transition-slow) both;
  }

  /* Footer */
  .app-footer {
    display: flex;
    justify-content: center;
    padding: var(--space-lg) 0 var(--space-md);
  }

  .footer-text {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
    font-size: 0.75rem;
    color: var(--text-muted);
  }

  .auto-toggle {
    background: var(--bg-surface);
    border: 1px solid var(--border-default);
    color: var(--text-secondary);
    padding: 2px 8px;
    border-radius: var(--radius-full);
    cursor: pointer;
    font-size: 0.7rem;
    font-weight: 600;
    text-transform: uppercase;
    transition: all var(--transition-fast);
  }

  .auto-toggle.active {
    background: var(--success-muted);
    border-color: var(--success);
    color: var(--success);
  }

  /* Responsive */
  @media (max-width: 640px) {
    .app {
      padding: var(--space-sm) var(--space-md);
    }

    .brand-subtitle { display: none; }
    .last-updated { display: none; }
    .tab-label { display: none; }

    .tab {
      padding: var(--space-sm);
    }

    .tab-icon {
      font-size: 1.2rem;
    }
  }
</style>
