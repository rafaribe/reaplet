<script>
  import NodeList from './lib/components/NodeList.svelte';
  import Recommendations from './lib/components/Recommendations.svelte';
  import GCEvents from './lib/components/GCEvents.svelte';

  let activeTab = $state('nodes');
</script>

<main>
  <header>
    <h1>🌾 Reaplet</h1>
    <p class="subtitle">Talos Node Storage Monitor</p>
  </header>

  <nav>
    <button class:active={activeTab === 'nodes'} onclick={() => activeTab = 'nodes'}>
      Nodes
    </button>
    <button class:active={activeTab === 'recommendations'} onclick={() => activeTab = 'recommendations'}>
      Recommendations
    </button>
    <button class:active={activeTab === 'gc-events'} onclick={() => activeTab = 'gc-events'}>
      GC Events
    </button>
  </nav>

  <section class="content">
    {#if activeTab === 'nodes'}
      <NodeList />
    {:else if activeTab === 'recommendations'}
      <Recommendations />
    {:else if activeTab === 'gc-events'}
      <GCEvents />
    {/if}
  </section>
</main>

<style>
  :global(body) {
    margin: 0;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: #0f1419;
    color: #e7e9ea;
  }

  main {
    max-width: 1200px;
    margin: 0 auto;
    padding: 1rem;
  }

  header {
    text-align: center;
    margin-bottom: 1.5rem;
  }

  header h1 {
    margin: 0;
    font-size: 2rem;
  }

  .subtitle {
    color: #71767b;
    margin: 0.25rem 0 0;
  }

  nav {
    display: flex;
    gap: 0.5rem;
    margin-bottom: 1.5rem;
    border-bottom: 1px solid #2f3336;
    padding-bottom: 0.5rem;
  }

  nav button {
    background: none;
    border: none;
    color: #71767b;
    padding: 0.5rem 1rem;
    cursor: pointer;
    border-radius: 4px;
    font-size: 0.9rem;
  }

  nav button:hover {
    background: #1d2125;
    color: #e7e9ea;
  }

  nav button.active {
    background: #1d9bf0;
    color: white;
  }

  .content {
    min-height: 400px;
  }
</style>
