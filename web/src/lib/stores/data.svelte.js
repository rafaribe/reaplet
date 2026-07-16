/**
 * Shared reactive data store with auto-refresh polling.
 */
import { fetchNodes, fetchGCEvents, fetchRecommendations } from '../api.js';

const DEFAULT_INTERVAL = 30_000; // 30s

// --- State ---
let nodes = $state([]);
let gcEvents = $state([]);
let recommendations = $state([]);
let loading = $state({ nodes: true, gcEvents: true, recommendations: true });
let errors = $state({ nodes: null, gcEvents: null, recommendations: null });
let lastUpdated = $state(null);
let autoRefreshEnabled = $state(true);
let refreshInterval = $state(DEFAULT_INTERVAL);
let timers = {};

// --- Internal fetchers ---
async function loadNodes() {
  loading.nodes = true;
  errors.nodes = null;
  try {
    nodes = await fetchNodes();
    lastUpdated = new Date();
  } catch (e) {
    errors.nodes = e.message;
  } finally {
    loading.nodes = false;
  }
}

async function loadGCEvents() {
  loading.gcEvents = true;
  errors.gcEvents = null;
  try {
    gcEvents = await fetchGCEvents();
    lastUpdated = new Date();
  } catch (e) {
    errors.gcEvents = e.message;
  } finally {
    loading.gcEvents = false;
  }
}

async function loadRecommendations() {
  loading.recommendations = true;
  errors.recommendations = null;
  try {
    recommendations = await fetchRecommendations();
    lastUpdated = new Date();
  } catch (e) {
    errors.recommendations = e.message;
  } finally {
    loading.recommendations = false;
  }
}

// --- Auto-refresh control ---
function startPolling() {
  stopPolling();
  if (!autoRefreshEnabled) return;
  timers.all = setInterval(() => {
    loadNodes();
    loadGCEvents();
    loadRecommendations();
  }, refreshInterval);
}

function stopPolling() {
  if (timers.all) {
    clearInterval(timers.all);
    timers.all = null;
  }
}

// --- Public API ---
export const dataStore = {
  get nodes() { return nodes; },
  get gcEvents() { return gcEvents; },
  get recommendations() { return recommendations; },
  get loading() { return loading; },
  get errors() { return errors; },
  get lastUpdated() { return lastUpdated; },
  get autoRefreshEnabled() { return autoRefreshEnabled; },
  get refreshInterval() { return refreshInterval; },

  async init() {
    await Promise.all([loadNodes(), loadGCEvents(), loadRecommendations()]);
    startPolling();
  },

  async refreshAll() {
    await Promise.all([loadNodes(), loadGCEvents(), loadRecommendations()]);
  },

  async refreshNodes() { await loadNodes(); },
  async refreshGCEvents() { await loadGCEvents(); },
  async refreshRecommendations() { await loadRecommendations(); },

  setAutoRefresh(enabled) {
    autoRefreshEnabled = enabled;
    if (enabled) startPolling();
    else stopPolling();
  },

  setInterval(ms) {
    refreshInterval = ms;
    if (autoRefreshEnabled) startPolling();
  },

  destroy() {
    stopPolling();
  }
};
