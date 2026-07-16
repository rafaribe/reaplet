/**
 * Theme store — persists to localStorage, applies to document.
 */

const STORAGE_KEY = 'reaplet-theme';
const THEMES = ['dark', 'light', 'catppuccin', 'nord', 'dracula'];
const DEFAULT_THEME = 'dark';

function getInitialTheme() {
  if (typeof window === 'undefined') return DEFAULT_THEME;
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored && THEMES.includes(stored)) return stored;
  // Respect system preference for initial load
  if (window.matchMedia('(prefers-color-scheme: light)').matches) return 'light';
  return DEFAULT_THEME;
}

let current = $state(getInitialTheme());

function applyTheme(theme) {
  document.documentElement.setAttribute('data-theme', theme);
}

// Apply on load
if (typeof window !== 'undefined') {
  applyTheme(current);
}

export const theme = {
  get current() { return current; },
  get themes() { return THEMES; },

  set(newTheme) {
    if (!THEMES.includes(newTheme)) return;
    current = newTheme;
    localStorage.setItem(STORAGE_KEY, newTheme);
    applyTheme(newTheme);
  },

  cycle() {
    const idx = THEMES.indexOf(current);
    const next = THEMES[(idx + 1) % THEMES.length];
    this.set(next);
  },

  labels: {
    dark: '🌙 Dark',
    light: '☀️ Light',
    catppuccin: '🐱 Catppuccin',
    nord: '❄️ Nord',
    dracula: '🧛 Dracula',
  }
};
