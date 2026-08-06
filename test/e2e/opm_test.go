package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/otiai10/copy"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry"
	"github.com/operator-framework/operator-registry/pkg/image/execregistry"
	"github.com/operator-framework/operator-registry/pkg/lib/bundle"
)

var (
	packageName    = "prometheus"
	channels       = "preview"
	defaultChannel = "preview"

	bundlePath3 = "manifests/prometheus/0.22.2"

	bundleTag3 = rand.String(6)

	bundleImage string

	fbcIndexImageTag = dockerHost + "/olmtest/e2e-fbc"
)

func inTemporaryBuildContext(f func() error, fromDir, toDir string) (rerr error) {
	td, err := os.MkdirTemp(".", "opm-")
	if err != nil {
		return err
	}
	err = copy.Copy(fromDir, filepath.Join(td, toDir))
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

func executeCommand(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Failed to exec %#v: %v", cmd.Args, err)
	}

	return nil
}

// buildFBCWith builds a file based catalog image in the local registry
func buildFBCWith(containerTool, image string) error {
	err := inTemporaryBuildContext(func() error {
		cmd, err := buildFBCImage(image, containerTool, "file-based-catalog.Dockerfile")
		if err != nil {
			return err
		}

		return executeCommand(cmd)
	}, "../../fbc-dir", "")

	if err != nil {
		return err
	}

	return nil
}

// buildFBCImagce uses docker or podman an executes the build command of a dockerfile
func buildFBCImage(imageTag, imageBuilder string, dockerFile string) (*exec.Cmd, error) {
	var args []string

	switch imageBuilder {
	case "docker", "podman":
		args = append(args, "build", "-f", dockerFile, "-t", imageTag, ".")
	case "buildah":
		args = append(args, "bud", "--format=docker", "-f", dockerFile, "-t", imageTag, ".")
	default:
		return nil, fmt.Errorf("%s is not supported image builder", imageBuilder)
	}

	return exec.Command(imageBuilder, args...), nil
}

func pushWith(containerTool, image string) error {
	dockerpush := exec.Command(containerTool, "push", image)
	dockerpush.Stderr = GinkgoWriter
	dockerpush.Stdout = GinkgoWriter
	return dockerpush.Run()
}

var _ = BeforeEach(func() {
	bundleImage = imageRegistry + "/e2e-bundle"
})

var _ = Describe("opm", func() {
	IncludeSharedSpecs := func(containerTool string) {
		It("builds and validates a bundle image", func() {
			By("building bundle")
			img := bundleImage + ":" + bundleTag3
			err := inTemporaryBuildContext(func() error {
				return bundle.BuildFunc(bundlePath3, "", img, containerTool, packageName, channels, defaultChannel, false, "scratch")
			}, "../../manifests", "manifests")
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

			unpackDir, err := os.MkdirTemp(".", bundleTag3)
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

		It("builds and pushes a file-based catalog image", func() {
			By("building a fbc index")
			err := buildFBCWith(containerTool, fbcIndexImageTag)
			Expect(err).NotTo(HaveOccurred())

			By("pushing a fbc index")
			err = pushWith(containerTool, fbcIndexImageTag)
			Expect(err).NotTo(HaveOccurred())
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
