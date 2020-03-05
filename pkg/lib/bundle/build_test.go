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
		imageTag     string
		imageBuilder string
		commandStr   string
		errorMsg     string
	}{
		{
			"test",
			"docker",
			"docker build -f bundle.Dockerfile -t test .",
			"",
		},
		{
			"test",
			"buildah",
			"buildah bud --format=docker -f bundle.Dockerfile -t test .",
			"",
		},
		{
			"test",
			"podman",
			"podman build -f bundle.Dockerfile -t test .",
			"",
		},
		{
			"test",
			"hello",
			"",
			"hello is not supported image builder",
		},
	}

	for _, item := range tests {
		var cmd *exec.Cmd
		cmd, err := BuildBundleImage(item.imageTag, item.imageBuilder)
		if item.errorMsg == "" {
			require.Contains(t, cmd.String(), item.commandStr)
		} else {
			require.Equal(t, item.errorMsg, err.Error())
		}
	}
}
