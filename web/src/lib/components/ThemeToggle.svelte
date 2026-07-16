<script>
  import { theme } from '../stores/theme.svelte.js';

  let open = $state(false);

  function select(t) {
    theme.set(t);
    open = false;
  }

  function handleClickOutside(e) {
    if (!e.target.closest('.theme-toggle')) {
      open = false;
    }
  }
</script>

<svelte:window onclick={handleClickOutside} />

<div class="theme-toggle">
  <button
    class="toggle-btn"
    onclick={() => open = !open}
    aria-label="Change theme"
    aria-expanded={open}
  >
    <span class="toggle-icon">
      {#if theme.current === 'light'}☀️
      {:else if theme.current === 'catppuccin'}🐱
      {:else if theme.current === 'nord'}❄️
      {:else if theme.current === 'dracula'}🧛
      {:else}🌙{/if}
    </span>
  </button>

  {#if open}
    <div class="dropdown" role="menu">
      {#each theme.themes as t}
        <button
          class="dropdown-item"
          class:active={theme.current === t}
          onclick={() => select(t)}
          role="menuitem"
        >
          {theme.labels[t]}
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .theme-toggle {
    position: relative;
  }

  .toggle-btn {
    background: var(--bg-surface);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md);
    padding: var(--space-xs) var(--space-sm);
    cursor: pointer;
    font-size: 1.2rem;
    line-height: 1;
    transition: all var(--transition-fast);
    display: flex;
    align-items: center;
    justify-content: center;
    width: 38px;
    height: 38px;
  }

  .toggle-btn:hover {
    background: var(--bg-elevated);
    border-color: var(--accent);
    box-shadow: var(--shadow-glow);
  }

  .dropdown {
    position: absolute;
    top: calc(100% + var(--space-sm));
    right: 0;
    background: var(--bg-elevated);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-lg);
    overflow: hidden;
    z-index: 100;
    min-width: 160px;
    animation: fadeIn var(--transition-fast) both;
  }

  .dropdown-item {
    display: block;
    width: 100%;
    padding: var(--space-sm) var(--space-md);
    background: none;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    text-align: left;
    font-size: 0.85rem;
    transition: all var(--transition-fast);
  }

  .dropdown-item:hover {
    background: var(--accent-muted);
    color: var(--text-primary);
  }

  .dropdown-item.active {
    background: var(--accent-muted);
    color: var(--accent);
    font-weight: 600;
  }
</style>
