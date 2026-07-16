package repository

import (
	"context"

	"github.com/rafaribe/reaplet/internal/domain/model"
)

// NodeRepository provides read access to node storage information.
type NodeRepository interface {
	// GetAll returns all nodes with their storage and image info.
	GetAll(ctx context.Context) ([]model.Node, error)
	// GetByName returns a single node's full details.
	GetByName(ctx context.Context, name string) (*model.Node, error)
}

// GCEventRepository provides access to garbage collection events.
type GCEventRepository interface {
	// GetRecentEvents returns recent image GC events across the cluster.
	GetRecentEvents(ctx context.Context, limit int) ([]model.GCEvent, error)
	// GetEventsForNode returns GC events for a specific node.
	GetEventsForNode(ctx context.Context, nodeName string, limit int) ([]model.GCEvent, error)
}

// PodEvictionRepository provides the ability to evict pods.
type PodEvictionRepository interface {
	// Evict evicts a pod from a node.
	Evict(ctx context.Context, req model.EvictionRequest) (*model.EvictionResult, error)
}

// ImageRepository provides direct image management via CRI.
type ImageRepository interface {
	// RemoveImage removes a container image from a specific node.
	RemoveImage(ctx context.Context, req model.ImageRemovalRequest) (*model.ImageRemovalResult, error)
	// ListImages lists all images on a specific node via CRI.
	ListImages(ctx context.Context, nodeName string) ([]model.ContainerImage, error)
}
