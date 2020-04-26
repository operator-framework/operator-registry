package e2e_test

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/otiai10/copy"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/lib/bundle"
	"github.com/operator-framework/operator-registry/pkg/lib/indexer"
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

	bundleImageSuffix = "/olmtest/e2e-bundle"
	indexImage1Suffix = "/olmtest/e2e-index:" + indexTag1
	indexImage2Suffix = "/olmtest/e2e-index:" + indexTag2
	indexImage3Suffix = "/olmtest/e2e-index:" + indexTag3
)

func inTemporaryBuildContext(f func() error) (rerr error) {
	td, err := ioutil.TempDir("", "opm-")
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

func buildBundlesWith(containerTool, bundleImage string, tagPaths map[string]string) error {
	for tag, path := range tagPaths {
		if err := inTemporaryBuildContext(func() error {
			return bundle.BuildFunc(path, "", bundleImage+":"+tag, containerTool, packageName, channels, defaultChannel, false)
		}); err != nil {
			return err
		}
	}
	return nil
}

func buildIndexWith(containerTool, indexImage string, bundleImages ...string) error {
	logger := logrus.WithFields(logrus.Fields{"bundleImages": bundleImages})
	indexAdder := indexer.NewIndexAdder(containertools.NewContainerTool(containerTool, containertools.NoneTool), logger)

	request := indexer.AddToIndexRequest{
		Generate:          false,
		FromIndex:         "",
		BinarySourceImage: "",
		OutDockerfile:     "",
		Tag:               indexImage,
		Bundles:           bundleImages,
		Permissive:        false,
	}

	return indexAdder.AddToIndex(request)
}

func buildFromIndexWith(containerTool, fromIndexImage, toIndexImage string, bundleImages ...string) error {
	logger := logrus.WithFields(logrus.Fields{"bundleImages": bundleImages})
	indexAdder := indexer.NewIndexAdder(containertools.NewContainerTool(containerTool, containertools.NoneTool), logger)

	request := indexer.AddToIndexRequest{
		Generate:          false,
		FromIndex:         fromIndexImage,
		BinarySourceImage: "",
		OutDockerfile:     "",
		Tag:               toIndexImage,
		Bundles:           bundleImages,
		Permissive:        false,
	}

	return indexAdder.AddToIndex(request)
}

// TODO(djzager): make this more complete than what should be a simple no-op
func pruneIndexWith(containerTool, fromIndexImage, toIndexImage string) error {
	logger := logrus.WithFields(logrus.Fields{"packages": packageName})
	indexAdder := indexer.NewIndexPruner(containertools.NewContainerTool(containerTool, containertools.NoneTool), logger)

	request := indexer.PruneFromIndexRequest{
		Generate:          false,
		FromIndex:         fromIndexImage,
		BinarySourceImage: "",
		OutDockerfile:     "",
		Tag:               toIndexImage,
		Packages:          []string{packageName},
		Permissive:        false,
	}

	return indexAdder.PruneFromIndex(request)
}

func pushWith(containerTool, image string) error {
	cmd := exec.Command(containerTool, "push", image)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("output: %s\n err: %v", out, err)
	}
	return err
}

func pushBundles(containerTool string, bundleImages ...string) error {
	for _, image := range bundleImages {
		By(fmt.Sprintf("pushing %s", image))
		err := pushWith(containerTool, image)
		if err != nil {
			return err
		}
	}

	return nil
}

func exportWith(containerTool, indexImage string) error {
	logger := logrus.WithFields(logrus.Fields{"package": packageName})
	indexExporter := indexer.NewIndexExporter(containertools.NewContainerTool(containerTool, containertools.NoneTool), logger)

	request := indexer.ExportFromIndexRequest{
		Index:         indexImage,
		Package:       packageName,
		DownloadPath:  "downloaded",
		ContainerTool: containertools.NewContainerTool(containerTool, containertools.NoneTool),
	}

	return indexExporter.ExportFromIndex(request)
}

func initialize() error {
	tmpDB, err := ioutil.TempFile("./", "index_tmp.db")
	if err != nil {
		return err
	}
	defer os.Remove(tmpDB.Name())

	db, err := sql.Open("sqlite3", tmpDB.Name())
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
	var (
		bundleImage string
		indexImage1 string
		indexImage2 string
		indexImage3 string
	)

	BeforeEach(func() {
		bundleImage = dockerHost + bundleImageSuffix
		indexImage1 = dockerHost + indexImage1Suffix
		indexImage2 = dockerHost + indexImage2Suffix
		indexImage3 = dockerHost + indexImage3Suffix
	})

	IncludeSharedSpecs := func(containerTool string) {
		BeforeEach(func() {
			if dockerUsername == "" || dockerPassword == "" {
				// No creds available, don't login
				return
			}

			By("logging in when registry credentials are available")
			dockerlogin := exec.Command(containerTool, "login", "-u", dockerUsername, "-p", dockerPassword, dockerHost)
			Expect(dockerlogin.Run()).To(Succeed(), "Error logging into %s", dockerHost)
		})

		It("builds and manipulates bundle and index images", func() {
			By("building bundles")
			err := buildBundlesWith(containerTool, bundleImage, map[string]string{
				bundleTag1: bundlePath1,
				bundleTag2: bundlePath2,
				bundleTag3: bundlePath3,
			})
			Expect(err).NotTo(HaveOccurred())

			By("pushing bundles")
			err = pushBundles(containerTool, bundleImage+":"+bundleTag1, bundleImage+":"+bundleTag2, bundleImage+":"+bundleTag3)
			Expect(err).NotTo(HaveOccurred())

			By("building an index")
			err = buildIndexWith(containerTool, indexImage1, bundleImage+":"+bundleTag1, bundleImage+":"+bundleTag2)
			Expect(err).NotTo(HaveOccurred())

			By("pushing an index")
			err = pushWith(containerTool, indexImage1)
			Expect(err).NotTo(HaveOccurred())

			By("building from an index")
			err = buildFromIndexWith(containerTool, indexImage1, indexImage2, bundleImage+":"+bundleTag3)
			Expect(err).NotTo(HaveOccurred())

			By("pushing an index")
			err = pushWith(containerTool, indexImage2)
			Expect(err).NotTo(HaveOccurred())

			By("pruning an index")
			err = pruneIndexWith(containerTool, indexImage1, indexImage3)
			Expect(err).NotTo(HaveOccurred())

			By("pushing an index")
			err = pushWith(containerTool, indexImage3)
			Expect(err).NotTo(HaveOccurred())

			By("exporting an index to disk")
			err = exportWith(containerTool, indexImage2)
			Expect(err).NotTo(HaveOccurred())

			By("loading manifests from a directory")
			err = initialize()
			Expect(err).NotTo(HaveOccurred())

			// clean and try again with containerd
			err = os.RemoveAll("downloaded")
			Expect(err).NotTo(HaveOccurred())

			By("exporting an index to disk with containerd")
			err = exportWith(containertools.NoneTool.String(), indexImage3)
			Expect(err).NotTo(HaveOccurred())

			By("loading manifests from a containerd-extracted directory")
			err = initialize()
			Expect(err).NotTo(HaveOccurred())
		})
	}

	Context("using docker", func() {
		IncludeSharedSpecs("docker")
	})

	Context("using podman", func() {
		IncludeSharedSpecs("podman")
	})
})
