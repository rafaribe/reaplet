package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
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

// --- Image Deduplication ---

// GetImageDeduplication finds images with the same digest present on multiple nodes.
func GetImageDeduplication(ctx context.Context, nodeRepo repository.NodeRepository) ([]model.ImageDuplicateGroup, error) {
	nodes, err := nodeRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	// Group images by their digest-like identifier (sha256 portion of image name)
	type imageInfo struct {
		names map[string]bool
		nodes map[string]bool
		size  int64
	}
	digestMap := make(map[string]*imageInfo)

	for _, node := range nodes {
		for _, img := range node.Images {
			for _, name := range img.Names {
				// Extract digest from image name (format: repo@sha256:xxx or sha256:xxx)
				digest := extractDigest(name)
				if digest == "" {
					continue
				}

				info, ok := digestMap[digest]
				if !ok {
					info = &imageInfo{
						names: make(map[string]bool),
						nodes: make(map[string]bool),
						size:  img.SizeBytes,
					}
					digestMap[digest] = info
				}
				info.names[name] = true
				info.nodes[node.Name] = true
			}
		}
	}

	// Build results — only include groups with multiple names or multiple nodes
	var groups []model.ImageDuplicateGroup
	for digest, info := range digestMap {
		if len(info.names) <= 1 && len(info.nodes) <= 1 {
			continue
		}

		names := make([]string, 0, len(info.names))
		for n := range info.names {
			names = append(names, n)
		}
		nodeList := make([]string, 0, len(info.nodes))
		for n := range info.nodes {
			nodeList = append(nodeList, n)
		}

		// Wasted = size * (nodeCount - 1) for cross-node duplication
		wastedBytes := info.size * int64(len(info.nodes)-1)

		groups = append(groups, model.ImageDuplicateGroup{
			Digest:      digest,
			Names:       names,
			Nodes:       nodeList,
			SizeBytes:   info.size,
			WastedBytes: wastedBytes,
		})
	}

	// Sort by wasted bytes descending
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].WastedBytes > groups[j].WastedBytes
	})

	return groups, nil
}

// extractDigest extracts the sha256 digest from an image reference.
func extractDigest(imageRef string) string {
	// Look for sha256: in the image name
	idx := len(imageRef)
	for i := 0; i < len(imageRef)-7; i++ {
		if imageRef[i:i+7] == "sha256:" {
			idx = i
			break
		}
	}
	if idx < len(imageRef) {
		return imageRef[idx:]
	}
	return ""
}

// --- Storage Forecast ---

const (
	forecastWarningPct  = 80.0
	forecastCriticalPct = 90.0
)

// GetStorageForecast performs linear regression on storage history to project threshold dates.
func GetStorageForecast(db *storage.DB, nodeRepo repository.NodeRepository, ctx context.Context, nodeName string) (*model.StorageForecast, error) {
	// Get current node state
	node, err := nodeRepo.GetByName(ctx, nodeName)
	if err != nil {
		return nil, fmt.Errorf("getting node %s: %w", nodeName, err)
	}

	capacity := node.EphemeralStorage.CapacityBytes
	allocated := node.EphemeralStorage.AllocatedBytes
	var currentPct float64
	if capacity > 0 {
		currentPct = float64(allocated) / float64(capacity) * 100
	}

	// Get 7 days of history
	points, err := db.GetHistoryForForecast(nodeName, 7)
	if err != nil {
		return nil, fmt.Errorf("getting history: %w", err)
	}

	forecast := &model.StorageForecast{
		NodeName:                nodeName,
		CurrentPct:              currentPct,
		ProjectedDaysToWarning:  -1, // -1 means "won't reach threshold"
		ProjectedDaysToCritical: -1,
		TrendBytesPerDay:        0,
	}

	if len(points) < 2 {
		return forecast, nil
	}

	// Linear regression: y = allocated_bytes, x = time in days
	trendPerDay := linearRegressionSlope(points)
	forecast.TrendBytesPerDay = int64(trendPerDay)

	if trendPerDay <= 0 {
		// Storage is stable or decreasing — won't hit thresholds
		return forecast, nil
	}

	// Project days to warning
	warningBytes := float64(capacity) * forecastWarningPct / 100
	if float64(allocated) < warningBytes {
		daysToWarning := (warningBytes - float64(allocated)) / trendPerDay
		forecast.ProjectedDaysToWarning = daysToWarning
	} else {
		forecast.ProjectedDaysToWarning = 0 // already past warning
	}

	// Project days to critical
	criticalBytes := float64(capacity) * forecastCriticalPct / 100
	if float64(allocated) < criticalBytes {
		daysToCritical := (criticalBytes - float64(allocated)) / trendPerDay
		forecast.ProjectedDaysToCritical = daysToCritical
	} else {
		forecast.ProjectedDaysToCritical = 0 // already past critical
	}

	return forecast, nil
}

