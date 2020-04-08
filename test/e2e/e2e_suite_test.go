package e2e_test

import (
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	dockerUsername = os.Getenv("DOCKER_USERNAME")
	dockerPassword = os.Getenv("DOCKER_PASSWORD")
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

var _ = BeforeSuite(func() {
	// FIXME: Since podman login doesn't work with daemonless image pulling, we need to login with docker first so podman tests don't fail.
	if dockerUsername == "" || dockerPassword == "" {
		// Test will be skipped anyway
		return
	}

	dockerlogin := exec.Command("docker", "login", "-u", dockerUsername, "-p", dockerPassword, "quay.io")
	err := dockerlogin.Run()
	Expect(err).NotTo(HaveOccurred(), "Error logging into quay.io")
})
