package http

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/rafaribe/reaplet/internal/usecase"
)

// MetricsHandler serves Prometheus-compatible metrics at /metrics.
func MetricsHandler(nodeUC *usecase.NodeUseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodes, err := nodeUC.GetNodes(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var b strings.Builder

		b.WriteString("# HELP reaplet_node_storage_capacity_bytes Total ephemeral storage capacity in bytes\n")
		b.WriteString("# TYPE reaplet_node_storage_capacity_bytes gauge\n")
		for _, n := range nodes {
			fmt.Fprintf(&b, "reaplet_node_storage_capacity_bytes{node=%q} %d\n", n.Name, n.EphemeralStorage.CapacityBytes)
		}

		b.WriteString("# HELP reaplet_node_storage_allocated_bytes Allocated ephemeral storage in bytes\n")
		b.WriteString("# TYPE reaplet_node_storage_allocated_bytes gauge\n")
		for _, n := range nodes {
			fmt.Fprintf(&b, "reaplet_node_storage_allocated_bytes{node=%q} %d\n", n.Name, n.EphemeralStorage.AllocatedBytes)
		}

		b.WriteString("# HELP reaplet_node_storage_available_bytes Available ephemeral storage in bytes\n")
		b.WriteString("# TYPE reaplet_node_storage_available_bytes gauge\n")
		for _, n := range nodes {
			fmt.Fprintf(&b, "reaplet_node_storage_available_bytes{node=%q} %d\n", n.Name, n.EphemeralStorage.AvailableBytes)
		}

		b.WriteString("# HELP reaplet_node_storage_usage_ratio Storage usage as a ratio (0-1)\n")
		b.WriteString("# TYPE reaplet_node_storage_usage_ratio gauge\n")
		for _, n := range nodes {
			ratio := 0.0
			if n.EphemeralStorage.CapacityBytes > 0 {
				ratio = float64(n.EphemeralStorage.AllocatedBytes) / float64(n.EphemeralStorage.CapacityBytes)
			}
			fmt.Fprintf(&b, "reaplet_node_storage_usage_ratio{node=%q} %.4f\n", n.Name, ratio)
		}

		b.WriteString("# HELP reaplet_node_images_total Total number of images on the node\n")
		b.WriteString("# TYPE reaplet_node_images_total gauge\n")
		for _, n := range nodes {
			fmt.Fprintf(&b, "reaplet_node_images_total{node=%q} %d\n", n.Name, len(n.Images))
		}

		b.WriteString("# HELP reaplet_node_images_unused Number of unused images on the node\n")
		b.WriteString("# TYPE reaplet_node_images_unused gauge\n")
		for _, n := range nodes {
			unused := 0
			for _, img := range n.Images {
				if !img.InUse {
					unused++
				}
			}
			fmt.Fprintf(&b, "reaplet_node_images_unused{node=%q} %d\n", n.Name, unused)
		}

		b.WriteString("# HELP reaplet_node_image_size_bytes_total Total size of all images in bytes\n")
		b.WriteString("# TYPE reaplet_node_image_size_bytes_total gauge\n")
		for _, n := range nodes {
			fmt.Fprintf(&b, "reaplet_node_image_size_bytes_total{node=%q} %d\n", n.Name, n.TotalImageSize)
		}

		b.WriteString("# HELP reaplet_node_image_unused_size_bytes Total size of unused images in bytes\n")
		b.WriteString("# TYPE reaplet_node_image_unused_size_bytes gauge\n")
		for _, n := range nodes {
			var unusedSize int64
			for _, img := range n.Images {
				if !img.InUse {
					unusedSize += img.SizeBytes
				}
			}
			fmt.Fprintf(&b, "reaplet_node_image_unused_size_bytes{node=%q} %d\n", n.Name, unusedSize)
		}

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.Write([]byte(b.String()))
	}
}
