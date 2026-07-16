package cri

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/rafaribe/reaplet/internal/domain/model"
)

const (
	// DefaultContainerdSocket is the default containerd CRI socket on Talos.
	DefaultContainerdSocket = "unix:///run/containerd/containerd.sock"
	dialTimeout             = 5 * time.Second
)

// ImageRepository implements repository.ImageRepository via CRI.
// This runs inside the privileged DaemonSet pod with the containerd socket mounted.
type ImageRepository struct {
	socketPath string
}

func NewImageRepository(socketPath string) *ImageRepository {
	if socketPath == "" {
		socketPath = DefaultContainerdSocket
	}
	return &ImageRepository{socketPath: socketPath}
}

func (r *ImageRepository) connect(ctx context.Context) (runtimeapi.ImageServiceClient, *grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, r.socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to CRI socket %s: %w", r.socketPath, err)
	}
	return runtimeapi.NewImageServiceClient(conn), conn, nil
}

func (r *ImageRepository) ListImages(ctx context.Context, _ string) ([]model.ContainerImage, error) {
	client, conn, err := r.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	resp, err := client.ListImages(ctx, &runtimeapi.ListImagesRequest{})
	if err != nil {
		return nil, fmt.Errorf("listing images via CRI: %w", err)
	}

	images := make([]model.ContainerImage, 0, len(resp.Images))
	for _, img := range resp.Images {
		names := make([]string, 0, len(img.RepoTags)+len(img.RepoDigests))
		names = append(names, img.RepoTags...)
		names = append(names, img.RepoDigests...)

		images = append(images, model.ContainerImage{
			Names:     names,
			SizeBytes: int64(img.Size_),
			InUse:     false, // caller determines this
		})
	}
	return images, nil
}

func (r *ImageRepository) RemoveImage(ctx context.Context, req model.ImageRemovalRequest) (*model.ImageRemovalResult, error) {
	client, conn, err := r.connect(ctx)
	if err != nil {
		return &model.ImageRemovalResult{
			ImageRef: req.ImageRef,
			NodeName: req.NodeName,
			Success:  false,
			Error:    err.Error(),
		}, nil
	}
	defer conn.Close()

	// Get image size before removal for reporting
	var freedBytes int64
	inspectResp, err := client.ImageStatus(ctx, &runtimeapi.ImageStatusRequest{
		Image: &runtimeapi.ImageSpec{Image: req.ImageRef},
	})
	if err == nil && inspectResp.Image != nil {
		freedBytes = int64(inspectResp.Image.Size_)
	}

	_, err = client.RemoveImage(ctx, &runtimeapi.RemoveImageRequest{
		Image: &runtimeapi.ImageSpec{Image: req.ImageRef},
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
