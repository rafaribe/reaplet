package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/rafaribe/reaplet/internal/infrastructure/notify"
	"github.com/rafaribe/reaplet/internal/infrastructure/storage"
)

// AlertEngine monitors storage thresholds and sends notifications.
type AlertEngine struct {
	db         *storage.DB
	nodeUC     *NodeUseCase
	mu         sync.Mutex
	nodeStates map[string]string // nodeName -> last level (ok/warning/critical)
	lastFired  map[string]time.Time
}

func NewAlertEngine(db *storage.DB, nodeUC *NodeUseCase) *AlertEngine {
	return &AlertEngine{
		db:         db,
		nodeUC:     nodeUC,
		nodeStates: make(map[string]string),
		lastFired:  make(map[string]time.Time),
	}
}

// Check evaluates all nodes against configured thresholds.
func (ae *AlertEngine) Check(ctx context.Context) {
	cfg, err := ae.db.GetAlertConfig()
	if err != nil {
		slog.Error("alert engine: failed to get config", "error", err)
		return
	}

	nodes, err := ae.nodeUC.GetNodes(ctx)
	if err != nil {
		slog.Error("alert engine: failed to get nodes", "error", err)
		return
	}

	ae.mu.Lock()
	defer ae.mu.Unlock()

	for _, node := range nodes {
		if node.EphemeralStorage.CapacityBytes == 0 {
			continue
		}

		usagePct := float64(node.EphemeralStorage.AllocatedBytes) / float64(node.EphemeralStorage.CapacityBytes) * 100

		// Determine thresholds (per-node override or global)
		warningPct := cfg.WarningPct
		criticalPct := cfg.CriticalPct
		if override, ok := cfg.NodeOverrides[node.Name]; ok {
			warningPct = override.WarningPct
			criticalPct = override.CriticalPct
		}

		// Determine current level
		var level string
		switch {
		case usagePct >= float64(criticalPct):
			level = "critical"
		case usagePct >= float64(warningPct):
			level = "warning"
		default:
			level = "ok"
		}

		prevLevel := ae.nodeStates[node.Name]

		// Only fire on state transitions
		if level == prevLevel {
			continue
		}

		// Check cooldown
		cooldown := time.Duration(cfg.CooldownMin) * time.Minute
		if lastFire, ok := ae.lastFired[node.Name]; ok && time.Since(lastFire) < cooldown {
			continue
		}

		ae.nodeStates[node.Name] = level

		// Don't notify on ok→ok
		if level == "ok" && prevLevel == "" {
			continue
		}

		// Build message
		var message string
		if level == "ok" {
			message = fmt.Sprintf("Node %s storage resolved: %.1f%% used", node.Name, usagePct)
		} else {
			message = fmt.Sprintf("Node %s at %.1f%% storage (threshold: %d%%)", node.Name, usagePct, map[string]int{"warning": warningPct, "critical": criticalPct}[level])
		}

		// Resolved uses "resolved" as level for notification
		notifyLevel := level
		if level == "ok" {
			notifyLevel = "resolved"
		}

		// Record event
		if err := ae.db.RecordAlertEvent(node.Name, notifyLevel, usagePct, message); err != nil {
			slog.Error("alert engine: failed to record event", "error", err)
		}

		// Send notifications
		ae.notify(cfg, notifyLevel, node.Name, usagePct, message)
		ae.lastFired[node.Name] = time.Now()
	}
}

func (ae *AlertEngine) notify(cfg *storage.AlertConfig, level, nodeName string, usagePct float64, message string) {
	if cfg.Discord.Enabled && cfg.Discord.WebhookURL != "" {
		d := notify.NewDiscord(cfg.Discord.WebhookURL)
		if err := d.Send(level, nodeName, usagePct, message); err != nil {
			slog.Error("alert: discord notification failed", "error", err)
		}
	}

	if cfg.Pushover.Enabled && cfg.Pushover.AppToken != "" {
		p := notify.NewPushover(cfg.Pushover.AppToken, cfg.Pushover.UserKey)
		if err := p.Send(level, nodeName, usagePct, message); err != nil {
			slog.Error("alert: pushover notification failed", "error", err)
		}
	}
}

// SendTest sends a test notification to all configured channels.
func (ae *AlertEngine) SendTest(ctx context.Context) error {
	cfg, err := ae.db.GetAlertConfig()
	if err != nil {
		return err
	}

	ae.notify(cfg, "warning", "test-node", 85.0, "This is a test alert from Reaplet")
	return nil
}
