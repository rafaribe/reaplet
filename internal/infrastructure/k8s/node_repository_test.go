package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildInUseImageMap(t *testing.T) {
	podList := &corev1.PodList{
		Items: []corev1.Pod{
			{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Image:   "nginx:1.25",
							ImageID: "docker-pullable://nginx@sha256:abc123",
						},
						{
							Image:   "sidecar:v2",
							ImageID: "docker-pullable://sidecar@sha256:def456",
						},
					},
				},
			},
			{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Image:   "app:latest",
							ImageID: "", // not pulled yet
						},
					},
				},
			},
		},
	}

	result := buildInUseImageMap(podList)

	// Should have nginx image and imageID
	if !result["nginx:1.25"] {
		t.Error("expected nginx:1.25 to be in use")
	}
	if !result["docker-pullable://nginx@sha256:abc123"] {
		t.Error("expected nginx imageID to be in use")
	}
	// Should have sidecar
	if !result["sidecar:v2"] {
		t.Error("expected sidecar:v2 to be in use")
	}
	// Should NOT have app:latest (empty imageID means not pulled)
	if result["app:latest"] {
		t.Error("expected app:latest to NOT be in use (empty imageID)")
	}
}

func TestBuildInUseImageMap_Nil(t *testing.T) {
	result := buildInUseImageMap(nil)
	if len(result) != 0 {
		t.Errorf("expected empty map for nil podList, got %d entries", len(result))
	}
}

func TestBuildInUseImageMap_Empty(t *testing.T) {
	result := buildInUseImageMap(&corev1.PodList{})
	if len(result) != 0 {
		t.Errorf("expected empty map for empty podList, got %d entries", len(result))
	}
}

func TestMapNode(t *testing.T) {
	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "turing-node-1",
		},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceEphemeralStorage: resource.MustParse("80Gi"),
			},
			Images: []corev1.ContainerImage{
				{
					Names:     []string{"nginx:1.25", "nginx@sha256:abc"},
					SizeBytes: 50 * 1024 * 1024,
				},
				{
					Names:     []string{"app:v3"},
					SizeBytes: 200 * 1024 * 1024,
				},
				{
					Names:     []string{"unused:old"},
					SizeBytes: 100 * 1024 * 1024,
				},
			},
		},
	}

	inUse := map[string]bool{
		"nginx:1.25": true,
		"app:v3":     true,
	}

	result := mapNode(node, inUse)

	if result.Name != "turing-node-1" {
		t.Errorf("expected turing-node-1, got %s", result.Name)
	}

	// Storage checks
	expectedCapacity := int64(100 * 1024 * 1024 * 1024)
	expectedAllocatable := int64(80 * 1024 * 1024 * 1024)
	if result.EphemeralStorage.CapacityBytes != expectedCapacity {
		t.Errorf("expected capacity %d, got %d", expectedCapacity, result.EphemeralStorage.CapacityBytes)
	}
	if result.EphemeralStorage.AvailableBytes != expectedAllocatable {
		t.Errorf("expected available %d, got %d", expectedAllocatable, result.EphemeralStorage.AvailableBytes)
	}
	if result.EphemeralStorage.AllocatedBytes != expectedCapacity-expectedAllocatable {
		t.Errorf("expected allocated %d, got %d", expectedCapacity-expectedAllocatable, result.EphemeralStorage.AllocatedBytes)
	}

	// Image checks
	if len(result.Images) != 3 {
		t.Fatalf("expected 3 images, got %d", len(result.Images))
	}

	// Total image size
	expectedTotal := int64((50 + 200 + 100) * 1024 * 1024)
	if result.TotalImageSize != expectedTotal {
		t.Errorf("expected total image size %d, got %d", expectedTotal, result.TotalImageSize)
	}

	// In-use flags
	if !result.Images[0].InUse {
		t.Error("expected nginx to be in use")
	}
	if !result.Images[1].InUse {
		t.Error("expected app:v3 to be in use")
	}
	if result.Images[2].InUse {
		t.Error("expected unused:old to NOT be in use")
	}
}

func TestMapNode_NoEphemeralStorage(t *testing.T) {
	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "empty-node"},
		Status: corev1.NodeStatus{
			Capacity:    corev1.ResourceList{},
			Allocatable: corev1.ResourceList{},
			Images:      []corev1.ContainerImage{},
		},
	}

	result := mapNode(node, map[string]bool{})

	if result.Name != "empty-node" {
		t.Errorf("expected empty-node, got %s", result.Name)
	}
	if result.EphemeralStorage.CapacityBytes != 0 {
		t.Errorf("expected 0 capacity, got %d", result.EphemeralStorage.CapacityBytes)
	}
	if result.TotalImageSize != 0 {
		t.Errorf("expected 0 total image size, got %d", result.TotalImageSize)
	}
}
