package usecase_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rafaribe/reaplet/internal/domain/model"
	"github.com/rafaribe/reaplet/internal/usecase"
)

// --- Fakes ---

type fakeNodeRepo struct {
	nodes []model.Node
	err   error
}

func (f *fakeNodeRepo) GetAll(_ context.Context) ([]model.Node, error) {
	return f.nodes, f.err
}

func (f *fakeNodeRepo) GetByName(_ context.Context, name string) (*model.Node, error) {
	if f.err != nil {
		return nil, f.err
	}
	for _, n := range f.nodes {
		if n.Name == name {
			return &n, nil
		}
	}
	return nil, nil
}

type fakeGCRepo struct {
	events []model.GCEvent
	err    error
}

func (f *fakeGCRepo) GetRecentEvents(_ context.Context, limit int) ([]model.GCEvent, error) {
	if f.err != nil {
		return nil, f.err
	}
	if limit >= len(f.events) {
		return f.events, nil
	}
	return f.events[:limit], nil
}

func (f *fakeGCRepo) GetEventsForNode(_ context.Context, _ string, limit int) ([]model.GCEvent, error) {
	return f.GetRecentEvents(context.Background(), limit)
}

type fakeEvictionRepo struct {
	result *model.EvictionResult
	err    error
}

func (f *fakeEvictionRepo) Evict(_ context.Context, req model.EvictionRequest) (*model.EvictionResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	return &model.EvictionResult{
		PodName:   req.PodName,
		Namespace: req.Namespace,
		Success:   true,
	}, nil
}

type fakeImageRepo struct {
	images []model.ContainerImage
	result *model.ImageRemovalResult
	err    error
}

func (f *fakeImageRepo) ListImages(_ context.Context, _ string) ([]model.ContainerImage, error) {
	return f.images, f.err
}

func (f *fakeImageRepo) RemoveImage(_ context.Context, req model.ImageRemovalRequest) (*model.ImageRemovalResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	return &model.ImageRemovalResult{
		ImageRef:   req.ImageRef,
		NodeName:   req.NodeName,
		Success:    true,
		FreedBytes: 100 * 1024 * 1024,
	}, nil
}

var errFake = fmt.Errorf("fake error")

// --- Specs ---

