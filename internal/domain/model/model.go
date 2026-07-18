package model

import "time"

// Node represents a Kubernetes node with storage information.
type Node struct {
	Name             string
	EphemeralStorage StorageInfo
	Images           []ContainerImage
	TotalImageSize   int64
	LastGCEvent      *GCEvent
}

// StorageInfo represents disk capacity and usage.
type StorageInfo struct {
	CapacityBytes  int64
	AllocatedBytes int64
	AvailableBytes int64
}

// ContainerImage represents a container image on a node.
type ContainerImage struct {
	Names     []string
	Digest    string // sha256 digest, extracted from names or Talos API
	SizeBytes int64
	InUse     bool // currently referenced by a running pod
}

// GCEvent represents a kubelet garbage collection event.
type GCEvent struct {
	Timestamp    time.Time
	Reason       string
	Message      string
	ImagesFreed  int
	SpaceFreed   int64
}

// ImageRecommendation suggests an image for removal.
type ImageRecommendation struct {
	Image        ContainerImage
	NodeName     string
	Reason       string // e.g. "unused", "large", "unused for 30 days"
	SavingsBytes int64
	UnusedDays   int // days since first seen unused (0 if unknown)
}

// EvictionRequest represents a request to evict a pod.
type EvictionRequest struct {
	PodName   string
	Namespace string
	NodeName  string
	Reason    string
}

// EvictionResult is the outcome of an eviction attempt.
type EvictionResult struct {
	PodName   string
	Namespace string
	Success   bool
	Error     string
}

// ImageRemovalRequest represents a request to remove images via CRI.
type ImageRemovalRequest struct {
	NodeName string
	ImageRef string
}

// ImageRemovalResult is the outcome of an image removal.
type ImageRemovalResult struct {
	ImageRef    string
	NodeName    string
	Success     bool
	FreedBytes  int64
	Error       string
}

// PodStorageInfo represents a pod's ephemeral storage usage on a node.
type PodStorageInfo struct {
	PodName            string `json:"podName"`
	Namespace          string `json:"namespace"`
	NodeName           string `json:"nodeName"`
	EphemeralUsageBytes int64  `json:"ephemeralUsageBytes"`
	ContainerCount     int    `json:"containerCount"`
}

// ImageDuplicateGroup identifies images with the same digest present across nodes.
type ImageDuplicateGroup struct {
	Digest      string   `json:"digest"`
	Names       []string `json:"names"`
	Nodes       []string `json:"nodes"`
	SizeBytes   int64    `json:"sizeBytes"`
	WastedBytes int64    `json:"wastedBytes"`
}

// StorageForecast projects when a node will hit storage thresholds.
type StorageForecast struct {
	NodeName                 string  `json:"nodeName"`
	CurrentPct               float64 `json:"currentPct"`
	ProjectedDaysToWarning   float64 `json:"projectedDaysToWarning"`
	ProjectedDaysToCritical  float64 `json:"projectedDaysToCritical"`
	TrendBytesPerDay         int64   `json:"trendBytesPerDay"`
}

// WarmListEntry represents an image in the warm list (pre-pull management).
type WarmListEntry struct {
	ID       int64     `json:"id"`
	ImageRef string    `json:"imageRef"`
	AddedAt  time.Time `json:"addedAt"`
}

// WarmListStatus shows warm list images and which nodes are missing them.
type WarmListStatus struct {
	Entries      []WarmListEntry       `json:"entries"`
	MissingOnNodes map[string][]string `json:"missingOnNodes"` // imageRef → []nodeName
}

// PreWarmCheckRequest is the input for pre-warm checking.
type PreWarmCheckRequest struct {
	ImageRef string `json:"imageRef"`
	NodeName string `json:"nodeName"`
}

// PreWarmCheckResult is the result of checking if an image exists on a node.
type PreWarmCheckResult struct {
	Exists    bool  `json:"exists"`
	SizeBytes int64 `json:"sizeBytes"`
	CanPull   bool  `json:"canPull"`
}

// UpgradeCheckResult estimates if a node has enough space for a Talos upgrade.
type UpgradeCheckResult struct {
	NodeName            string `json:"nodeName"`
	AvailableBytes      int64  `json:"availableBytes"`
	EstimatedNeededBytes int64  `json:"estimatedNeededBytes"`
	Safe                bool   `json:"safe"`
	Message             string `json:"message"`
}
