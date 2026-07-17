package storage

import (
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Storage Suite")
}

var _ = Describe("SQLite DB", func() {
	var db *DB
	var dbPath string

	BeforeEach(func() {
		f, err := os.CreateTemp("", "reaplet-test-*.db")
		Expect(err).NotTo(HaveOccurred())
		dbPath = f.Name()
		f.Close()

		db, err = New(dbPath)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		db.Close()
		os.Remove(dbPath)
	})

	Describe("History", func() {
		It("records and retrieves history points", func() {
			err := db.RecordHistory("node-1", 100000, 50000, 30000)
			Expect(err).NotTo(HaveOccurred())

			err = db.RecordHistory("node-1", 100000, 55000, 32000)
			Expect(err).NotTo(HaveOccurred())

			points, err := db.GetHistory("node-1", time.Now().Add(-time.Hour))
			Expect(err).NotTo(HaveOccurred())
			Expect(points).To(HaveLen(2))
			Expect(points[0].AllocatedBytes).To(Equal(int64(50000)))
			Expect(points[1].AllocatedBytes).To(Equal(int64(55000)))
		})

		It("returns empty for unknown node", func() {
			points, err := db.GetHistory("unknown", time.Now().Add(-time.Hour))
			Expect(err).NotTo(HaveOccurred())
			Expect(points).To(BeEmpty())
		})

		It("prunes old data", func() {
			err := db.RecordHistory("node-1", 100000, 50000, 30000)
			Expect(err).NotTo(HaveOccurred())

			err = db.PruneHistory(0) // prune everything older than now
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Alert Config", func() {
		It("returns default config", func() {
			cfg, err := db.GetAlertConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.WarningPct).To(Equal(80))
			Expect(cfg.CriticalPct).To(Equal(90))
			Expect(cfg.CooldownMin).To(Equal(15))
		})

		It("saves and retrieves config", func() {
			cfg := &AlertConfig{
				WarningPct:  75,
				CriticalPct: 85,
				CooldownMin: 10,
				Discord:     DiscordConfig{Enabled: true, WebhookURL: "https://discord.com/api/webhooks/test"},
				Pushover:    PushoverConfig{Enabled: true, AppToken: "tok", UserKey: "key"},
			}

			err := db.SaveAlertConfig(cfg)
			Expect(err).NotTo(HaveOccurred())

			loaded, err := db.GetAlertConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded.WarningPct).To(Equal(75))
			Expect(loaded.Discord.Enabled).To(BeTrue())
			Expect(loaded.Discord.WebhookURL).To(Equal("https://discord.com/api/webhooks/test"))
			Expect(loaded.Pushover.AppToken).To(Equal("tok"))
		})
	})

	Describe("Alert Events", func() {
		It("records and retrieves events", func() {
			err := db.RecordAlertEvent("node-1", "warning", 82.5, "Node at 82.5%")
			Expect(err).NotTo(HaveOccurred())

			err = db.RecordAlertEvent("node-2", "critical", 91.0, "Node at 91%")
			Expect(err).NotTo(HaveOccurred())

			events, err := db.GetAlertEvents(10)
			Expect(err).NotTo(HaveOccurred())
			Expect(events).To(HaveLen(2))
			// Most recent first
			Expect(events[0].NodeName).To(Equal("node-2"))
			Expect(events[0].Level).To(Equal("critical"))
			Expect(events[1].NodeName).To(Equal("node-1"))
		})
	})

	Describe("Image Age Tracking", func() {
		It("records first-seen and retrieves age", func() {
			err := db.RecordImageSeen("node-1", "nginx:1.25")
			Expect(err).NotTo(HaveOccurred())

			// Second insert should be ignored (OR IGNORE)
			err = db.RecordImageSeen("node-1", "nginx:1.25")
			Expect(err).NotTo(HaveOccurred())

			ages, err := db.GetAllImageAges("node-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(ages).To(HaveKey("nginx:1.25"))
			Expect(time.Since(ages["nginx:1.25"])).To(BeNumerically("<", 5*time.Second))
		})

		It("returns empty for unknown node", func() {
			ages, err := db.GetAllImageAges("unknown")
			Expect(err).NotTo(HaveOccurred())
			Expect(ages).To(BeEmpty())
		})
	})

	Describe("Cleanup Config", func() {
		It("returns default config", func() {
			cfg, err := db.GetCleanupConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.Enabled).To(BeFalse())
			Expect(cfg.DryRun).To(BeTrue())
			Expect(cfg.MaxPerCycle).To(Equal(5))
		})

		It("saves and retrieves config", func() {
			cfg := &CleanupConfig{
				Enabled:       true,
				IntervalHours: 12,
				MaxAgeDays:    14,
				MaxSizeMB:     1000,
				KeepPatterns:  []string{".*pause.*"},
				MaxPerCycle:   10,
				DryRun:        false,
			}

			err := db.SaveCleanupConfig(cfg)
			Expect(err).NotTo(HaveOccurred())

			loaded, err := db.GetCleanupConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded.Enabled).To(BeTrue())
			Expect(loaded.MaxAgeDays).To(Equal(14))
			Expect(loaded.MaxPerCycle).To(Equal(10))
		})
	})
})
