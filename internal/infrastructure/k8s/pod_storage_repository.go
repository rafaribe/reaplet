package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/rafaribe/reaplet/internal/domain/model"
)

// PodStorageRepository implements repository.PodStorageRepository using the Kubernetes API.
type PodStorageRepository struct {
	client kubernetes.Interface
}

func NewPodStorageRepository(client kubernetes.Interface) *PodStorageRepository {
	return &PodStorageRepository{client: client}
}

func (r *PodStorageRepository) GetPodsOnNode(ctx context.Context, nodeName string) ([]model.PodStorageInfo, error) {
	podList, err := r.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s,status.phase=Running", nodeName),
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods on node %s: %w", nodeName, err)
	}

	var pods []model.PodStorageInfo
	for _, pod := range podList.Items {
		var ephemeralBytes int64
		containerCount := len(pod.Spec.Containers)

		// Sum ephemeral storage requests/limits from all containers
		for _, c := range pod.Spec.Containers {
			if req, ok := c.Resources.Requests["ephemeral-storage"]; ok {
				ephemeralBytes += req.Value()
			} else if lim, ok := c.Resources.Limits["ephemeral-storage"]; ok {
				ephemeralBytes += lim.Value()
			}
		}

		// Also check init containers
		for _, c := range pod.Spec.InitContainers {
			if req, ok := c.Resources.Requests["ephemeral-storage"]; ok {
				ephemeralBytes += req.Value()
			} else if lim, ok := c.Resources.Limits["ephemeral-storage"]; ok {
				ephemeralBytes += lim.Value()
			}
		}

		pods = append(pods, model.PodStorageInfo{
			PodName:             pod.Name,
			Namespace:           pod.Namespace,
			NodeName:            nodeName,
			EphemeralUsageBytes: ephemeralBytes,
			ContainerCount:      containerCount,
		})
	}

	return pods, nil
}
