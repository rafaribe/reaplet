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
	Names   []string
	SizeBytes int64
	InUse   bool // currently referenced by a running pod
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
	Image       ContainerImage
	NodeName    string
	Reason      string // e.g. "unused", "large", "duplicate tag"
	SavingsBytes int64
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
