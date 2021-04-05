package e2e_test

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/otiai10/copy"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry"
	"github.com/operator-framework/operator-registry/pkg/image/execregistry"
	"github.com/operator-framework/operator-registry/pkg/lib/bundle"
	"github.com/operator-framework/operator-registry/pkg/lib/indexer"
	lregistry "github.com/operator-framework/operator-registry/pkg/lib/registry"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

var (
	packageName    = "prometheus"
	channels       = "preview"
	defaultChannel = "preview"

	bundlePath1 = "manifests/prometheus/0.14.0"
	bundlePath2 = "manifests/prometheus/0.15.0"
	bundlePath3 = "manifests/prometheus/0.22.2"

	bundleTag1 = rand.String(6)
	bundleTag2 = rand.String(6)
	bundleTag3 = rand.String(6)
	indexTag1  = rand.String(6)
	indexTag2  = rand.String(6)
	indexTag3  = rand.String(6)

	bundleImage = dockerHost + "/olmtest/e2e-bundle"
	indexImage  = dockerHost + "/olmtest/e2e-index"
	indexImage1 = dockerHost + "/olmtest/e2e-index:" + indexTag1
	indexImage2 = dockerHost + "/olmtest/e2e-index:" + indexTag2
	indexImage3 = dockerHost + "/olmtest/e2e-index:" + indexTag3

	// publishedIndex is an index used to check for regressions in opm's behavior.
	publishedIndex = os.Getenv("PUBLISHED_INDEX")
)

type bundleLocation struct {
	image, path string
}

type bundleLocations []bundleLocation

func (bl bundleLocations) images() []string {
	images := make([]string, len(bl))
	for i, b := range bl {
		images[i] = b.image
	}

	return images
}

