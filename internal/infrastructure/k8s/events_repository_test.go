package k8s

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("filterGCEvents", func() {
	It("filters to only GC-related events", func() {
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

		Expect(result).To(HaveLen(3))
		for _, e := range result {
			Expect(e.Reason).To(BeElementOf("ImageGCSucceeded", "ImageGCFailed", "NodeHasDiskPressure"))
		}
	})

	It("returns empty for empty input", func() {
		result := filterGCEvents([]corev1.Event{})
		Expect(result).To(BeEmpty())
	})

	It("returns empty when no GC events exist", func() {
		events := []corev1.Event{
			{Reason: "Pulling", Message: "pulling image"},
			{Reason: "Created", Message: "created container"},
			{Reason: "Started", Message: "started container"},
		}
		result := filterGCEvents(events)
		Expect(result).To(BeEmpty())
	})

	It("uses EventTime when LastTimestamp is zero", func() {
		eventTime := time.Now()
		events := []corev1.Event{
			{
				Reason:    "NodeHasNoDiskPressure",
				Message:   "node no longer has disk pressure",
				EventTime: metav1.MicroTime{Time: eventTime},
			},
		}

		result := filterGCEvents(events)
		Expect(result).To(HaveLen(1))
		Expect(result[0].Timestamp).To(BeTemporally("~", eventTime, time.Second))
	})
})
