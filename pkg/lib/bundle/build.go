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

	dockerfilePath := path.Join(directory, DockerFile)

	switch imageBuilder {
	case "docker", "podman":
		args = append(args, "build", "-f", dockerfilePath, "-t", imageTag, directory)
	case "buildah":
		args = append(args, "bud", "--format=docker", "-f", dockerfilePath, "-t", imageTag, directory)
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

// BuildFunc is used to build bundle container image from a list of manifests
// that exist in local directory and it also generates Dockerfile annotations.yaml
// which contains media type, package name and channels information if the file
// doesn't exist locally.
// Inputs:
// @directory: The local directory where bundle manifests and metadata are located
// @imageTag: The image tag that is applied to the bundle image
// @imageBuilder: The image builder tool that is used to build container image
// (docker, buildah or podman)
// @packageName: The name of the package that bundle image belongs to
// @channels: The list of channels that bundle image belongs to
// @channelDefault: The default channel for the bundle image
// @overwrite: Boolean flag to enable overwriting annotations.yaml locally if existed
func BuildFunc(directory, imageTag, imageBuilder, packageName, channels, channelDefault string, overwrite bool, labels []string) error {
	_, err := os.Stat(directory)
	if os.IsNotExist(err) {
		return err
	}

	// Generate annotations.yaml and Dockerfile
	err = GenerateFunc(directory, packageName, channels, channelDefault, overwrite, labels)
	if err != nil {
		return err
	}

	// Build bundle image
	log.Info("Building bundle image")
	buildCmd, err := BuildBundleImage(path.Clean(directory), imageTag, imageBuilder)
	if err != nil {
		return err
	}

	if err := ExecuteCommand(buildCmd); err != nil {
		return err
	}

	return nil
}
