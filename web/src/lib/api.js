const BASE = '';

export async function fetchNodes() {
  const res = await fetch(`${BASE}/api/nodes`);
  if (!res.ok) throw new Error(`Failed to fetch nodes: ${res.statusText}`);
  return res.json();
}

export async function fetchNode(name) {
  const res = await fetch(`${BASE}/api/nodes/${name}`);
  if (!res.ok) throw new Error(`Failed to fetch node: ${res.statusText}`);
  return res.json();
}

export async function fetchGCEvents() {
  const res = await fetch(`${BASE}/api/gc-events`);
  if (!res.ok) throw new Error(`Failed to fetch GC events: ${res.statusText}`);
  return res.json();
}

export async function fetchRecommendations() {
  const res = await fetch(`${BASE}/api/recommendations`);
  if (!res.ok) throw new Error(`Failed to fetch recommendations: ${res.statusText}`);
  return res.json();
}

export async function evictPod(podName, namespace, nodeName, reason) {
  const res = await fetch(`${BASE}/api/evict`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ PodName: podName, Namespace: namespace, NodeName: nodeName, Reason: reason }),
  });
  if (!res.ok) throw new Error(`Eviction failed: ${res.statusText}`);
  return res.json();
}

export async function removeImage(imageRef, nodeName) {
  const res = await fetch(`${BASE}/api/remove-image`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ NodeName: nodeName, ImageRef: imageRef }),
  });
  if (!res.ok) throw new Error(`Image removal failed: ${res.statusText}`);
  return res.json();
}

export function formatBytes(bytes) {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

export function formatPercent(used, total) {
  if (total === 0) return '0%';
  return `${((used / total) * 100).toFixed(1)}%`;
}
