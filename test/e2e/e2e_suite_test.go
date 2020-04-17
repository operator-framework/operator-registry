package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	dockerUsername = os.Getenv("DOCKER_USERNAME")
	dockerPassword = os.Getenv("DOCKER_PASSWORD")
	dockerHost     = os.Getenv("DOCKER_REGISTRY_HOST") // 'DOCKER_HOST' is reserved for the docker daemon
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

var _ = BeforeSuite(func() {
	// FIXME: Since podman login doesn't work with daemonless image pulling, we need to login with docker first so podman tests don't fail.
	if dockerHost == "" {
		// Default to Quay.io
		dockerHost = "quay.io"
	}
	By(fmt.Sprintf("Using container image registry %s", dockerHost))

	if dockerUsername == "" || dockerPassword == "" {
		// No creds given, don't login
		return
	}

	dockerlogin := exec.Command("docker", "login", "-u", dockerUsername, "-p", dockerPassword, dockerHost)
	Expect(dockerlogin.Run()).To(Succeed(), "Error logging into %s", dockerHost)
})
