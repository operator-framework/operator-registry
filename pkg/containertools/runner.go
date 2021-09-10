//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . CommandRunner
package containertools

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// CommandRunner defines methods to shell out to common container tools
type CommandRunner interface {
	GetToolName() string
	Pull(image string) error
	Build(dockerfile, tag string) error
	Inspect(image string) ([]byte, error)
}

// ContainerCommandRunner is configured to select a container cli tool and
// execute commands with that tooling.
type ContainerCommandRunner struct {
	logger        *logrus.Entry
	containerTool ContainerTool
	config        *RunnerConfig
}

type RunnerConfig struct {
	SkipTLS bool
}

type RunnerOption func(config *RunnerConfig)

func SkipTLS(skip bool) RunnerOption {
	return func(config *RunnerConfig) {
		config.SkipTLS = skip
	}
}

func (r *RunnerConfig) apply(options []RunnerOption) {
	for _, option := range options {
		option(r)
	}
}

func (r *ContainerCommandRunner) argsForCmd(cmd string, args ...string) []string {
	cmdArgs := []string{cmd}
	switch r.containerTool {
	case PodmanTool:
		switch cmd {
		case "pull", "push", "login", "search":
			// --tls-verify is a valid flag for these podman subcommands
			if r.config.SkipTLS {
				cmdArgs = append(cmdArgs, "--tls-verify=false")
			}
		}
	default:
	}
	cmdArgs = append(cmdArgs, args...)
	return cmdArgs
}

// NewCommandRunner takes the containerTool as an input string and returns a
// CommandRunner to run commands with that cli tool
func NewCommandRunner(containerTool ContainerTool, logger *logrus.Entry, opts ...RunnerOption) *ContainerCommandRunner {
	var config RunnerConfig
	config.apply(opts)
	r := &ContainerCommandRunner{
		logger:        logger,
		containerTool: containerTool,
		config:        &config,
	}
	return r
}

// GetToolName returns the container tool this command runner is using
func (r *ContainerCommandRunner) GetToolName() string {
	return r.containerTool.String()
}

// Pull takes a container image path hosted on a container registry and runs the
// pull command to download it onto the local environment
func (r *ContainerCommandRunner) Pull(image string) error {
	args := r.argsForCmd("pull", image)

	command := exec.Command(r.containerTool.String(), args...)

	r.logger.Infof("running %s", command.String())

	out, err := command.CombinedOutput()
	if err != nil {
		r.logger.Errorf(string(out))
		return fmt.Errorf("error pulling image: %s. %v", string(out), err)
	}

	return nil
}

// Build takes a dockerfile and a tag and builds a container image
func (r *ContainerCommandRunner) Build(dockerfile, tag string) error {
	o := DefaultBuildOptions()
	if tag != "" {
		o.AddTag(tag)
	}
	o.SetDockerfile(dockerfile)
	o.SetContext(".")
	command, err := r.containerTool.CommandFactory().BuildCommand(o)
	if err != nil {
		return fmt.Errorf("unable to perform build: %v", err)
	}

	r.logger.Infof("running %s build", r.containerTool)
	r.logger.Infof("%s", command.Args)

	out, err := command.CombinedOutput()
	if err != nil {
		r.logger.Errorf(string(out))
		return fmt.Errorf("error building image: %s. %v", string(out), err)
	}

	return nil
}

// Unpack copies a file or directory from a local container image to a directory on the local filesystem.
func (r *ContainerCommandRunner) Unpack(image, src, dst string) error {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	args := r.argsForCmd("create", image, "")
	command := exec.CommandContext(ctx, r.containerTool.String(), args...)
	r.logger.Infof("running %s create", r.containerTool)
	r.logger.Debugf("%s", command.Args)

	out, err := command.CombinedOutput()
	if err != nil {
		r.logger.Errorf(string(out))
		return fmt.Errorf("error creating container %s: %w", string(out), err)
	}

	// Create a new untarer to copy and read output from stdout to a tar reader.
	// This is done to rewrite file permissions on the fly to avoid permissions-related errors in standard docker cp.
	piper, pipew := io.Pipe()
	defer func() {
		if err := piper.Close(); err != nil {
			r.logger.Warnf("error closing command read pipe: %s", err)
		}
	}()
	defer func() {
		if err := pipew.Close(); err != nil {
			r.logger.Warnf("error closing command write pipe: %s", err)
		}
	}()

	// Run untarer
	var (
		wg       sync.WaitGroup
		expError error
		untarer  = newUntarer(r.logger)
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := untarer.Untar(ctx, tar.NewReader(piper), dst); err != nil {
			expError = err
		}
	}()

	// Execute copy command and pipe output to the untarer
	id := strings.TrimSuffix(string(out), "\n")
	args = r.argsForCmd("cp", id+":"+src, "-")
	command = exec.CommandContext(ctx, r.containerTool.String(), args...)
	command.Stdout = pipew

	r.logger.Infof("running %s cp", r.containerTool)
	r.logger.Debugf("%s", command.Args)
	if err = command.Run(); err != nil {
		return fmt.Errorf("error copying container filesystem %s: %w", string(out), err)
	}

	wg.Wait()

	if expError != nil {
		return fmt.Errorf("error untarring container filesystem: %w", err)
	}

	// Cleanup leftover containers
	args = r.argsForCmd("rm", id)
	command = exec.Command(r.containerTool.String(), args...)

	r.logger.Infof("running %s rm", r.containerTool)
	r.logger.Debugf("%s", command.Args)

	out, err = command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error removing container %s: %w", string(out), err)
	}

	return nil
}

// Inspect runs the 'inspect' command to get image metadata of a local container
// image and returns a byte array of the command's output
func (r *ContainerCommandRunner) Inspect(image string) ([]byte, error) {
	args := r.argsForCmd("inspect", image)

	command := exec.Command(r.containerTool.String(), args...)

	r.logger.Infof("running %s inspect", r.containerTool)
	r.logger.Debugf("%s", command.Args)

	out, err := command.Output()
	if err != nil {
		r.logger.Errorf(string(out))
		return nil, err
	}

	return out, err
}
