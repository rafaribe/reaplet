package usecase

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/rafaribe/reaplet/internal/domain/model"
	"github.com/rafaribe/reaplet/internal/domain/repository"
)

// NodeUseCase handles node storage monitoring logic.
type NodeUseCase struct {
	nodeRepo    repository.NodeRepository
	gcRepo      repository.GCEventRepository
	imageAgeRepo repository.ImageAgeRepository // optional, nil disables staleness
}

func NewNodeUseCase(nodeRepo repository.NodeRepository, gcRepo repository.GCEventRepository) *NodeUseCase {
	return &NodeUseCase{
		nodeRepo: nodeRepo,
		gcRepo:   gcRepo,
	}
}

// SetImageAgeRepo sets an optional image age repository for staleness-aware recommendations.
func (uc *NodeUseCase) SetImageAgeRepo(repo repository.ImageAgeRepository) {
	uc.imageAgeRepo = repo
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
// When an ImageAgeRepository is configured, recommendations include staleness
// information and are sorted by staleness (oldest unused first).
func (uc *NodeUseCase) RecommendImages(ctx context.Context) ([]model.ImageRecommendation, error) {
	nodes, err := uc.nodeRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	var recommendations []model.ImageRecommendation

	for _, node := range nodes {
		// Load image ages if available
		var ages map[string]time.Time
		if uc.imageAgeRepo != nil {
			ages, _ = uc.imageAgeRepo.GetAllImageAges(node.Name)
		}

		for _, img := range node.Images {
			if img.InUse {
				continue
			}

			imageRef := ""
			if len(img.Names) > 0 {
				imageRef = img.Names[0]
			} else if img.Digest != "" {
				// Digest-only image (untagged) — use digest as reference
				imageRef = img.Digest
			} else {
				// No name and no digest — skip, can't be referenced for removal
				continue
			}

			reason := "unused by any running pod"
			var unusedDays int

			// Check staleness
			if ages != nil && imageRef != "" {
				if firstSeen, ok := ages[imageRef]; ok {
					unusedDays = int(time.Since(firstSeen).Hours() / 24)
					if unusedDays >= 30 {
						reason = fmt.Sprintf("unused for %d days (>30d)", unusedDays)
					} else if unusedDays >= 7 {
						reason = fmt.Sprintf("unused for %d days (>7d)", unusedDays)
					} else if unusedDays > 0 {
						reason = fmt.Sprintf("unused for %d days", unusedDays)
					}
				}
			}

			if img.SizeBytes > 500*1024*1024 {
				if unusedDays > 0 {
					reason += ", large (>500MB)"
				} else {
					reason = "unused and large (>500MB)"
				}
			}

			recommendations = append(recommendations, model.ImageRecommendation{
				Image:        img,
				NodeName:     node.Name,
				Reason:       reason,
				SavingsBytes: img.SizeBytes,
				UnusedDays:   unusedDays,
			})
		}
	}

	// Sort by staleness first (oldest unused), then by size
	sort.Slice(recommendations, func(i, j int) bool {
		if recommendations[i].UnusedDays != recommendations[j].UnusedDays {
			return recommendations[i].UnusedDays > recommendations[j].UnusedDays
		}
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
