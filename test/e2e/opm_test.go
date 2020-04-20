package e2e_test

import (
	"context"
	"database/sql"
	"github.com/operator-framework/operator-registry/pkg/containertools"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/otiai10/copy"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/rand"

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

	bundleImage = "quay.io/olmtest/e2e-bundle"
	indexImage1 = "quay.io/olmtest/e2e-index:" + indexTag1
	indexImage2 = "quay.io/olmtest/e2e-index:" + indexTag2
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

func buildBundlesWith(containerTool string) error {
	for tag, path := range map[string]string{
		bundleTag1: bundlePath1,
		bundleTag2: bundlePath2,
		bundleTag3: bundlePath3,
	} {
		if err := inTemporaryBuildContext(func() error {
			return bundle.BuildFunc(path, "", bundleImage+":"+tag, containerTool, packageName, channels, defaultChannel, false)
		}); err != nil {
			return err
		}
	}
	return nil
}

func buildIndexWith(containerTool string) error {
	bundles := []string{
		bundleImage + ":" + bundleTag1,
		bundleImage + ":" + bundleTag2,
	}
	logger := logrus.WithFields(logrus.Fields{"bundles": bundles})
	indexAdder := indexer.NewIndexAdder(containertools.NewContainerTool(containerTool, containertools.NoneTool), logger)

	request := indexer.AddToIndexRequest{
		Generate:          false,
		FromIndex:         "",
		BinarySourceImage: "",
		OutDockerfile:     "",
		Tag:               indexImage1,
		Bundles:           bundles,
		Permissive:        false,
	}

	return indexAdder.AddToIndex(request)
}

func buildFromIndexWith(containerTool string) error {
	bundles := []string{
		bundleImage + ":" + bundleTag3,
	}
	logger := logrus.WithFields(logrus.Fields{"bundles": bundles})
	indexAdder := indexer.NewIndexAdder(containertools.NewContainerTool(containerTool, containertools.NoneTool), logger)

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

func pushWith(containerTool, image string) error {
	dockerpush := exec.Command(containerTool, "push", image)
	return dockerpush.Run()
}

func pushBundles(containerTool string) error {
	err := pushWith(containerTool, bundleImage+":"+bundleTag1)
	if err != nil {
		return err
	}
	err = pushWith(containerTool, bundleImage+":"+bundleTag2)
	if err != nil {
		return err
	}
	err = pushWith(containerTool, bundleImage+":"+bundleTag3)
	return err
}

func exportWith(containerTool string) error {
	logger := logrus.WithFields(logrus.Fields{"package": packageName})
	indexExporter := indexer.NewIndexExporter(containertools.NewContainerTool(containerTool, containertools.NoneTool), logger)

	request := indexer.ExportFromIndexRequest{
		Index:         indexImage2,
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
	IncludeSharedSpecs := func(containerTool string) {
		BeforeEach(func() {
			if dockerUsername == "" || dockerPassword == "" {
				Skip("registry credentials are not available")
			}

			dockerlogin := exec.Command(containerTool, "login", "-u", dockerUsername, "-p", dockerPassword, "quay.io")
			err := dockerlogin.Run()
			Expect(err).NotTo(HaveOccurred(), "Error logging into quay.io")
		})

		It("builds and manipulates bundle and index images", func() {
			By("building bundles")
			err := buildBundlesWith(containerTool)
			Expect(err).NotTo(HaveOccurred())

			By("pushing bundles")
			err = pushBundles(containerTool)
			Expect(err).NotTo(HaveOccurred())

			By("building an index")
			err = buildIndexWith(containerTool)
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

			By("exporting an index to disk")
			err = exportWith(containerTool)
			Expect(err).NotTo(HaveOccurred())

			By("loading manifests from a directory")
			err = initialize()
			Expect(err).NotTo(HaveOccurred())

			// clean and try again with containerd
			err = os.RemoveAll("downloaded")
			Expect(err).NotTo(HaveOccurred())

			By("exporting an index to disk with containerd")
			err = exportWith(containertools.NoneTool.String())
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
