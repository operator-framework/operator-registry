package e2e_test

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/mod/sumdb/dirhash"

	libimage "github.com/operator-framework/operator-registry/pkg/lib/image"
)

var _ = Describe("opm alpha bundle", func() {
	// out captures opm command output
	var out bytes.Buffer

	BeforeEach(func() {
		// Reset the command's output buffer
		out = bytes.Buffer{}
		opm.SetOut(&out)
		opm.SetErr(&out)
	})

	Context("for an invalid bundle", func() {
		var (
			bundleRef      string
			bundleChecksum string
			tmpDir         string
			rootCA         string
			stopRegistry   func()
		)

		BeforeEach(func() {
			ctx, cancel := context.WithCancel(context.Background())
			stopRegistry = func() {
				cancel()
				<-ctx.Done()
			}

			// Spin up an in-process docker registry with a set of preconfigured test images
			var (
				// Directory containing the docker registry filesystem
				goldenFiles = "../../pkg/image/testdata/golden"

				host string
				err  error
			)
			host, rootCA, err = libimage.RunDockerRegistry(ctx, goldenFiles)
			Expect(err).ToNot(HaveOccurred())

			// Create a bundle ref using the local registry host name and the namespace/name of a bundle we already know the content of
			bundleRef = host + "/olmtest/kiali@sha256:a1bec450c104ceddbb25b252275eb59f1f1e6ca68e0ced76462042f72f7057d8"

			// Generate a checksum of the expected content for the bundle under test
			bundleChecksum, err = dirhash.HashDir(filepath.Join(goldenFiles, "bundles/kiali"), "", dirhash.DefaultHash)
			Expect(err).ToNot(HaveOccurred())

			// Set up a temporary directory that we can use for testing
			tmpDir, err = ioutil.TempDir("", "opm-alpha-bundle-")
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			stopRegistry()
			if CurrentGinkgoTestDescription().Failed {
				// Skip additional cleanup
				return
			}

			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		It("fails to unpack", func() {
			unpackDir := filepath.Join(tmpDir, "unpacked")
			opm.SetArgs([]string{
				"alpha",
				"bundle",
				"unpack",
				"--root-ca",
				rootCA,
				"--out",
				unpackDir,
				bundleRef,
			})

			Expect(opm.Execute()).ToNot(Succeed())
			result, err := ioutil.ReadAll(&out)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(result)).To(ContainSubstring("bundle content validation failed"))
		})

		It("unpacks successfully", func() {
			By("setting --skip-validation")

			unpackDir := filepath.Join(tmpDir, "unpacked")
			opm.SetArgs([]string{
				"alpha",
				"bundle",
				"unpack",
				"--root-ca",
				rootCA,
				"--out",
				unpackDir,
				bundleRef,
				"--skip-validation",
			})

			Expect(opm.Execute()).To(Succeed())
			result, err := ioutil.ReadAll(&out)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(ContainSubstring("bundle content validation failed"))

			checksum, err := dirhash.HashDir(unpackDir, "", dirhash.DefaultHash)
			Expect(err).ToNot(HaveOccurred())
			Expect(checksum).To(Equal(bundleChecksum))
		})
	})
})
