package k8s

import (
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/rafaribe/reaplet/internal/domain/model"
)

// GCEventRepository implements repository.GCEventRepository.
type GCEventRepository struct {
	client kubernetes.Interface
}

func NewGCEventRepository(client kubernetes.Interface) *GCEventRepository {
	return &GCEventRepository{client: client}
}

func (r *GCEventRepository) GetRecentEvents(ctx context.Context, limit int) ([]model.GCEvent, error) {
	events, err := r.client.CoreV1().Events("").List(ctx, metav1.ListOptions{
		FieldSelector: "reason=ImageGCFailed,reason=ImageGCSucceeded,reason=FreeDiskSpaceFailed",
	})
	if err != nil {
		// Fall back to broader search for image-related events
		events, err = r.client.CoreV1().Events("").List(ctx, metav1.ListOptions{
			FieldSelector: "involvedObject.kind=Node",
		})
		if err != nil {
			return nil, fmt.Errorf("listing GC events: %w", err)
		}
	}

	gcEvents := filterGCEvents(events.Items)

	sort.Slice(gcEvents, func(i, j int) bool {
		return gcEvents[i].Timestamp.After(gcEvents[j].Timestamp)
	})

	if len(gcEvents) > limit {
		gcEvents = gcEvents[:limit]
	}
	return gcEvents, nil
}

func (r *GCEventRepository) GetEventsForNode(ctx context.Context, nodeName string, limit int) ([]model.GCEvent, error) {
	events, err := r.client.CoreV1().Events("").List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.kind=Node,involvedObject.name=%s", nodeName),
	})
	if err != nil {
		return nil, fmt.Errorf("listing events for node %s: %w", nodeName, err)
	}

	gcEvents := filterGCEvents(events.Items)

	sort.Slice(gcEvents, func(i, j int) bool {
		return gcEvents[i].Timestamp.After(gcEvents[j].Timestamp)
	})

	if len(gcEvents) > limit {
		gcEvents = gcEvents[:limit]
	}
	return gcEvents, nil
}

func filterGCEvents(events []corev1.Event) []model.GCEvent {
	var gcEvents []model.GCEvent
	gcReasons := map[string]bool{
		"ImageGCFailed":      true,
		"ImageGCSucceeded":   true,
		"FreeDiskSpaceFailed": true,
		"NodeHasDiskPressure": true,
		"NodeHasNoDiskPressure": true,
	}

	for _, e := range events {
		if !gcReasons[e.Reason] {
			continue
		}
		ts := e.LastTimestamp.Time
		if ts.IsZero() {
			ts = e.EventTime.Time
		}
		gcEvents = append(gcEvents, model.GCEvent{
			Timestamp: ts,
			Reason:    e.Reason,
			Message:   e.Message,
		})
	}
	return gcEvents
}

// EvictionRepository implements repository.PodEvictionRepository.
type EvictionRepository struct {
	client kubernetes.Interface
}

func NewEvictionRepository(client kubernetes.Interface) *EvictionRepository {
	return &EvictionRepository{client: client}
}

func (r *EvictionRepository) Evict(ctx context.Context, req model.EvictionRequest) (*model.EvictionResult, error) {
	eviction := &policyv1.Eviction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.PodName,
			Namespace: req.Namespace,
		},
	}

	err := r.client.PolicyV1().Evictions(req.Namespace).Evict(ctx, eviction)
	if err != nil {
		return &model.EvictionResult{
			PodName:   req.PodName,
			Namespace: req.Namespace,
			Success:   false,
			Error:     err.Error(),
		}, nil
	}

	return &model.EvictionResult{
		PodName:   req.PodName,
		Namespace: req.Namespace,
		Success:   true,
	}, nil
}
