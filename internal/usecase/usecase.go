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
	nodeRepo     repository.NodeRepository
	gcRepo       repository.GCEventRepository
	imageAgeRepo repository.ImageAgeRepository  // optional, nil disables staleness
	imageRepo    repository.ImageRepository     // optional, nil disables Talos enrichment
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

// SetImageRepo sets an optional image repository for Talos-based image enrichment.
// When set, images without names are enriched with references from the Talos API.
func (uc *NodeUseCase) SetImageRepo(repo repository.ImageRepository) {
	uc.imageRepo = repo
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
// When an ImageRepository is configured, images without names are enriched
// with references from the Talos API to enable their removal.
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

		// Check if this node has nameless images that need Talos enrichment
		var talosRefsBySize map[int64][]string
		if uc.imageRepo != nil {
			hasNameless := false
			for _, img := range node.Images {
				if !img.InUse && len(img.Names) == 0 && img.Digest == "" {
					hasNameless = true
					break
				}
			}
			if hasNameless {
				talosRefsBySize = uc.buildTalosRefMap(ctx, node.Name)
			}
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
			} else if talosRefsBySize != nil {
				// Try to find a Talos reference by matching size
				if refs, ok := talosRefsBySize[img.SizeBytes]; ok && len(refs) > 0 {
					imageRef = refs[0]
					img.Digest = imageRef
					// Consume this ref so it's not reused for another image
					talosRefsBySize[img.SizeBytes] = refs[1:]
				}
			}

			if imageRef == "" {
				// Still no reference — skip
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

// buildTalosRefMap calls Talos ListImages for a node and returns a map of
// size → []imageRef for images that are NOT already known by the K8s API.
// This enables matching nameless K8s images to their Talos CRI reference.
func (uc *NodeUseCase) buildTalosRefMap(ctx context.Context, nodeName string) map[int64][]string {
	talosImages, err := uc.imageRepo.ListImages(ctx, nodeName)
	if err != nil {
		return nil
	}

	// Build a map: size → list of Talos image references
	// Only include images whose reference looks like a digest (sha256:...)
	// since named images are already handled by the K8s API path
	refsBySize := make(map[int64][]string)
	for _, ti := range talosImages {
		if len(ti.Names) > 0 {
			ref := ti.Names[0]
			// Only include digest-style refs (sha256:...) — named images are
			// already available from the K8s API
			if len(ref) > 7 && ref[:7] == "sha256:" {
				refsBySize[ti.SizeBytes] = append(refsBySize[ti.SizeBytes], ref)
			}
		}
	}
	return refsBySize
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
