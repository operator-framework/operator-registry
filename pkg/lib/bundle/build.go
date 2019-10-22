package bundle

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	log "github.com/sirupsen/logrus"
)

// Create build command to build bundle manifests image
func BuildBundleImage(directory, imageTag, imageBuilder string) (*exec.Cmd, error) {
	var args []string

	dockerfilePath := path.Join(directory, dockerFile)

	switch imageBuilder {
	case "docker", "podman":
		args = append(args, "build", "-f", dockerfilePath, "-t", imageTag, ".")
	case "buildah":
		args = append(args, "bud", "--format=docker", "-f", dockerfilePath, "-t", imageTag, ".")
	default:
		return nil, fmt.Errorf("%s is not supported image builder", imageBuilder)
	}

	return exec.Command(imageBuilder, args...), nil
}

func ExecuteCommand(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Debugf("Running %#v", cmd.Args)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Failed to exec %#v: %v", cmd.Args, err)
	}

	return nil
}

func BuildFunc(directory, imageTag, imageBuilder, packageName, channels, channelDefault string, overwrite bool) error {
	// Generate annotations.yaml and Dockerfile
	err := GenerateFunc(directory, packageName, channels, channelDefault, overwrite)
	if err != nil {
		return err
	}

	// Build bundle image
	log.Info("Building bundle image")
	buildCmd, err := BuildBundleImage(path.Clean(directory), imageBuilder, imageTag)
	if err != nil {
		return err
	}

	if err := ExecuteCommand(buildCmd); err != nil {
		return err
	}

	return nil
}
