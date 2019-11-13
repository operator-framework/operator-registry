package bundle

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildBundleImage(t *testing.T) {
	setup("")
	defer cleanup()

	tests := []struct {
		directory    string
		imageTag     string
		imageBuilder string
		commandStr   string
		errorMsg     string
	}{
		{
			testOperatorDir,
			"test",
			"docker",
			"docker build -f /test-operator/Dockerfile -t test /test-operator",
			"",
		},
		{
			testOperatorDir,
			"test",
			"buildah",
			"buildah bud --format=docker -f /test-operator/Dockerfile -t test /test-operator",
			"",
		},
		{
			testOperatorDir,
			"test",
			"podman",
			"podman build -f /test-operator/Dockerfile -t test /test-operator",
			"",
		},
		{
			testOperatorDir,
			"test",
			"hello",
			"",
			"hello is not supported image builder",
		},
	}

	for _, item := range tests {
		var cmd *exec.Cmd
		cmd, err := BuildBundleImage(item.directory, item.imageTag, item.imageBuilder)
		if item.errorMsg == "" {
			require.Contains(t, cmd.String(), item.commandStr)
		} else {
			require.Equal(t, item.errorMsg, err.Error())
		}
	}
}
