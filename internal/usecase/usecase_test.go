package usecase_test

import (
	"context"
	"fmt"
	"testing"
	"time"

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

// --- NodeUseCase Tests ---

func TestNodeUseCase_GetNodes(t *testing.T) {
	nodes := []model.Node{
		{Name: "node-1", TotalImageSize: 1024},
		{Name: "node-2", TotalImageSize: 2048},
	}
	uc := usecase.NewNodeUseCase(&fakeNodeRepo{nodes: nodes}, &fakeGCRepo{})

	result, err := uc.GetNodes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(result))
	}
	if result[0].Name != "node-1" {
		t.Errorf("expected node-1, got %s", result[0].Name)
	}
}

func TestNodeUseCase_GetNodes_Error(t *testing.T) {
	uc := usecase.NewNodeUseCase(
		&fakeNodeRepo{err: errFake},
		&fakeGCRepo{},
	)

	_, err := uc.GetNodes(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNodeUseCase_GetNode(t *testing.T) {
	nodes := []model.Node{
		{Name: "node-1"},
		{Name: "node-2"},
	}
	uc := usecase.NewNodeUseCase(&fakeNodeRepo{nodes: nodes}, &fakeGCRepo{})

	result, err := uc.GetNode(context.Background(), "node-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected node, got nil")
	}
	if result.Name != "node-2" {
		t.Errorf("expected node-2, got %s", result.Name)
	}
}

func TestNodeUseCase_GetNode_NotFound(t *testing.T) {
	uc := usecase.NewNodeUseCase(&fakeNodeRepo{nodes: []model.Node{}}, &fakeGCRepo{})

	result, err := uc.GetNode(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got %+v", result)
	}
}

func TestNodeUseCase_GetRecentGCEvents(t *testing.T) {
	events := []model.GCEvent{
		{Timestamp: time.Now(), Reason: "ImageGCSucceeded", Message: "freed 500MB"},
		{Timestamp: time.Now().Add(-time.Hour), Reason: "ImageGCFailed", Message: "disk full"},
	}
	uc := usecase.NewNodeUseCase(&fakeNodeRepo{}, &fakeGCRepo{events: events})

	result, err := uc.GetRecentGCEvents(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 events, got %d", len(result))
	}
}

func TestNodeUseCase_GetRecentGCEvents_DefaultLimit(t *testing.T) {
	uc := usecase.NewNodeUseCase(&fakeNodeRepo{}, &fakeGCRepo{events: []model.GCEvent{}})

	// Should use default limit of 20 when 0 is passed
	result, err := uc.GetRecentGCEvents(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestNodeUseCase_RecommendImages(t *testing.T) {
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

	recs, err := uc.RecommendImages(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should only recommend unused images (2 out of 3)
	if len(recs) != 2 {
		t.Fatalf("expected 2 recommendations, got %d", len(recs))
	}
	// Should be sorted by size descending
	if recs[0].SavingsBytes < recs[1].SavingsBytes {
		t.Error("expected recommendations sorted by size descending")
	}
	// Large unused image should have special reason
	if recs[0].Reason != "unused and large (>500MB)" {
		t.Errorf("expected 'unused and large' reason, got %q", recs[0].Reason)
	}
}

func TestNodeUseCase_RecommendImages_AllInUse(t *testing.T) {
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

	recs, err := uc.RecommendImages(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("expected 0 recommendations, got %d", len(recs))
	}
}

func TestNodeUseCase_RecommendImages_Error(t *testing.T) {
	uc := usecase.NewNodeUseCase(&fakeNodeRepo{err: errFake}, &fakeGCRepo{})

	_, err := uc.RecommendImages(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- ActionUseCase Tests ---

func TestActionUseCase_EvictPod(t *testing.T) {
	uc := usecase.NewActionUseCase(&fakeEvictionRepo{}, &fakeImageRepo{})

	result, err := uc.EvictPod(context.Background(), model.EvictionRequest{
		PodName:   "heavy-pod",
		Namespace: "default",
		NodeName:  "node-1",
		Reason:    "storage pressure",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got failure: %s", result.Error)
	}
	if result.PodName != "heavy-pod" {
		t.Errorf("expected heavy-pod, got %s", result.PodName)
	}
}

func TestActionUseCase_EvictPod_Failure(t *testing.T) {
	uc := usecase.NewActionUseCase(
		&fakeEvictionRepo{result: &model.EvictionResult{
			PodName: "protected-pod",
			Success: false,
			Error:   "Cannot evict pod as it would violate the pod's disruption budget",
		}},
		&fakeImageRepo{},
	)

	result, err := uc.EvictPod(context.Background(), model.EvictionRequest{
		PodName:   "protected-pod",
		Namespace: "default",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure, got success")
	}
}

func TestActionUseCase_RemoveImage(t *testing.T) {
	uc := usecase.NewActionUseCase(&fakeEvictionRepo{}, &fakeImageRepo{})

	result, err := uc.RemoveImage(context.Background(), model.ImageRemovalRequest{
		NodeName: "node-1",
		ImageRef: "old-image:v1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got failure: %s", result.Error)
	}
	if result.FreedBytes != 100*1024*1024 {
		t.Errorf("expected 100MB freed, got %d", result.FreedBytes)
	}
}

func TestActionUseCase_RemoveImage_Failure(t *testing.T) {
	uc := usecase.NewActionUseCase(
		&fakeEvictionRepo{},
		&fakeImageRepo{result: &model.ImageRemovalResult{
			ImageRef: "busy-image:v1",
			Success:  false,
			Error:    "image is in use",
		}},
	)

	result, err := uc.RemoveImage(context.Background(), model.ImageRemovalRequest{
		NodeName: "node-1",
		ImageRef: "busy-image:v1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure, got success")
	}
}

var errFake = fmt.Errorf("fake error")