var _ = Describe("NodeUseCase", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("GetNodes", func() {
		It("returns all nodes", func() {
			nodes := []model.Node{
				{Name: "node-1", TotalImageSize: 1024},
				{Name: "node-2", TotalImageSize: 2048},
			}
			uc := usecase.NewNodeUseCase(&fakeNodeRepo{nodes: nodes}, &fakeGCRepo{})

			result, err := uc.GetNodes(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(2))
			Expect(result[0].Name).To(Equal("node-1"))
		})

		It("returns an error when the repository fails", func() {
			uc := usecase.NewNodeUseCase(&fakeNodeRepo{err: errFake}, &fakeGCRepo{})

			_, err := uc.GetNodes(ctx)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("GetNode", func() {
		It("returns the requested node", func() {
			nodes := []model.Node{
				{Name: "node-1"},
				{Name: "node-2"},
			}
			uc := usecase.NewNodeUseCase(&fakeNodeRepo{nodes: nodes}, &fakeGCRepo{})

			result, err := uc.GetNode(ctx, "node-2")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.Name).To(Equal("node-2"))
		})

		It("returns nil when node is not found", func() {
			uc := usecase.NewNodeUseCase(&fakeNodeRepo{nodes: []model.Node{}}, &fakeGCRepo{})

			result, err := uc.GetNode(ctx, "nonexistent")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Describe("GetRecentGCEvents", func() {
		It("returns events up to the limit", func() {
			events := []model.GCEvent{
				{Timestamp: time.Now(), Reason: "ImageGCSucceeded", Message: "freed 500MB"},
				{Timestamp: time.Now().Add(-time.Hour), Reason: "ImageGCFailed", Message: "disk full"},
			}
			uc := usecase.NewNodeUseCase(&fakeNodeRepo{}, &fakeGCRepo{events: events})

			result, err := uc.GetRecentGCEvents(ctx, 10)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(2))
		})

		It("uses default limit when 0 is passed", func() {
			uc := usecase.NewNodeUseCase(&fakeNodeRepo{}, &fakeGCRepo{events: []model.GCEvent{}})

			result, err := uc.GetRecentGCEvents(ctx, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
		})
	})

	Describe("RecommendImages", func() {
		It("recommends only unused images sorted by size descending", func() {
			nodes := []model.Node{
				{
					Name: "node-1",
					Images: []model.ContainerImage{
						{Names: []string{"nginx:latest"}, SizeBytes: 200 * 1024 * 1024, InUse: true},
						{Names: []string{"old-app:v1"}, SizeBytes: 300 * 1024 * 1024, InUse: false},
						{Names: []string{"huge-unused:latest"}, SizeBytes: 800 * 1024 * 1024, InUse: false},
					},
				},
			}
			uc := usecase.NewNodeUseCase(&fakeNodeRepo{nodes: nodes}, &fakeGCRepo{})

			recs, err := uc.RecommendImages(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(recs).To(HaveLen(2))
			Expect(recs[0].SavingsBytes).To(BeNumerically(">=", recs[1].SavingsBytes))
			Expect(recs[0].Reason).To(Equal("unused and large (>500MB)"))
		})

		It("returns empty when all images are in use", func() {
			nodes := []model.Node{
				{
					Name: "node-1",
					Images: []model.ContainerImage{
						{Names: []string{"nginx:latest"}, SizeBytes: 200 * 1024 * 1024, InUse: true},
						{Names: []string{"app:v2"}, SizeBytes: 500 * 1024 * 1024, InUse: true},
					},
				},
			}
			uc := usecase.NewNodeUseCase(&fakeNodeRepo{nodes: nodes}, &fakeGCRepo{})

			recs, err := uc.RecommendImages(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(recs).To(BeEmpty())
		})

		It("returns an error when the repository fails", func() {
			uc := usecase.NewNodeUseCase(&fakeNodeRepo{err: errFake}, &fakeGCRepo{})

			_, err := uc.RecommendImages(ctx)
			Expect(err).To(HaveOccurred())
		})

		It("enriches nameless images using Talos ListImages when imageRepo is set", func() {
			nodes := []model.Node{
				{
					Name: "node-1",
					Images: []model.ContainerImage{
						{Names: []string{"nginx:latest"}, SizeBytes: 200 * 1024 * 1024, InUse: true},
						{Names: nil, SizeBytes: 3*1024*1024*1024 + 500*1024*1024, InUse: false}, // nameless, 3.5 GB
					},
				},
			}
			// Talos returns the image by digest
			talosImages := []model.ContainerImage{
				{Names: []string{"sha256:deadbeef123456"}, SizeBytes: 3*1024*1024*1024 + 500*1024*1024},
			}
			uc := usecase.NewNodeUseCase(&fakeNodeRepo{nodes: nodes}, &fakeGCRepo{})
			uc.SetImageRepo(&fakeImageRepo{images: talosImages})

			recs, err := uc.RecommendImages(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(recs).To(HaveLen(1))
			Expect(recs[0].Image.Digest).To(Equal("sha256:deadbeef123456"))
			Expect(recs[0].Reason).To(ContainSubstring("large"))
		})

		It("skips nameless images when no Talos match is found", func() {
			nodes := []model.Node{
				{
					Name: "node-1",
					Images: []model.ContainerImage{
						{Names: nil, SizeBytes: 1024 * 1024 * 1024, InUse: false}, // 1 GB, no match
					},
				},
			}
			// Talos returns different sizes — no match
			talosImages := []model.ContainerImage{
				{Names: []string{"sha256:abc123"}, SizeBytes: 2 * 1024 * 1024 * 1024},
			}
			uc := usecase.NewNodeUseCase(&fakeNodeRepo{nodes: nodes}, &fakeGCRepo{})
			uc.SetImageRepo(&fakeImageRepo{images: talosImages})

			recs, err := uc.RecommendImages(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(recs).To(BeEmpty())
		})
	})
})

var _ = Describe("ActionUseCase", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("EvictPod", func() {
		It("successfully evicts a pod", func() {
			uc := usecase.NewActionUseCase(&fakeEvictionRepo{}, &fakeImageRepo{})

			result, err := uc.EvictPod(ctx, model.EvictionRequest{
				PodName:   "heavy-pod",
				Namespace: "default",
				NodeName:  "node-1",
				Reason:    "storage pressure",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Success).To(BeTrue())
			Expect(result.PodName).To(Equal("heavy-pod"))
		})

		It("reports failure from the eviction result", func() {
			uc := usecase.NewActionUseCase(
				&fakeEvictionRepo{result: &model.EvictionResult{
					PodName: "protected-pod",
					Success: false,
					Error:   "Cannot evict pod as it would violate the pod's disruption budget",
				}},
				&fakeImageRepo{},
			)

			result, err := uc.EvictPod(ctx, model.EvictionRequest{
				PodName:   "protected-pod",
				Namespace: "default",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Success).To(BeFalse())
		})
	})

	Describe("RemoveImage", func() {
		It("successfully removes an image", func() {
			uc := usecase.NewActionUseCase(&fakeEvictionRepo{}, &fakeImageRepo{})

			result, err := uc.RemoveImage(ctx, model.ImageRemovalRequest{
				NodeName: "node-1",
				ImageRef: "old-image:v1",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Success).To(BeTrue())
			Expect(result.FreedBytes).To(Equal(int64(100 * 1024 * 1024)))
		})

		It("reports failure from the removal result", func() {
			uc := usecase.NewActionUseCase(
				&fakeEvictionRepo{},
				&fakeImageRepo{result: &model.ImageRemovalResult{
					ImageRef: "busy-image:v1",
					Success:  false,
					Error:    "image is in use",
				}},
			)

			result, err := uc.RemoveImage(ctx, model.ImageRemovalRequest{
				NodeName: "node-1",
				ImageRef: "busy-image:v1",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Success).To(BeFalse())
		})
	})
})
