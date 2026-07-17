package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/rafaribe/reaplet/internal/domain/model"
	"github.com/rafaribe/reaplet/internal/domain/repository"
	"github.com/rafaribe/reaplet/internal/infrastructure/storage"
)

// HistoryRecorder periodically records storage metrics.
type HistoryRecorder struct {
	db     *storage.DB
	nodeUC *NodeUseCase
}

func NewHistoryRecorder(db *storage.DB, nodeUC *NodeUseCase) *HistoryRecorder {
	return &HistoryRecorder{db: db, nodeUC: nodeUC}
}

// Record captures current storage state for all nodes.
func (hr *HistoryRecorder) Record(ctx context.Context) {
	nodes, err := hr.nodeUC.GetNodes(ctx)
	if err != nil {
		slog.Error("history recorder: failed to get nodes", "error", err)
		return
	}

	for _, node := range nodes {
		if err := hr.db.RecordHistory(
			node.Name,
			node.EphemeralStorage.CapacityBytes,
			node.EphemeralStorage.AllocatedBytes,
			node.TotalImageSize,
		); err != nil {
			slog.Error("history recorder: failed to record", "node", node.Name, "error", err)
		}

		// Track image ages
		for _, img := range node.Images {
			if len(img.Names) > 0 {
				_ = hr.db.RecordImageSeen(node.Name, img.Names[0])
			}
		}
	}

	// Prune old data (keep 7 days)
	if err := hr.db.PruneHistory(7 * 24 * time.Hour); err != nil {
		slog.Error("history recorder: prune failed", "error", err)
	}
}

// GetHistory returns history for a node within a time range.
func (hr *HistoryRecorder) GetHistory(nodeName string, rangeStr string) ([]storage.HistoryPoint, error) {
	duration := 24 * time.Hour
	switch rangeStr {
	case "1h":
		duration = time.Hour
	case "24h":
		duration = 24 * time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	}
	return hr.db.GetHistory(nodeName, time.Now().Add(-duration))
}

// CleanupEngine handles scheduled automatic image removal.
type CleanupEngine struct {
	db       *storage.DB
	actionUC *ActionUseCase
	nodeUC   *NodeUseCase
}

func NewCleanupEngine(db *storage.DB, nodeUC *NodeUseCase, actionUC *ActionUseCase) *CleanupEngine {
	return &CleanupEngine{db: db, nodeUC: nodeUC, actionUC: actionUC}
}

// CleanupResult reports what was (or would be) removed.
type CleanupResult struct {
	Removed []CleanupAction `json:"removed"`
	Skipped []CleanupAction `json:"skipped"`
	DryRun  bool            `json:"dryRun"`
}

type CleanupAction struct {
	NodeName string `json:"nodeName"`
	ImageRef string `json:"imageRef"`
	Reason   string `json:"reason"`
	SizeMB   int64  `json:"sizeMB"`
}