func inTemporaryBuildContext(f func() error) (rerr error) {
	td, err := ioutil.TempDir(".", "opm-")
	if err != nil {
		return err
	}
	err = copy.Copy("../../manifests", filepath.Join(td, "manifests"))
	if err != nil {
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	err = os.Chdir(td)
	if err != nil {
		return err
	}
	defer func() {
		err := os.Chdir(wd)
		if rerr == nil {
			rerr = err
		}
	}()
	return f()
}

func buildIndexWith(containerTool, fromIndexImage, toIndexImage string, bundleImages []string, mode registry.Mode, overwriteLatest bool) error {
	logger := logrus.WithFields(logrus.Fields{"bundles": bundleImages})
	indexAdder := indexer.NewIndexAdder(containertools.NewContainerTool(containerTool, containertools.NoneTool), containertools.NewContainerTool(containerTool, containertools.NoneTool), logger)

	request := indexer.AddToIndexRequest{
		Generate:          false,
		FromIndex:         fromIndexImage,
		BinarySourceImage: "",
		OutDockerfile:     "",
		Tag:               toIndexImage,
		Mode:              mode,
		Bundles:           bundleImages,
		Permissive:        false,
		Overwrite:         overwriteLatest,
		SkipTLS:           *skipTLSForRegistry,
	}

	return indexAdder.AddToIndex(request)
}

func buildFromIndexWith(containerTool string) error {
	bundles := []string{
		bundleImage + ":" + bundleTag3,
	}
	logger := logrus.WithFields(logrus.Fields{"bundles": bundles})
	indexAdder := indexer.NewIndexAdder(containertools.NewContainerTool(containerTool, containertools.NoneTool), containertools.NewContainerTool(containerTool, containertools.NoneTool), logger)

	request := indexer.AddToIndexRequest{
		Generate:          false,
		FromIndex:         indexImage1,
		BinarySourceImage: "",
		OutDockerfile:     "",
		Tag:               indexImage2,
		Bundles:           bundles,
		Permissive:        false,
	}

	return indexAdder.AddToIndex(request)
}

// TODO(djzager): make this more complete than what should be a simple no-op
func pruneIndexWith(containerTool string) error {
	logger := logrus.WithFields(logrus.Fields{"packages": packageName})
	indexAdder := indexer.NewIndexPruner(containertools.NewContainerTool(containerTool, containertools.NoneTool), logger)

	request := indexer.PruneFromIndexRequest{
		Generate:          false,
		FromIndex:         indexImage2,
		BinarySourceImage: "",
		OutDockerfile:     "",
		Tag:               indexImage3,
		Packages:          []string{packageName},
		Permissive:        false,
	}

	return indexAdder.PruneFromIndex(request)
}

func pushWith(containerTool, image string) error {
	dockerpush := exec.Command(containerTool, "push", image)
	dockerpush.Stderr = GinkgoWriter
	dockerpush.Stdout = GinkgoWriter
	return dockerpush.Run()
}

func exportPackageWith(containerTool string) error {
	packages := []string{packageName}
	logger := logrus.WithFields(logrus.Fields{"package": packages})
	indexExporter := indexer.NewIndexExporter(containertools.NewContainerTool(containerTool, containertools.NoneTool), logger)

	request := indexer.ExportFromIndexRequest{
		Index:         indexImage2,
		Packages:      packages,
		DownloadPath:  "downloaded",
		ContainerTool: containertools.NewContainerTool(containerTool, containertools.NoneTool),
		SkipTLS:       *skipTLSForRegistry,
	}

	return indexExporter.ExportFromIndex(request)
}

func exportIndexImageWith(containerTool string) error {

	logger := logrus.NewEntry(logrus.New())
	indexExporter := indexer.NewIndexExporter(containertools.NewContainerTool(containerTool, containertools.NoneTool), logger)

	request := indexer.ExportFromIndexRequest{
		Index:         indexImage2,
		Packages:      []string{},
		DownloadPath:  "downloaded",
		ContainerTool: containertools.NewContainerTool(containerTool, containertools.NoneTool),
		SkipTLS:       *skipTLSForRegistry,
	}

	return indexExporter.ExportFromIndex(request)
}

func initialize() error {
	tmpDB, err := ioutil.TempFile("./", "index_tmp.db")
	if err != nil {
		return err
	}
	defer os.Remove(tmpDB.Name())

	db, err := sqlite.Open(tmpDB.Name())
	if err != nil {
		return err
	}
	defer db.Close()

	dbLoader, err := sqlite.NewSQLLiteLoader(db)
	if err != nil {
		return err
	}
	if err := dbLoader.Migrate(context.TODO()); err != nil {
		return err
	}

	loader := sqlite.NewSQLLoaderForDirectory(dbLoader, "downloaded")
	return loader.Populate()
}

var _ = Describe("opm", func() {
	IncludeSharedSpecs := func(containerTool string) {
		It("builds and validates a bundle image", func() {
			By("building bundle")
			img := bundleImage + ":" + bundleTag3
			err := inTemporaryBuildContext(func() error {
				return bundle.BuildFunc(bundlePath3, "", img, containerTool, packageName, channels, defaultChannel, false)
			})
			Expect(err).NotTo(HaveOccurred())

			By("pushing bundle")
			Expect(pushWith(containerTool, img)).To(Succeed())

			By("pulling bundle")
			logger := logrus.WithFields(logrus.Fields{"image": img})
			tool := containertools.NewContainerTool(containerTool, containertools.NoneTool)
			var registry image.Registry
			switch tool {
			case containertools.PodmanTool, containertools.DockerTool:
				registry, err = execregistry.NewRegistry(tool, logger)
			case containertools.NoneTool:
				registry, err = containerdregistry.NewRegistry(containerdregistry.WithLog(logger))
			default:
				err = fmt.Errorf("unrecognized container-tool option: %s", containerTool)
			}
			Expect(err).NotTo(HaveOccurred())

			unpackDir, err := ioutil.TempDir(".", bundleTag3)
			Expect(err).NotTo(HaveOccurred())
			validator := bundle.NewImageValidator(registry, logger)
			Expect(validator.PullBundleImage(img, unpackDir)).To(Succeed())

			By("validating bundle format")
			Expect(validator.ValidateBundleFormat(unpackDir)).To(Succeed())

			By("validating bundle content")
			manifestsDir := filepath.Join(unpackDir, bundle.ManifestsDir)
			Expect(validator.ValidateBundleContent(manifestsDir)).To(Succeed())
			Expect(os.RemoveAll(unpackDir)).To(Succeed())
		})

		It("builds and manipulates bundle and index images", func() {
			By("building bundles")
			bundles := bundleLocations{
				{bundleImage + ":" + bundleTag1, bundlePath1},
				{bundleImage + ":" + bundleTag2, bundlePath2},
				{bundleImage + ":" + bundleTag3, bundlePath3},
			}
			var err error
			for _, b := range bundles {
				err = inTemporaryBuildContext(func() error {
					return bundle.BuildFunc(b.path, "", b.image, containerTool, packageName, channels, defaultChannel, false)
				})
				Expect(err).NotTo(HaveOccurred())
			}

			By("pushing bundles")
			for _, b := range bundles {
				Expect(pushWith(containerTool, b.image)).NotTo(HaveOccurred())
			}

			By("building an index")
			err = buildIndexWith(containerTool, "", indexImage1, bundles[:2].images(), registry.ReplacesMode, false)
			Expect(err).NotTo(HaveOccurred())

			By("pushing an index")
			err = pushWith(containerTool, indexImage1)
			Expect(err).NotTo(HaveOccurred())

			By("building from an index")
			err = buildFromIndexWith(containerTool)
			Expect(err).NotTo(HaveOccurred())

			By("pushing an index")
			err = pushWith(containerTool, indexImage2)
			Expect(err).NotTo(HaveOccurred())

			By("pruning an index")
			err = pruneIndexWith(containerTool)
			Expect(err).NotTo(HaveOccurred())

			By("pushing an index")
			err = pushWith(containerTool, indexImage3)
			Expect(err).NotTo(HaveOccurred())

			By("exporting a package from an index to disk")
			err = exportPackageWith(containerTool)
			Expect(err).NotTo(HaveOccurred())

			By("loading manifests from a directory")
			err = initialize()
			Expect(err).NotTo(HaveOccurred())

			// clean and try again with containerd
			err = os.RemoveAll("downloaded")
			Expect(err).NotTo(HaveOccurred())

			By("exporting a package from an index to disk with containerd")
			err = exportPackageWith(containertools.NoneTool.String())
			Expect(err).NotTo(HaveOccurred())

			By("loading manifests from a containerd-extracted directory")
			err = initialize()
			Expect(err).NotTo(HaveOccurred())

			// clean containerd-extracted directory
			err = os.RemoveAll("downloaded")
			Expect(err).NotTo(HaveOccurred())

			By("exporting an entire index to disk")
			err = exportIndexImageWith(containerTool)
			Expect(err).NotTo(HaveOccurred())

			By("loading manifests from a directory")
			err = initialize()
			Expect(err).NotTo(HaveOccurred())

		})

		It("build bundles and index via inference", func() {

			bundles := bundleLocations{
				{bundleImage + ":" + rand.String(6), "./testdata/aqua/0.0.1"},
				{bundleImage + ":" + rand.String(6), "./testdata/aqua/0.0.2"},
				{bundleImage + ":" + rand.String(6), "./testdata/aqua/1.0.0"},
				{bundleImage + ":" + rand.String(6), "./testdata/aqua/1.0.1"},
			}

			By("building bundles")
			for _, b := range bundles {
				td, err := ioutil.TempDir(".", "opm-")
				Expect(err).NotTo(HaveOccurred())
				defer os.RemoveAll(td)

				err = bundle.BuildFunc(b.path, td, b.image, containerTool, "", "", "", true)
				Expect(err).NotTo(HaveOccurred())
			}

			By("pushing bundles")
			for _, b := range bundles {
				Expect(pushWith(containerTool, b.image)).NotTo(HaveOccurred())
			}

			By("building an index")
			indexImage := indexImage + ":" + rand.String(6)
			err := buildIndexWith(containerTool, "", indexImage, bundles.images(), registry.ReplacesMode, false)
			Expect(err).NotTo(HaveOccurred())
		})
		It("build index without bundles", func() {
			indexImage := indexImage + ":" + rand.String(6)
			By("building an index")
			err := buildIndexWith(containerTool, "", indexImage, []string{}, registry.ReplacesMode, true)
			Expect(err).NotTo(HaveOccurred())
		})

		PIt("can overwrite existing bundles in an index", func() {
			// TODO fix regression overwriting existing bundles in an index
			bundles := bundleLocations{
				{bundleImage + ":" + rand.String(6), "./testdata/aqua/0.0.1"},
				{bundleImage + ":" + rand.String(6), "./testdata/aqua/0.0.2"},
				{bundleImage + ":" + rand.String(6), "./testdata/aqua/1.0.0"},
				{bundleImage + ":" + rand.String(6), "./testdata/aqua/1.0.1"},
				{bundleImage + ":" + rand.String(6), "./testdata/aqua/1.0.1-overwrite"},
			}

			for _, b := range bundles {
				td, err := ioutil.TempDir(".", "opm-")
				Expect(err).NotTo(HaveOccurred())
				defer os.RemoveAll(td)

				err = bundle.BuildFunc(b.path, td, b.image, containerTool, "", "", "", true)
				Expect(err).NotTo(HaveOccurred())
			}

			By("pushing bundles")
			for _, b := range bundles {
				Expect(pushWith(containerTool, b.image)).NotTo(HaveOccurred())
			}

			indexImage := indexImage + ":" + rand.String(6)
			By("adding net-new bundles to an index")
			err := buildIndexWith(containerTool, "", indexImage, bundles[:4].images(), registry.ReplacesMode, true) // 0.0.1, 0.0.2, 1.0.0, 1.0.1
			Expect(err).NotTo(HaveOccurred())
			Expect(pushWith(containerTool, indexImage)).NotTo(HaveOccurred())

			By("failing to overwrite a non-latest bundle")
			nextIndex := indexImage + "-next"
			err = buildIndexWith(containerTool, indexImage, nextIndex, bundles[1:2].images(), registry.ReplacesMode, true) // 0.0.2
			Expect(err).To(HaveOccurred())

			By("failing to overwrite in a non-replace mode")
			err = buildIndexWith(containerTool, indexImage, nextIndex, bundles[4:].images(), registry.SemVerMode, true) // 1.0.1-overwrite
			Expect(err).To(HaveOccurred())

			By("overwriting the latest bundle in an index")
			err = buildIndexWith(containerTool, indexImage, nextIndex, bundles[4:].images(), registry.ReplacesMode, true) // 1.0.1-overwrite
			Expect(err).NotTo(HaveOccurred())
		})

		It("doesn't change published content on overwrite", func() {
			if publishedIndex == "" {
				Skip("Set the PUBLISHED_INDEX environment variable to enable this test")
			}

			logger := logrus.NewEntry(logrus.StandardLogger())
			logger.Logger.SetLevel(logrus.WarnLevel)
			tool := containertools.NewContainerTool(containerTool, containertools.NoneTool)
			imageIndexer := indexer.ImageIndexer{
				PullTool: tool,
				Logger:   logger,
			}
			dbFile, err := imageIndexer.ExtractDatabase(".", publishedIndex, "", true)
			Expect(err).NotTo(HaveOccurred(), "error extracting registry db")

			db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s", dbFile))
			Expect(err).NotTo(HaveOccurred(), "Error reading db file")

			querier := sqlite.NewSQLLiteQuerierFromDb(db)

			graphLoader, err := sqlite.NewSQLGraphLoaderFromDB(db)
			Expect(err).NotTo(HaveOccurred(), "Error reading db file")

			packages, err := querier.ListPackages(context.TODO())
			Expect(err).NotTo(HaveOccurred(), "Error listing packages")

			var errs []error

			adder := lregistry.NewRegistryAdder(logger)
			for _, pkg := range packages {
				existing, err := graphLoader.Generate(pkg)
				Expect(err).NotTo(HaveOccurred(), "Error generating graph for package %s", pkg, existing)

				for name, ch := range existing.Channels {
					replacement, err := querier.GetBundleThatReplaces(context.TODO(), ch.Head.CsvName, pkg, name)
					if err != nil && err.Error() != fmt.Errorf("no entry found for %s %s", pkg, name).Error() {
						errs = append(errs, err)
						continue
					}

					if replacement != nil {
						continue
					}

					request := lregistry.AddToRegistryRequest{
						Permissive:    false,
						SkipTLS:       *skipTLSForRegistry,
						InputDatabase: dbFile,
						Bundles:       []string{ch.Head.BundlePath},
						Mode:          registry.ReplacesMode,
						ContainerTool: tool,
						Overwrite:     true,
					}

					err = adder.AddToRegistry(request)
					if err != nil {
						errs = append(errs, fmt.Errorf("Error overwriting bundles for package %s: %s", pkg, err))
					}
				}

				overwritten, err := graphLoader.Generate(pkg)
				Expect(err).NotTo(HaveOccurred(), "Error generating graph for package %s", pkg)

				if !reflect.DeepEqual(existing, overwritten) {
					errs = append(errs, fmt.Errorf("the update graph has changed during overwrite-latest for package: %s\nWas\n%s\nIs now\n%s", pkg, existing, overwritten))
				}
			}

			Expect(errs).To(BeEmpty(), fmt.Sprintf("%s", errs))
		})
	}

	Context("using docker", func() {
		cmd := exec.Command("docker")
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			GinkgoT().Logf("container tool docker not found - skipping docker-based opm e2e tests: %v", err)
			return
		}
		IncludeSharedSpecs("docker")
	})

	Context("using podman", func() {
		cmd := exec.Command("podman", "info")
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			GinkgoT().Logf("container tool podman not found - skipping podman-based opm e2e tests: %v", err)
			return
		}
		IncludeSharedSpecs("podman")
	})
})
