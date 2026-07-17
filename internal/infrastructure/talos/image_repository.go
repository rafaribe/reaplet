package talos

import (
	"context"
	"fmt"
	"io"

	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"

	"github.com/rafaribe/reaplet/internal/domain/model"
)

// ImageRepository implements repository.ImageRepository via the Talos gRPC API.
// This runs in a single Deployment — no DaemonSet or privileged access needed.
type ImageRepository struct {
	client *client.Client
}

// NewImageRepository creates a new Talos-backed image repository.
func NewImageRepository(c *client.Client) *ImageRepository {
	return &ImageRepository{client: c}
}

// criInstance returns the containerd instance config for CRI (kubernetes) images.
func criInstance() *common.ContainerdInstance {
	return &common.ContainerdInstance{
		Driver:    common.ContainerDriver_CRI,
		Namespace: common.ContainerdNamespace_NS_CRI,
	}
}

// ListImages lists all container images on a specific node via Talos API.
func (r *ImageRepository) ListImages(ctx context.Context, nodeName string) ([]model.ContainerImage, error) {
	// Target the specific node
	ctx = client.WithNode(ctx, nodeName)

	stream, err := r.client.ImageClient.List(ctx, &machine.ImageServiceListRequest{
		Containerd: criInstance(),
	})
	if err != nil {
		return nil, fmt.Errorf("listing images on %s: %w", nodeName, err)
	}

	var images []model.ContainerImage

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("receiving image list from %s: %w", nodeName, err)
		}

		images = append(images, model.ContainerImage{
			Names:     []string{resp.GetName()},
			SizeBytes: resp.GetSize(),
			InUse:     false, // caller determines this via pod cross-reference
		})
	}

	return images, nil
}

// RemoveImage removes a container image from a specific node via Talos API.
func (r *ImageRepository) RemoveImage(ctx context.Context, req model.ImageRemovalRequest) (*model.ImageRemovalResult, error) {
	// Target the specific node
	ctx = client.WithNode(ctx, req.NodeName)

	// Get image size before removal (best effort — list and find matching)
	var freedBytes int64

	stream, err := r.client.ImageClient.List(ctx, &machine.ImageServiceListRequest{
		Containerd: criInstance(),
	})
	if err == nil {
		for {
			resp, err := stream.Recv()
			if err != nil {
				break
			}
			if resp.GetName() == req.ImageRef {
				freedBytes = resp.GetSize()
				break
			}
		}
	}

	// Remove the image
	_, err = r.client.ImageClient.Remove(ctx, &machine.ImageServiceRemoveRequest{
		Containerd: criInstance(),
		ImageRef:   req.ImageRef,
	})
	if err != nil {
		return &model.ImageRemovalResult{
			ImageRef: req.ImageRef,
			NodeName: req.NodeName,
			Success:  false,
			Error:    err.Error(),
		}, nil
	}

	return &model.ImageRemovalResult{
		ImageRef:   req.ImageRef,
		NodeName:   req.NodeName,
		Success:    true,
		FreedBytes: freedBytes,
	}, nil
}