// Run executes the cleanup policy.
func (ce *CleanupEngine) Run(ctx context.Context) (*CleanupResult, error) {
	cfg, err := ce.db.GetCleanupConfig()
	if err != nil {
		return nil, fmt.Errorf("getting cleanup config: %w", err)
	}

	if !cfg.Enabled {
		return &CleanupResult{DryRun: true}, nil
	}

	nodes, err := ce.nodeUC.GetNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting nodes: %w", err)
	}

	// Compile keep patterns
	var keepRegexps []*regexp.Regexp
	for _, pattern := range cfg.KeepPatterns {
		if r, err := regexp.Compile(pattern); err == nil {
			keepRegexps = append(keepRegexps, r)
		}
	}

	result := &CleanupResult{DryRun: cfg.DryRun}
	removed := 0

	for _, node := range nodes {
		ages, _ := ce.db.GetAllImageAges(node.Name)

		for _, img := range node.Images {
			if img.InUse {
				continue
			}
			if removed >= cfg.MaxPerCycle {
				break
			}

			imageRef := ""
			if len(img.Names) > 0 {
				imageRef = img.Names[0]
			}
			if imageRef == "" {
				continue
			}

			// Check keep patterns
			kept := false
			for _, re := range keepRegexps {
				if re.MatchString(imageRef) {
					kept = true
					break
				}
			}
			if kept {
				result.Skipped = append(result.Skipped, CleanupAction{
					NodeName: node.Name, ImageRef: imageRef, Reason: "matches keep pattern", SizeMB: img.SizeBytes / (1024 * 1024),
				})
				continue
			}

			// Check age
			var reason string
			if firstSeen, ok := ages[imageRef]; ok {
				age := time.Since(firstSeen)
				if age > time.Duration(cfg.MaxAgeDays)*24*time.Hour {
					reason = fmt.Sprintf("unused for %d days", int(age.Hours()/24))
				}
			}

			// Check size
			sizeMB := img.SizeBytes / (1024 * 1024)
			if reason == "" && sizeMB > int64(cfg.MaxSizeMB) {
				reason = fmt.Sprintf("large image (%d MB)", sizeMB)
			}

			if reason == "" {
				continue
			}

			action := CleanupAction{NodeName: node.Name, ImageRef: imageRef, Reason: reason, SizeMB: sizeMB}

			if cfg.DryRun {
				result.Removed = append(result.Removed, action)
			} else {
				_, err := ce.actionUC.RemoveImage(ctx, model.ImageRemovalRequest{
					NodeName: node.Name,
					ImageRef: imageRef,
				})
				if err != nil {
					slog.Error("cleanup: removal failed", "node", node.Name, "image", imageRef, "error", err)
					continue
				}
				result.Removed = append(result.Removed, action)
				removed++
			}
		}
	}

	return result, nil
}

// ClusterSummary provides a cluster-wide storage overview.
type ClusterSummary struct {
	TotalNodes       int                `json:"totalNodes"`
	TotalCapacity    int64              `json:"totalCapacity"`
	TotalAllocated   int64              `json:"totalAllocated"`
	TotalImages      int                `json:"totalImages"`
	TotalUnused      int                `json:"totalUnused"`
	ReclaimableBytes int64              `json:"reclaimableBytes"`
	Nodes            []NodeSummaryEntry `json:"nodes"`
}

type NodeSummaryEntry struct {
	Name        string  `json:"name"`
	UsagePct    float64 `json:"usagePct"`
	ImageCount  int     `json:"imageCount"`
	UnusedCount int     `json:"unusedCount"`
	Reclaimable int64   `json:"reclaimable"`
}

// GetClusterSummary builds a cluster-wide overview.
func GetClusterSummary(ctx context.Context, nodeRepo repository.NodeRepository) (*ClusterSummary, error) {
	nodes, err := nodeRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	summary := &ClusterSummary{TotalNodes: len(nodes)}

	for _, node := range nodes {
		summary.TotalCapacity += node.EphemeralStorage.CapacityBytes
		summary.TotalAllocated += node.EphemeralStorage.AllocatedBytes
		summary.TotalImages += len(node.Images)

		var unused int
		var reclaimable int64
		for _, img := range node.Images {
			if !img.InUse {
				unused++
				reclaimable += img.SizeBytes
			}
		}
		summary.TotalUnused += unused
		summary.ReclaimableBytes += reclaimable

		var usagePct float64
		if node.EphemeralStorage.CapacityBytes > 0 {
			usagePct = float64(node.EphemeralStorage.AllocatedBytes) / float64(node.EphemeralStorage.CapacityBytes) * 100
		}

		summary.Nodes = append(summary.Nodes, NodeSummaryEntry{
			Name:        node.Name,
			UsagePct:    usagePct,
			ImageCount:  len(node.Images),
			UnusedCount: unused,
			Reclaimable: reclaimable,
		})
	}

	return summary, nil
}
