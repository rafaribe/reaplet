package usecase

import (
	"context"
	"sort"

	"github.com/rafaribe/reaplet/internal/domain/model"
	"github.com/rafaribe/reaplet/internal/domain/repository"
)

// NodeUseCase handles node storage monitoring logic.
type NodeUseCase struct {
	nodeRepo repository.NodeRepository
	gcRepo   repository.GCEventRepository
}

func NewNodeUseCase(nodeRepo repository.NodeRepository, gcRepo repository.GCEventRepository) *NodeUseCase {
	return &NodeUseCase{
		nodeRepo: nodeRepo,
		gcRepo:   gcRepo,
	}
}

func (uc *NodeUseCase) GetNodes(ctx context.Context) ([]model.Node, error) {
	return uc.nodeRepo.GetAll(ctx)
}

func (uc *NodeUseCase) GetNode(ctx context.Context, name string) (*model.Node, error) {
	return uc.nodeRepo.GetByName(ctx, name)
}

func (uc *NodeUseCase) GetRecentGCEvents(ctx context.Context, limit int) ([]model.GCEvent, error) {
	if limit <= 0 {
		limit = 20
	}
	return uc.gcRepo.GetRecentEvents(ctx, limit)
}

// RecommendImages identifies images that are candidates for removal.
func (uc *NodeUseCase) RecommendImages(ctx context.Context) ([]model.ImageRecommendation, error) {
	nodes, err := uc.nodeRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	var recommendations []model.ImageRecommendation

	for _, node := range nodes {
		for _, img := range node.Images {
			if img.InUse {
				continue
			}
			reason := "unused by any running pod"
			if img.SizeBytes > 500*1024*1024 { // > 500MB
				reason = "unused and large (>500MB)"
			}
			recommendations = append(recommendations, model.ImageRecommendation{
				Image:        img,
				NodeName:     node.Name,
				Reason:       reason,
				SavingsBytes: img.SizeBytes,
			})
		}
	}

	// Sort by size descending — biggest savings first
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].SavingsBytes > recommendations[j].SavingsBytes
	})

	return recommendations, nil
}

// ActionUseCase handles destructive operations (eviction, image removal).
type ActionUseCase struct {
	evictionRepo repository.PodEvictionRepository
	imageRepo    repository.ImageRepository
}

func NewActionUseCase(evictionRepo repository.PodEvictionRepository, imageRepo repository.ImageRepository) *ActionUseCase {
	return &ActionUseCase{
		evictionRepo: evictionRepo,
		imageRepo:    imageRepo,
	}
}

func (uc *ActionUseCase) EvictPod(ctx context.Context, req model.EvictionRequest) (*model.EvictionResult, error) {
	return uc.evictionRepo.Evict(ctx, req)
}

func (uc *ActionUseCase) RemoveImage(ctx context.Context, req model.ImageRemovalRequest) (*model.ImageRemovalResult, error) {
	return uc.imageRepo.RemoveImage(ctx, req)
}
