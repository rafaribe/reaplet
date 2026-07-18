package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/rafaribe/reaplet/internal/domain/model"
)

// NodeRepository implements repository.NodeRepository using the Kubernetes API.
type NodeRepository struct {
	client kubernetes.Interface
}

func NewNodeRepository(client kubernetes.Interface) *NodeRepository {
	return &NodeRepository{client: client}
}

func (r *NodeRepository) GetAll(ctx context.Context) ([]model.Node, error) {
	nodeList, err := r.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}

	// Get all pods to determine which images are in use
	podList, err := r.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	inUseImages := buildInUseImageMap(podList)

	nodes := make([]model.Node, 0, len(nodeList.Items))
	for _, n := range nodeList.Items {
		nodes = append(nodes, mapNode(n, inUseImages))
	}
	return nodes, nil
}

func (r *NodeRepository) GetByName(ctx context.Context, name string) (*model.Node, error) {
	n, err := r.client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting node %s: %w", name, err)
	}

	podList, err := r.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s,status.phase=Running", name),
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods on node %s: %w", name, err)
	}

	inUseImages := buildInUseImageMap(podList)
	node := mapNode(*n, inUseImages)
	return &node, nil
}

func mapNode(n corev1.Node, inUseImages map[string]bool) model.Node {
	var totalImageSize int64
	images := make([]model.ContainerImage, 0, len(n.Status.Images))

	for _, img := range n.Status.Images {
		inUse := false
		for _, name := range img.Names {
			if inUseImages[name] {
				inUse = true
				break
			}
		}
		totalImageSize += img.SizeBytes

		// Extract digest from image names (format: repo@sha256:xxx or sha256:xxx)
		digest := extractImageDigest(img.Names)

		images = append(images, model.ContainerImage{
			Names:     img.Names,
			Digest:    digest,
			SizeBytes: img.SizeBytes,
			InUse:     inUse,
		})
	}

	ephemeral := n.Status.Capacity[corev1.ResourceEphemeralStorage]
	allocatable := n.Status.Allocatable[corev1.ResourceEphemeralStorage]

	return model.Node{
		Name: n.Name,
		EphemeralStorage: model.StorageInfo{
			CapacityBytes:  ephemeral.Value(),
			AllocatedBytes: ephemeral.Value() - allocatable.Value(),
			AvailableBytes: allocatable.Value(),
		},
		Images:         images,
		TotalImageSize: totalImageSize,
	}
}

// extractImageDigest extracts sha256 digest from a list of image names.
// Images may have entries like "nginx@sha256:abc123" or "sha256:abc123".
func extractImageDigest(names []string) string {
	for _, name := range names {
		for i := 0; i < len(name)-7; i++ {
			if name[i:i+7] == "sha256:" {
				return name[i:]
			}
		}
	}
	return ""
}

func buildInUseImageMap(podList *corev1.PodList) map[string]bool {
	inUse := make(map[string]bool)
	if podList == nil {
		return inUse
	}
	for _, pod := range podList.Items {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.ImageID != "" {
				inUse[cs.Image] = true
				inUse[cs.ImageID] = true
			}
		}
	}
	return inUse
}
