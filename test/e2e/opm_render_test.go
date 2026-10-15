package e2e_test

import (
	"bytes"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

var _ = Describe("opm render", func() {
	// opmBin is the opm built by `make build`, exercised here as a subprocess so
	// that the test reads the same stdout an operator author would. The render
	// subcommand writes to os.Stdout directly, so opm.SetOut() would not capture it.
	var opmBin string

	BeforeEach(func() {
		var err error
		opmBin, err = filepath.Abs(filepath.Join("..", "..", "bin", "opm"))
		Expect(err).NotTo(HaveOccurred())
		Expect(opmBin).To(BeAnExistingFile(), "opm binary not found; run `make build` before `make e2e`")
	})

	Context("for a bundle directory whose CSV labels its relatedImages", func() {
		It("carries the labels into the olm.bundle blob", func() {
			By("rendering the bundle directory")
			var stdout, stderr bytes.Buffer
			cmd := exec.Command(opmBin, "render", "testdata/bundles/related-image-labels.0.1.0")
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			Expect(cmd.Run()).To(Succeed(), "opm render failed: %s", stderr.String())

			By("loading the rendered file-based catalog")
			cfg, err := declcfg.LoadReader(bytes.NewReader(stdout.Bytes()))
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.Bundles).To(HaveLen(1))

			By("checking the relatedImages labels round-tripped")
			// opm render sorts relatedImages by image reference, and appends the
			// CSV's containerImage as an unnamed entry.
			Expect(cfg.Bundles[0].RelatedImages).To(Equal([]declcfg.RelatedImage{
				{
					Name:   "labeled",
					Image:  "quay.io/olmtest/labeled@sha256:c68135620167c41e3d9f6c1d2ca1eb8fa24312b86186d09b8010656b9d25fb47",
					Labels: map[string]string{"feature": "example", "tier": "1"},
				},
				{
					Name:  "",
					Image: "quay.io/olmtest/related-image-labels:v0.1.0",
				},
				{
					Name:  "unlabeled",
					Image: "quay.io/olmtest/unlabeled@sha256:49ed7d6155342adaa2b12fd80c6761c3081d8e6149d187cb7ff91a247cdf2e7a",
				},
			}))
		})
	})
})
