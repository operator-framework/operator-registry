package composite

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

const (
	BasicVeneerBuilderSchema  = "olm.veneer.basic"
	SemverVeneerBuilderSchema = "olm.veneer.semver"
	RawVeneerBuilderSchema    = "olm.veneer.raw"
	CustomVeneerBuilderSchema = "olm.veneer.custom"
)

type ContainerConfig struct {
	ContainerTool string
	BaseImage     string
	WorkingDir    string
}

type BuilderConfig struct {
	ContainerCfg ContainerConfig
	OutputType   string
}

type Builder interface {
	Build(dir string, vd VeneerDefinition) error
	Validate(dir string) error
}

type BasicBuilder struct {
	builderCfg BuilderConfig
}

var _ Builder = &BasicBuilder{}

func NewBasicBuilder(builderCfg BuilderConfig) *BasicBuilder {
	return &BasicBuilder{
		builderCfg: builderCfg,
	}
}

func (bb *BasicBuilder) Build(dir string, vd VeneerDefinition) error {
	if vd.Schema != BasicVeneerBuilderSchema {
		return fmt.Errorf("schema %q does not match the basic veneer builder schema %q", vd.Schema, BasicVeneerBuilderSchema)
	}
	// Parse out the basic veneer configuration
	basicConfig := &BasicVeneerConfig{}
	err := json.Unmarshal(vd.Config, basicConfig)
	if err != nil {
		return fmt.Errorf("unmarshalling basic veneer config: %w", err)
	}

	// get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current working directory: %w", err)
	}

	// build the container command
	containerCmd := exec.Command(bb.builderCfg.ContainerCfg.ContainerTool,
		"run",
		"--rm",
		"-v",
		fmt.Sprintf("%s:%s", wd, bb.builderCfg.ContainerCfg.WorkingDir),
		bb.builderCfg.ContainerCfg.BaseImage,
		"alpha",
		"render-veneer",
		"basic",
		path.Join(bb.builderCfg.ContainerCfg.WorkingDir, basicConfig.Input))

	return build(containerCmd, path.Join(dir, basicConfig.Output), bb.builderCfg.OutputType)
}

func (bb *BasicBuilder) Validate(dir string) error {
	// get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current working directory: %w", err)
	}

	return validate(bb.builderCfg.ContainerCfg, path.Join(wd, dir))
}

type SemverBuilder struct {
	builderCfg BuilderConfig
}

var _ Builder = &SemverBuilder{}

func NewSemverBuilder(builderCfg BuilderConfig) *SemverBuilder {
	return &SemverBuilder{
		builderCfg: builderCfg,
	}
}

func (sb *SemverBuilder) Build(dir string, vd VeneerDefinition) error {
	if vd.Schema != SemverVeneerBuilderSchema {
		return fmt.Errorf("schema %q does not match the semver veneer builder schema %q", vd.Schema, SemverVeneerBuilderSchema)
	}
	// Parse out the semver veneer configuration
	semverConfig := &SemverVeneerConfig{}
	err := json.Unmarshal(vd.Config, semverConfig)
	if err != nil {
		return fmt.Errorf("unmarshalling semver veneer config: %w", err)
	}

	// get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current working directory: %w", err)
	}

	// build the container command
	containerCmd := exec.Command(sb.builderCfg.ContainerCfg.ContainerTool,
		"run",
		"--rm",
		"-v",
		fmt.Sprintf("%s:%s", wd, sb.builderCfg.ContainerCfg.WorkingDir),
		sb.builderCfg.ContainerCfg.BaseImage,
		"alpha",
		"render-veneer",
		"semver",
		path.Join(sb.builderCfg.ContainerCfg.WorkingDir, semverConfig.Input))

	return build(containerCmd, path.Join(dir, semverConfig.Output), sb.builderCfg.OutputType)
}

func (sb *SemverBuilder) Validate(dir string) error {
	// get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current working directory: %w", err)
	}

	return validate(sb.builderCfg.ContainerCfg, path.Join(wd, dir))
}

type RawBuilder struct {
	builderCfg BuilderConfig
}

var _ Builder = &RawBuilder{}

func NewRawBuilder(builderCfg BuilderConfig) *RawBuilder {
	return &RawBuilder{
		builderCfg: builderCfg,
	}
}

