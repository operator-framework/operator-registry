package composite

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

const (
	BasicBuilderSchema  = "olm.builder.basic"
	SemverBuilderSchema = "olm.builder.semver"
	RawBuilderSchema    = "olm.builder.raw"
	CustomBuilderSchema = "olm.builder.custom"
)

type ContainerConfig struct {
	ContainerTool string
	BaseImage     string
	WorkingDir    string
}

type BuilderConfig struct {
	ContainerCfg     ContainerConfig
	OutputType       string
	CurrentDirectory string
}

type Builder interface {
	Build(dir string, td TemplateDefinition) error
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

func (bb *BasicBuilder) Build(dir string, td TemplateDefinition) error {
	if td.Schema != BasicBuilderSchema {
		return fmt.Errorf("schema %q does not match the basic template builder schema %q", td.Schema, BasicBuilderSchema)
	}
	// Parse out the basic template configuration
	basicConfig := &BasicConfig{}
	err := yaml.UnmarshalStrict(td.Config, basicConfig)
	if err != nil {
		return fmt.Errorf("unmarshalling basic template config: %w", err)
	}

	// validate the basic config fields
	valid := true
	validationErrs := []string{}
	if basicConfig.Input == "" {
		valid = false
		validationErrs = append(validationErrs, "basic template config must have a non-empty input (templateDefinition.config.input)")
	}

	if basicConfig.Output == "" {
		valid = false
		validationErrs = append(validationErrs, "basic template config must have a non-empty output (templateDefinition.config.output)")
	}

	if !valid {
		return fmt.Errorf("basic template configuration is invalid: %s", strings.Join(validationErrs, ","))
	}

	// build the container command
	containerCmd := exec.Command(bb.builderCfg.ContainerCfg.ContainerTool,
		"run",
		"--rm",
		"-v",
		fmt.Sprintf("%s:%s:Z", bb.builderCfg.CurrentDirectory, bb.builderCfg.ContainerCfg.WorkingDir),
		bb.builderCfg.ContainerCfg.BaseImage,
		"alpha",
		"render-template",
		"basic",
		path.Join(bb.builderCfg.ContainerCfg.WorkingDir, basicConfig.Input))

	return build(containerCmd, path.Join(dir, basicConfig.Output), bb.builderCfg.OutputType)
}

func (bb *BasicBuilder) Validate(dir string) error {
	return validate(bb.builderCfg.ContainerCfg, path.Join(bb.builderCfg.CurrentDirectory, dir))
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

func (sb *SemverBuilder) Build(dir string, td TemplateDefinition) error {
	if td.Schema != SemverBuilderSchema {
		return fmt.Errorf("schema %q does not match the semver template builder schema %q", td.Schema, SemverBuilderSchema)
	}
	// Parse out the semver template configuration
	semverConfig := &SemverConfig{}
	err := yaml.UnmarshalStrict(td.Config, semverConfig)
	if err != nil {
		return fmt.Errorf("unmarshalling semver template config: %w", err)
	}

	// validate the semver config fields
	valid := true
	validationErrs := []string{}
	if semverConfig.Input == "" {
		valid = false
		validationErrs = append(validationErrs, "semver template config must have a non-empty input (templateDefinition.config.input)")
	}

	if semverConfig.Output == "" {
		valid = false
		validationErrs = append(validationErrs, "semver template config must have a non-empty output (templateDefinition.config.output)")
	}

	if !valid {
		return fmt.Errorf("semver template configuration is invalid: %s", strings.Join(validationErrs, ","))
	}

	// build the container command
	containerCmd := exec.Command(sb.builderCfg.ContainerCfg.ContainerTool,
		"run",
		"--rm",
		"-v",
		fmt.Sprintf("%s:%s:Z", sb.builderCfg.CurrentDirectory, sb.builderCfg.ContainerCfg.WorkingDir),
		sb.builderCfg.ContainerCfg.BaseImage,
		"alpha",
		"render-template",
		"semver",
		path.Join(sb.builderCfg.ContainerCfg.WorkingDir, semverConfig.Input))

	return build(containerCmd, path.Join(dir, semverConfig.Output), sb.builderCfg.OutputType)
}

func (sb *SemverBuilder) Validate(dir string) error {
	return validate(sb.builderCfg.ContainerCfg, path.Join(sb.builderCfg.CurrentDirectory, dir))
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

func (rb *RawBuilder) Build(dir string, td TemplateDefinition) error {
	if td.Schema != RawBuilderSchema {
		return fmt.Errorf("schema %q does not match the raw template builder schema %q", td.Schema, RawBuilderSchema)
	}
	// Parse out the raw veneer configuration
	rawConfig := &RawConfig{}
	err := yaml.UnmarshalStrict(td.Config, rawConfig)
	if err != nil {
		return fmt.Errorf("unmarshalling raw template config: %w", err)
	}

	// validate the raw config fields
	valid := true
	validationErrs := []string{}
	if rawConfig.Input == "" {
		valid = false
		validationErrs = append(validationErrs, "raw template config must have a non-empty input (templateDefinition.config.input)")
	}

	if rawConfig.Output == "" {
		valid = false
		validationErrs = append(validationErrs, "raw template config must have a non-empty output (templateDefinition.config.output)")
	}

	if !valid {
		return fmt.Errorf("raw template configuration is invalid: %s", strings.Join(validationErrs, ","))
	}

	// build the container command
	containerCmd := exec.Command(rb.builderCfg.ContainerCfg.ContainerTool,
		"run",
		"--rm",
		"-v",
		fmt.Sprintf("%s:%s:Z", rb.builderCfg.CurrentDirectory, rb.builderCfg.ContainerCfg.WorkingDir),
		"--entrypoint=cat", // This assumes that the `cat` command is available in the container -- Should we also build a `... render-template raw` command to ensure consistent operation? Does OPM already have a way to render a raw FBC?
		rb.builderCfg.ContainerCfg.BaseImage,
		path.Join(rb.builderCfg.ContainerCfg.WorkingDir, rawConfig.Input))

	return build(containerCmd, path.Join(dir, rawConfig.Output), rb.builderCfg.OutputType)
}

func (rb *RawBuilder) Validate(dir string) error {
	return validate(rb.builderCfg.ContainerCfg, path.Join(rb.builderCfg.CurrentDirectory, dir))
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

func (cb *CustomBuilder) Build(dir string, td TemplateDefinition) error {
	if td.Schema != CustomBuilderSchema {
		return fmt.Errorf("schema %q does not match the custom template builder schema %q", td.Schema, CustomBuilderSchema)
	}
	// Parse out the raw template configuration
	customConfig := &CustomConfig{}
	err := yaml.UnmarshalStrict(td.Config, customConfig)
	if err != nil {
		return fmt.Errorf("unmarshalling custom template config: %w", err)
	}

	// validate the custom config fields
	valid := true
	validationErrs := []string{}
	if customConfig.Command == "" {
		valid = false
		validationErrs = append(validationErrs, "custom template config must have a non-empty command (templateDefinition.config.command)")
	}

	if customConfig.Output == "" {
		valid = false
		validationErrs = append(validationErrs, "custom template config must have a non-empty output (templateDefinition.config.output)")
	}

	if !valid {
		return fmt.Errorf("custom template configuration is invalid: %s", strings.Join(validationErrs, ","))
	}
	// build the command to execute
	cmd := exec.Command(customConfig.Command, customConfig.Args...)
	cmd.Dir = cb.builderCfg.CurrentDirectory

	// custom template should output a valid FBC to STDOUT so we can
	// build the FBC just like all the other templates.
	return build(cmd, path.Join(dir, customConfig.Output), cb.builderCfg.OutputType)
}

func (cb *CustomBuilder) Validate(dir string) error {
	return validate(cb.builderCfg.ContainerCfg, path.Join(cb.builderCfg.CurrentDirectory, dir))
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
		fmt.Sprintf("%s:%s:Z", dir, containerCfg.WorkingDir),
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
