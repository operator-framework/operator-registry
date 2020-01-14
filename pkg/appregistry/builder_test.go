package appregistry

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/operator-framework/operator-registry/pkg/apprclient"
	"github.com/operator-framework/operator-registry/pkg/apprclient/apprclientfakes"
)

var _ = Describe("Image building", func() {
	var (
		options  []AppregistryBuildOption
		builder  *AppregistryImageBuilder
		pkg      *apprclient.RegistryMetadata
		operator *apprclient.OperatorMetadata
	)

	Context("with no manifests returned", func() {
		BeforeEach(func() {
			var noopAppender ImageAppendFunc = func(from, to, layer string) error {
				return nil
			}

			client := &apprclientfakes.FakeClient{}
			client.RetrieveOneReturns(nil, nil)

			options = append(options,
				WithAppender(noopAppender),
				WithAppRegistryOrg("metatheria"),
				WithClient(client),
			)
		})

		It("should show an error", func() {
			var err error
			builder, err = NewAppregistryImageBuilder(options...)
			Expect(err).ToNot(HaveOccurred())
			Expect(builder).ToNot(BeNil())
			err = builder.Build()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("no manifests downloaded from appregistry https://quay.io/cnr/metatheria"))
		})
	})

	Context("with good manifests returned", func() {
		BeforeEach(func() {
			var noopAppender ImageAppendFunc = func(from, to, layer string) error {
				return nil
			}

			client := &apprclientfakes.FakeClient{}
			pkg = &apprclient.RegistryMetadata{
				Namespace: "marsupials",
				Name:      "koala",
			}
			client.ListPackagesReturns([]*apprclient.RegistryMetadata{pkg}, nil)

			pkgBlob, err := ioutil.ReadFile("testdata/golden/marsupials_pkg.tar")
			Expect(err).ToNot(HaveOccurred())

			operator = &apprclient.OperatorMetadata{
				RegistryMetadata: *pkg,
				Blob:             pkgBlob,
			}
			client.RetrieveOneReturns(operator, nil)

			options = append(options,
				WithAppender(noopAppender),
				WithAppRegistryOrg("metatheria"),
				WithClient(client),
			)
		})

		JustBeforeEach(func() {
			var err error
			builder, err = NewAppregistryImageBuilder(options...)
			Expect(err).ToNot(HaveOccurred())
			Expect(builder).ToNot(BeNil())
			Expect(builder.Build()).To(Succeed())
		})

		Context("with a custom cache dir", func() {
			var (
				cacheDir      string
				manifestsGlob string
			)

			BeforeEach(func() {
				cacheDir = filepath.Join("testdata", "custom-cache")
				options = append(options, WithCacheDir(cacheDir))
				manifestsGlob = filepath.Join(cacheDir, "manifests-*/koala/marsupials")
			})

			AfterEach(func() {
				Expect(os.RemoveAll(cacheDir)).To(Succeed())
			})

			It("should retain unpacked operator manifests", func() {
				Expect(cacheDir).To(BeADirectory())
				Expect(filepath.Glob(filepath.Join(manifestsGlob, "package.yaml"))).To(HaveLen(1))
				Expect(filepath.Glob(filepath.Join(manifestsGlob, "v1.0.0", "koala.v1.0.0.clusterserviceversion.yaml"))).To(HaveLen(1))
			})

		})
	})

})
