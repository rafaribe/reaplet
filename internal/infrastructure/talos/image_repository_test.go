package talos

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTalos(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Talos Infrastructure Suite")
}

var _ = Describe("ImageRepository", func() {
	Describe("criInstance", func() {
		It("returns CRI driver and namespace", func() {
			inst := criInstance()
			Expect(inst).NotTo(BeNil())
			Expect(inst.Driver.String()).To(Equal("CRI"))
			Expect(inst.Namespace.String()).To(Equal("NS_CRI"))
		})
	})

	Describe("NewImageRepository", func() {
		It("creates a repository with the given client", func() {
			// We can't easily create a real client without a talosconfig,
			// but we can verify the constructor doesn't panic with nil
			// (it's just a struct assignment)
			repo := NewImageRepository(nil)
			Expect(repo).NotTo(BeNil())
			Expect(repo.client).To(BeNil())
		})
	})
})
