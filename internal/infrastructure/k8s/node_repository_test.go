package k8s

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("buildInUseImageMap", func() {
	It("marks running container images and imageIDs as in use", func() {
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

		Expect(result["nginx:1.25"]).To(BeTrue())
		Expect(result["docker-pullable://nginx@sha256:abc123"]).To(BeTrue())
		Expect(result["sidecar:v2"]).To(BeTrue())
		Expect(result["app:latest"]).To(BeFalse(), "empty imageID means not pulled")
	})

	It("returns an empty map for nil podList", func() {
		result := buildInUseImageMap(nil)
		Expect(result).To(BeEmpty())
	})

	It("returns an empty map for empty podList", func() {
		result := buildInUseImageMap(&corev1.PodList{})
		Expect(result).To(BeEmpty())
	})
})

var _ = Describe("mapNode", func() {
	It("maps a node with storage, images, and in-use flags", func() {
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

		Expect(result.Name).To(Equal("turing-node-1"))

		// Storage checks
		expectedCapacity := int64(100 * 1024 * 1024 * 1024)
		expectedAllocatable := int64(80 * 1024 * 1024 * 1024)
		Expect(result.EphemeralStorage.CapacityBytes).To(Equal(expectedCapacity))
		Expect(result.EphemeralStorage.AvailableBytes).To(Equal(expectedAllocatable))
		Expect(result.EphemeralStorage.AllocatedBytes).To(Equal(expectedCapacity - expectedAllocatable))

		// Image checks
		Expect(result.Images).To(HaveLen(3))

		expectedTotal := int64((50 + 200 + 100) * 1024 * 1024)
		Expect(result.TotalImageSize).To(Equal(expectedTotal))

		Expect(result.Images[0].InUse).To(BeTrue())
		Expect(result.Images[1].InUse).To(BeTrue())
		Expect(result.Images[2].InUse).To(BeFalse())
	})

	It("handles a node with no ephemeral storage", func() {
		node := corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "empty-node"},
			Status: corev1.NodeStatus{
				Capacity:    corev1.ResourceList{},
				Allocatable: corev1.ResourceList{},
				Images:      []corev1.ContainerImage{},
			},
		}

		result := mapNode(node, map[string]bool{})

		Expect(result.Name).To(Equal("empty-node"))
		Expect(result.EphemeralStorage.CapacityBytes).To(BeZero())
		Expect(result.TotalImageSize).To(BeZero())
	})
})
