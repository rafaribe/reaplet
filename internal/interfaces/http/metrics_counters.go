package http

import "sync"

// Counters tracks operation counters for Prometheus metrics.
// Thread-safe for concurrent access from HTTP handlers.
type Counters struct {
	mu        sync.RWMutex
	removals  map[string]map[string]int64 // node -> status -> count
	evictions map[string]map[string]int64 // node -> status -> count
	gcEvents  map[string]int64            // reason -> count
}

// GlobalCounters is the singleton counter instance.
var GlobalCounters = NewCounters()

func NewCounters() *Counters {
	return &Counters{
		removals:  make(map[string]map[string]int64),
		evictions: make(map[string]map[string]int64),
		gcEvents:  make(map[string]int64),
	}
}

// IncrImageRemoval increments the image removal counter.
func (c *Counters) IncrImageRemoval(node, status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.removals[node] == nil {
		c.removals[node] = make(map[string]int64)
	}
	c.removals[node][status]++
}

// IncrPodEviction increments the pod eviction counter.
func (c *Counters) IncrPodEviction(node, status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.evictions[node] == nil {
		c.evictions[node] = make(map[string]int64)
	}
	c.evictions[node][status]++
}

// IncrGCEvent increments the GC event counter by reason.
func (c *Counters) IncrGCEvent(reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gcEvents[reason]++
}

// SetGCEvents replaces the GC event counts (used when pulling from K8s events).
func (c *Counters) SetGCEvents(counts map[string]int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gcEvents = counts
}

// Snapshot returns a copy of all counter values.
func (c *Counters) Snapshot() (removals, evictions map[string]map[string]int64, gcEvents map[string]int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	removals = make(map[string]map[string]int64)
	for node, statuses := range c.removals {
		removals[node] = make(map[string]int64)
		for status, count := range statuses {
			removals[node][status] = count
		}
	}

	evictions = make(map[string]map[string]int64)
	for node, statuses := range c.evictions {
		evictions[node] = make(map[string]int64)
		for status, count := range statuses {
			evictions[node][status] = count
		}
	}

	gcEvents = make(map[string]int64)
	for reason, count := range c.gcEvents {
		gcEvents[reason] = count
	}

	return
}
