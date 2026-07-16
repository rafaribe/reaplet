/**
 * Toast notification system.
 */

let toasts = $state([]);
let idCounter = 0;

export const toastStore = {
  get toasts() { return toasts; },

  add(message, type = 'info', duration = 4000) {
    const id = ++idCounter;
    const toast = { id, message, type, timestamp: Date.now() };
    toasts = [...toasts, toast];

    if (duration > 0) {
      setTimeout(() => this.dismiss(id), duration);
    }
    return id;
  },

  success(message, duration) { return this.add(message, 'success', duration); },
  error(message, duration) { return this.add(message, 'error', duration ?? 6000); },
  warning(message, duration) { return this.add(message, 'warning', duration); },
  info(message, duration) { return this.add(message, 'info', duration); },

  dismiss(id) {
    toasts = toasts.filter(t => t.id !== id);
  },

  clear() {
    toasts = [];
  }
};