func (rb *RawBuilder) Build(dir string, vd VeneerDefinition) error {
	if vd.Schema != RawVeneerBuilderSchema {
		return fmt.Errorf("schema %q does not match the raw veneer builder schema %q", vd.Schema, RawVeneerBuilderSchema)
	}
	// Parse out the raw veneer configuration
	rawConfig := &RawVeneerConfig{}
	err := json.Unmarshal(vd.Config, rawConfig)
	if err != nil {
		return fmt.Errorf("unmarshalling raw veneer config: %w", err)
	}
	// get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current working directory: %w", err)
	}

	// build the container command
	containerCmd := exec.Command(rb.builderCfg.ContainerCfg.ContainerTool,
		"run",
		"--rm",
		"-v",
		fmt.Sprintf("%s:%s", wd, rb.builderCfg.ContainerCfg.WorkingDir),
		"--entrypoint=cat", // This assumes that the `cat` command is available in the container -- Should we also build a `... render-veneer raw` command to ensure consistent operation? Does OPM already have a way to render a raw FBC?
		rb.builderCfg.ContainerCfg.BaseImage,
		path.Join(rb.builderCfg.ContainerCfg.WorkingDir, rawConfig.Input))

	return build(containerCmd, path.Join(dir, rawConfig.Output), rb.builderCfg.OutputType)
}

func (rb *RawBuilder) Validate(dir string) error {
	// get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current working directory: %w", err)
	}

	return validate(rb.builderCfg.ContainerCfg, path.Join(wd, dir))
}

type CustomBuilder struct {
	builderCfg BuilderConfig
}

var _ Builder = &CustomBuilder{}

func NewCustomBuilder(builderCfg BuilderConfig) *CustomBuilder {
	return &CustomBuilder{
		builderCfg: builderCfg,
	}
}

func (cb *CustomBuilder) Build(dir string, vd VeneerDefinition) error {
	if vd.Schema != CustomVeneerBuilderSchema {
		return fmt.Errorf("schema %q does not match the custom veneer builder schema %q", vd.Schema, CustomVeneerBuilderSchema)
	}
	// Parse out the raw veneer configuration
	customConfig := &CustomVeneerConfig{}
	err := json.Unmarshal(vd.Config, customConfig)
	if err != nil {
		return fmt.Errorf("unmarshalling custom veneer config: %w", err)
	}

	// build the command to execute
	// TODO: should the command be run within the container?
	cmd := exec.Command(customConfig.Command, customConfig.Args...)

	// TODO: Should we capture the output here for any reason?
	// Should the custom veneer output an FBC to STDOUT like the other veneer outputs?
	_, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("running command %q: %w", cmd.String(), err)
	}

	return nil
}

func (cb *CustomBuilder) Validate(dir string) error {
	// get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current working directory: %w", err)
	}

	return validate(cb.builderCfg.ContainerCfg, path.Join(wd, dir))
}

func writeDeclCfg(dcfg declcfg.DeclarativeConfig, w io.Writer, output string) error {
	switch output {
	case "yaml":
		return declcfg.WriteYAML(dcfg, w)
	case "json":
		return declcfg.WriteJSON(dcfg, w)
	default:
		return fmt.Errorf("invalid --output value %q, expected (json|yaml)", output)
	}
}

func validate(containerCfg ContainerConfig, dir string) error {
	// build the container command
	containerCmd := exec.Command(containerCfg.ContainerTool,
		"run",
		"--rm",
		"-v",
		fmt.Sprintf("%s:%s", dir, containerCfg.WorkingDir),
		containerCfg.BaseImage,
		"validate",
		containerCfg.WorkingDir)

	_, err := containerCmd.Output()
	if err != nil {
		return fmt.Errorf("running command %q: %w", containerCmd.String(), err)
	}
	return nil
}

func build(cmd *exec.Cmd, outPath string, outType string) error {
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("running command %q: %w", cmd.String(), err)
	}

	// parse out to dcfg
	dcfg, err := declcfg.LoadReader(bytes.NewReader(out))
	if err != nil {
		return fmt.Errorf("parsing builder output: %w", err)
	}

	// write the dcfg
	file, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating output file %q: %w", outPath, err)
	}
	defer file.Close()

	err = writeDeclCfg(*dcfg, file, outType)
	if err != nil {
		return fmt.Errorf("writing to output file %q: %w", outPath, err)
	}

	return nil
}
