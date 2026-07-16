<script>
  import { toastStore } from '../stores/toast.svelte.js';

  const icons = {
    success: '✓',
    error: '✕',
    warning: '⚠',
    info: 'ℹ',
  };
</script>

{#if toastStore.toasts.length > 0}
  <div class="toast-container" role="status" aria-live="polite">
    {#each toastStore.toasts as toast (toast.id)}
      <div class="toast toast-{toast.type}" class:exiting={false}>
        <span class="toast-icon">{icons[toast.type]}</span>
        <span class="toast-message">{toast.message}</span>
        <button class="toast-dismiss" onclick={() => toastStore.dismiss(toast.id)} aria-label="Dismiss">×</button>
      </div>
    {/each}
  </div>
{/if}

<style>
  .toast-container {
    position: fixed;
    bottom: var(--space-lg);
    right: var(--space-lg);
    z-index: 9999;
    display: flex;
    flex-direction: column-reverse;
    gap: var(--space-sm);
    max-width: 380px;
    pointer-events: none;
  }

  .toast {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
    padding: var(--space-sm) var(--space-md);
    border-radius: var(--radius-md);
    background: var(--bg-elevated);
    border: 1px solid var(--border-default);
    box-shadow: var(--shadow-lg);
    animation: slideInUp var(--transition-normal) both;
    pointer-events: all;
    backdrop-filter: blur(8px);
  }

  .toast-success { border-left: 3px solid var(--success); }
  .toast-error { border-left: 3px solid var(--danger); }
  .toast-warning { border-left: 3px solid var(--warning); }
  .toast-info { border-left: 3px solid var(--accent); }

  .toast-icon {
    font-size: 1rem;
    font-weight: 700;
    min-width: 1.25rem;
    text-align: center;
  }

  .toast-success .toast-icon { color: var(--success); }
  .toast-error .toast-icon { color: var(--danger); }
  .toast-warning .toast-icon { color: var(--warning); }
  .toast-info .toast-icon { color: var(--accent); }

  .toast-message {
    flex: 1;
    font-size: 0.85rem;
    color: var(--text-primary);
  }

  .toast-dismiss {
    background: none;
    border: none;
    color: var(--text-muted);
    font-size: 1.2rem;
    cursor: pointer;
    padding: 0 var(--space-xs);
    line-height: 1;
    border-radius: var(--radius-sm);
    transition: color var(--transition-fast);
  }

  .toast-dismiss:hover {
    color: var(--text-primary);
  }

  @media (max-width: 480px) {
    .toast-container {
      left: var(--space-md);
      right: var(--space-md);
      bottom: var(--space-md);
      max-width: none;
    }
  }
</style>
