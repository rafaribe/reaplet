package k8s

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFilterGCEvents(t *testing.T) {
	now := time.Now()
	events := []corev1.Event{
		{
			Reason:        "ImageGCSucceeded",
			Message:       "freed 200MB of images",
			LastTimestamp: metav1.Time{Time: now},
		},
		{
			Reason:        "ImageGCFailed",
			Message:       "disk full, cannot GC",
			LastTimestamp: metav1.Time{Time: now.Add(-time.Hour)},
		},
		{
			Reason:        "NodeHasDiskPressure",
			Message:       "node has disk pressure",
			LastTimestamp: metav1.Time{Time: now.Add(-2 * time.Hour)},
		},
		{
			Reason:        "Pulling",
			Message:       "pulling image nginx:latest",
			LastTimestamp: metav1.Time{Time: now.Add(-3 * time.Hour)},
		},
		{
			Reason:        "Scheduled",
			Message:       "pod scheduled",
			LastTimestamp: metav1.Time{Time: now.Add(-4 * time.Hour)},
		},
	}

	result := filterGCEvents(events)

	// Should only include GC-related events (3 out of 5)
	if len(result) != 3 {
		t.Fatalf("expected 3 GC events, got %d", len(result))
	}

	// Check all are GC-related
	for _, e := range result {
		switch e.Reason {
		case "ImageGCSucceeded", "ImageGCFailed", "NodeHasDiskPressure":
			// ok
		default:
			t.Errorf("unexpected reason in filtered results: %s", e.Reason)
		}
	}
}

func TestFilterGCEvents_Empty(t *testing.T) {
	result := filterGCEvents([]corev1.Event{})
	if len(result) != 0 {
		t.Errorf("expected 0 events, got %d", len(result))
	}
}

func TestFilterGCEvents_NoGCEvents(t *testing.T) {
	events := []corev1.Event{
		{Reason: "Pulling", Message: "pulling image"},
		{Reason: "Created", Message: "created container"},
		{Reason: "Started", Message: "started container"},
	}
	result := filterGCEvents(events)
	if len(result) != 0 {
		t.Errorf("expected 0 GC events, got %d", len(result))
	}
}

func TestFilterGCEvents_UsesEventTimeWhenLastTimestampZero(t *testing.T) {
	eventTime := time.Now()
	events := []corev1.Event{
		{
			Reason:    "NodeHasNoDiskPressure",
			Message:   "node no longer has disk pressure",
			EventTime: metav1.MicroTime{Time: eventTime},
			// LastTimestamp is zero
		},
	}

	result := filterGCEvents(events)
	if len(result) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result))
	}
	if !result[0].Timestamp.Equal(eventTime) {
		t.Errorf("expected timestamp %v, got %v", eventTime, result[0].Timestamp)
	}
}
