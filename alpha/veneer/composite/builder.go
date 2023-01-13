package composite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/veneer/basic"
	"github.com/operator-framework/operator-registry/alpha/veneer/semver"
)

const (
	BasicVeneerBuilderSchema  = "olm.veneer.basic"
	SemverVeneerBuilderSchema = "olm.veneer.semver"
	RawVeneerBuilderSchema    = "olm.veneer.raw"
)

type ContainerConfig struct {
	ContainerTool string
	BaseImage     string
	WorkingDir    string
}

// TODO: update this to use docker ...
type Builder interface {
	Build(ctx context.Context, vd VeneerDef, containerCfg ContainerConfig) (*declcfg.DeclarativeConfig, string, error)
}

type BasicBuilder struct {
	veneer basic.Veneer
}

var _ Builder = &BasicBuilder{}

func NewBasicBuilder() *BasicBuilder {
	return &BasicBuilder{
		veneer: basic.Veneer{},
	}
}

func (bb *BasicBuilder) Build(ctx context.Context, vd VeneerDef, containerCfg ContainerConfig) (*declcfg.DeclarativeConfig, string, error) {
	// Parse out the basic veneer configuration
	basicConfig := &BasicVeneerConfig{}
	err := json.Unmarshal(vd.Config, basicConfig)
	if err != nil {
		return nil, "", fmt.Errorf("unmarshalling basic veneer config: %w", err)
	}

	// get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("getting current working directory: %w", err)
	}

	// build the container command
	containerCmd := exec.Command(containerCfg.ContainerTool,
		"run",
		"--rm",
		"-v",
		fmt.Sprintf("%s:%s", wd, containerCfg.WorkingDir),
		containerCfg.BaseImage,
		"alpha",
		"render-veneer",
		"basic",
		path.Join(containerCfg.WorkingDir, basicConfig.Input))

	out, err := containerCmd.Output()
	if err != nil {
		return nil, "", fmt.Errorf("running command %q | STDERR: %s", containerCmd.String(), err.(*exec.ExitError).Stderr)
	}

	// parse out to dcfg
	dcfg, err := declcfg.LoadReader(bytes.NewReader(out))
	if err != nil {
		return nil, "", fmt.Errorf("parsing basic veneer render output: %w", err)
	}

	return dcfg, basicConfig.Output, nil
}

type SemverBuilder struct {
	veneer semver.Veneer
}

var _ Builder = &SemverBuilder{}

func NewSemverBuilder() *SemverBuilder {
	return &SemverBuilder{
		veneer: semver.Veneer{},
	}
}

func (sb *SemverBuilder) Build(ctx context.Context, vd VeneerDef, containerCfg ContainerConfig) (*declcfg.DeclarativeConfig, string, error) {
	// Parse out the semver veneer configuration
	semverConfig := &SemverVeneerConfig{}
	err := json.Unmarshal(vd.Config, semverConfig)
	if err != nil {
		return nil, "", fmt.Errorf("unmarshalling semver veneer config: %w", err)
	}

	// get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("getting current working directory: %w", err)
	}

	// build the container command
	containerCmd := exec.Command(containerCfg.ContainerTool,
		"run",
		"--rm",
		"-v",
		fmt.Sprintf("%s:%s", wd, containerCfg.WorkingDir),
		containerCfg.BaseImage,
		"alpha",
		"render-veneer",
		"semver",
		path.Join(containerCfg.WorkingDir, semverConfig.Input))

	out, err := containerCmd.Output()
	if err != nil {
		return nil, "", fmt.Errorf("running command %q | STDERR: %s", containerCmd.String(), err.(*exec.ExitError).Stderr)
	}

	// parse out to dcfg
	dcfg, err := declcfg.LoadReader(bytes.NewReader(out))
	if err != nil {
		return nil, "", fmt.Errorf("parsing semver veneer render output: %w", err)
	}

	return dcfg, semverConfig.Output, nil
}

type RawBuilder struct{}

var _ Builder = &RawBuilder{}

func NewRawBuilder() *RawBuilder {
	return &RawBuilder{}
}

func (rb *RawBuilder) Build(ctx context.Context, vd VeneerDef, containerCfg ContainerConfig) (*declcfg.DeclarativeConfig, string, error) {
	// Parse out the basic veneer configuration
	rawConfig := &RawVeneerConfig{}
	err := json.Unmarshal(vd.Config, rawConfig)
	if err != nil {
		return nil, "", fmt.Errorf("unmarshalling raw veneer config: %w", err)
	}
	// get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("getting current working directory: %w", err)
	}

	// build the container command
	containerCmd := exec.Command(containerCfg.ContainerTool,
		"run",
		"--rm",
		"-v",
		fmt.Sprintf("%s:%s", wd, containerCfg.WorkingDir),
		"--entrypoint=cat", // This assumes that the `cat` command is available in the container -- Should we also build a `... render-veneer raw` command to ensure consistent operation? Does OPM already have a way to render a raw FBC?
		containerCfg.BaseImage,
		path.Join(containerCfg.WorkingDir, rawConfig.Input))

	out, err := containerCmd.Output()
	if err != nil {
		return nil, "", fmt.Errorf("running command %q | STDERR: %s", containerCmd.String(), err.(*exec.ExitError).Stderr)
	}

	// parse out to dcfg
	dcfg, err := declcfg.LoadReader(bytes.NewReader(out))
	if err != nil {
		return nil, "", fmt.Errorf("parsing raw veneer render output: %w", err)
	}

	return dcfg, rawConfig.Output, nil
}