// linearRegressionSlope computes the slope (bytes per day) from history points.
func linearRegressionSlope(points []storage.HistoryPoint) float64 {
	n := float64(len(points))
	if n < 2 {
		return 0
	}

	// Use time since first point in days as X, allocated_bytes as Y
	baseTime := points[0].Timestamp
	var sumX, sumY, sumXY, sumX2 float64

	for _, p := range points {
		x := p.Timestamp.Sub(baseTime).Hours() / 24.0
		y := float64(p.AllocatedBytes)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0
	}

	slope := (n*sumXY - sumX*sumY) / denominator
	return slope
}

// --- Warm List ---

// WarmListUseCase manages the warm list and checks node coverage.
type WarmListUseCase struct {
	db       *storage.DB
	nodeRepo repository.NodeRepository
}

func NewWarmListUseCase(db *storage.DB, nodeRepo repository.NodeRepository) *WarmListUseCase {
	return &WarmListUseCase{db: db, nodeRepo: nodeRepo}
}

// GetStatus returns the warm list with info about which nodes are missing which images.
func (uc *WarmListUseCase) GetStatus(ctx context.Context) (*model.WarmListStatus, error) {
	entries, err := uc.db.GetWarmList()
	if err != nil {
		return nil, err
	}

	nodes, err := uc.nodeRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	// Build set of images per node
	nodeImages := make(map[string]map[string]bool)
	for _, node := range nodes {
		imgSet := make(map[string]bool)
		for _, img := range node.Images {
			for _, name := range img.Names {
				imgSet[name] = true
			}
		}
		nodeImages[node.Name] = imgSet
	}

	// Check which warm list images are missing from which nodes
	missingOnNodes := make(map[string][]string)
	domainEntries := make([]model.WarmListEntry, 0, len(entries))

	for _, e := range entries {
		domainEntries = append(domainEntries, model.WarmListEntry{
			ID:       e.ID,
			ImageRef: e.ImageRef,
			AddedAt:  e.AddedAt,
		})

		for _, node := range nodes {
			if !nodeImages[node.Name][e.ImageRef] {
				missingOnNodes[e.ImageRef] = append(missingOnNodes[e.ImageRef], node.Name)
			}
		}
	}

	return &model.WarmListStatus{
		Entries:        domainEntries,
		MissingOnNodes: missingOnNodes,
	}, nil
}

// Add adds an image to the warm list.
func (uc *WarmListUseCase) Add(imageRef string) (*model.WarmListEntry, error) {
	entry, err := uc.db.AddWarmListEntry(imageRef)
	if err != nil {
		return nil, err
	}
	return &model.WarmListEntry{
		ID:       entry.ID,
		ImageRef: entry.ImageRef,
		AddedAt:  entry.AddedAt,
	}, nil
}

// Delete removes an image from the warm list.
func (uc *WarmListUseCase) Delete(id int64) error {
	return uc.db.DeleteWarmListEntry(id)
}

// --- Pre-Warm Check ---

// PreWarmCheck verifies if an image exists on a node.
func PreWarmCheck(ctx context.Context, nodeRepo repository.NodeRepository, req model.PreWarmCheckRequest) (*model.PreWarmCheckResult, error) {
	node, err := nodeRepo.GetByName(ctx, req.NodeName)
	if err != nil {
		return nil, fmt.Errorf("getting node %s: %w", req.NodeName, err)
	}

	result := &model.PreWarmCheckResult{
		Exists:    false,
		SizeBytes: 0,
		CanPull:   true, // Assume can pull unless proven otherwise
	}

	for _, img := range node.Images {
		for _, name := range img.Names {
			if name == req.ImageRef {
				result.Exists = true
				result.SizeBytes = img.SizeBytes
				return result, nil
			}
		}
	}

	return result, nil
}

// --- Upgrade Check ---

const talosUpgradeEstimatedBytes = 2 * 1024 * 1024 * 1024 // 2 GB

// GetUpgradeCheck estimates if nodes have enough free space for a Talos upgrade.
func GetUpgradeCheck(ctx context.Context, nodeRepo repository.NodeRepository) ([]model.UpgradeCheckResult, error) {
	nodes, err := nodeRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	var results []model.UpgradeCheckResult
	for _, node := range nodes {
		available := node.EphemeralStorage.AvailableBytes
		safe := available >= talosUpgradeEstimatedBytes
		message := "sufficient space for Talos upgrade"
		if !safe {
			deficit := talosUpgradeEstimatedBytes - available
			message = fmt.Sprintf("insufficient space: need ~2GB, only %dMB available (deficit: %dMB)",
				available/(1024*1024), deficit/(1024*1024))
		}

		results = append(results, model.UpgradeCheckResult{
			NodeName:             node.Name,
			AvailableBytes:       available,
			EstimatedNeededBytes: talosUpgradeEstimatedBytes,
			Safe:                 safe,
			Message:              message,
		})
	}

	return results, nil
}
