package composite

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/operator-framework/operator-registry/alpha/action"
	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/veneer/basic"
	"github.com/operator-framework/operator-registry/alpha/veneer/semver"
	"github.com/operator-framework/operator-registry/pkg/image"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
)

func CreateBuilders(bc BuildConfig, pcs []PackageConfig, imageRegistry image.Registry) ([]Builder, error) {
	var errs []error

	builderMap := map[string]*Builder{}
	for _, ctlg := range bc.Catalogs {
		if _, ok := builderMap[ctlg.Name]; ok {
			errs = append(errs, fmt.Errorf("duplicate catalog build config name %q", ctlg.Name))
			continue
		}
		builderMap[ctlg.Name] = &Builder{BuildConfig: ctlg}
	}

	for _, pc := range pcs {
		for _, ctlg := range pc.Catalogs {
			for _, bc := range ctlg.BuildConfigs {
				builder, ok := builderMap[bc]
				if !ok {
					errs = append(errs, fmt.Errorf("unknown catalog build config name %q referenced in package %q config", bc, pc.packageName))
					continue
				}

				var pb packageBuilder
				switch ctlg.BuildStrategy.Name {
				case BuildStrategyNameOPMBasicVeneer:
					if ctlg.BuildStrategy.OPMBasicVeneer == nil {
						errs = append(errs, fmt.Errorf("requested strategy %q not defined for catalog build %q in package %q", ctlg.BuildStrategy.Name, bc, pc.packageName))
						continue
					}
					pb = basicVeneerBuilder{
						OPMBasicVeneerStrategy: *ctlg.BuildStrategy.OPMBasicVeneer,
						Registry:               imageRegistry,
						workDir:                pc.directory,
					}
				case BuildStrategyNameOPMSemverVeneer:
					if ctlg.BuildStrategy.OPMSemverVeneer == nil {
						errs = append(errs, fmt.Errorf("requested strategy %q not defined for catalog build %q in package %q", ctlg.BuildStrategy.Name, bc, pc.packageName))
						continue
					}
					pb = semverVeneerBuilder{
						OPMSemverVeneerStrategy: *ctlg.BuildStrategy.OPMSemverVeneer,
						Registry:                imageRegistry,
						workDir:                 pc.directory,
					}
				case BuildStrategyNameCustom:
					if ctlg.BuildStrategy.Custom == nil {
						errs = append(errs, fmt.Errorf("requested strategy %q not defined for catalog build %q in package %q", ctlg.BuildStrategy.Name, bc, pc.packageName))
						continue
					}
					pb = customBuilder{
						CustomStrategy: *ctlg.BuildStrategy.Custom,
						workDir:        pc.directory,
					}
				case BuildStrategyNameRaw:
					if ctlg.BuildStrategy.Raw == nil {
						errs = append(errs, fmt.Errorf("requested strategy %q not defined for catalog build %q in package %q", ctlg.BuildStrategy.Name, bc, pc.packageName))
						continue
					}
					pb = rawBuilder{
						RawStrategy: *ctlg.BuildStrategy.Raw,
						workDir:     pc.directory,
					}
				default:
					errs = append(errs, fmt.Errorf("unknown strategy %q references in catalog build %q for package %q config", ctlg.BuildStrategy.Name, bc, pc.packageName))
					continue
				}
				builder.packageBuilders = append(builder.packageBuilders, pb)
			}
		}
	}

	if len(errs) > 0 {
		return nil, errors.NewAggregate(errs)
	}

	builders := make([]Builder, 0, len(builderMap))
	for _, ctlg := range bc.Catalogs {
		builder := builderMap[ctlg.Name]
		sort.Slice(builder.packageBuilders, func(i, j int) bool {
			return builder.packageBuilders[i].PackageName() < builder.packageBuilders[j].PackageName()
		})
		builders = append(builders, *builder)
	}
	return builders, nil
}

type Builder struct {
	BuildConfig     CatalogBuildConfig
	packageBuilders []packageBuilder
}

func (b Builder) Packages() []string {
	pkgs := sets.NewString()
	for _, pb := range b.packageBuilders {
		pkgs.Insert(pb.PackageName())
	}
	return pkgs.List()
}

func (b Builder) Build(ctx context.Context) error {
	buildTempDir, err := os.MkdirTemp("", "fbcb-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(buildTempDir)

	var (
		catalogFBC  declcfg.DeclarativeConfig
		packageFBCs []declcfg.DeclarativeConfig
		errs        []error
	)

	for _, pb := range b.packageBuilders {
		packageFBC, err := pb.Build(ctx)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		packageFBCs = append(packageFBCs, *packageFBC)

		catalogFBC.Packages = append(catalogFBC.Packages, packageFBC.Packages...)
		catalogFBC.Channels = append(catalogFBC.Channels, packageFBC.Channels...)
		catalogFBC.Bundles = append(catalogFBC.Bundles, packageFBC.Bundles...)
		catalogFBC.Others = append(catalogFBC.Others, packageFBC.Others...)
	}

	if len(errs) > 0 {
		return errors.NewAggregate(errs)
	}

	if _, err := declcfg.ConvertToModel(catalogFBC); err != nil {
		return err
	}

	for _, pfbc := range packageFBCs {
		packageDir := filepath.Join(buildTempDir, pfbc.Packages[0].Name)
		if err := os.MkdirAll(packageDir, 0777); err != nil {
			return err
		}
		if err := writeFBCFile(pfbc, packageDir); err != nil {
			return err
		}
	}

	dockerfileDir := filepath.Dir(buildTempDir)
	dockerfileName := filepath.Base(buildTempDir) + ".Dockerfile"
	dockerfilePath := filepath.Join(dockerfileDir, dockerfileName)
	dockerfile, err := os.Create(dockerfilePath)
	if err != nil {
		return err
	}
	defer dockerfile.Close()
	defer os.RemoveAll(dockerfilePath)

	gd := action.GenerateDockerfile{
		BaseImage:   b.BuildConfig.Destination.BaseImage,
		IndexDir:    filepath.Base(buildTempDir),
		ExtraLabels: b.BuildConfig.Destination.ExtraLabels,
		Writer:      dockerfile,
	}
	if err := gd.Run(); err != nil {
		return err
	}

	cmdName := "docker"
	args := []string{"build", "-t", b.BuildConfig.Destination.OutputImage, "-f", dockerfilePath, dockerfileDir}
	cmd := exec.CommandContext(context.TODO(), cmdName, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("run command %v: %v\n%s", append([]string{cmdName}, args...), err, string(out))
	}
	return nil
}

func writeFBCFile(fbc declcfg.DeclarativeConfig, packageDir string) error {
	f, err := os.Create(filepath.Join(packageDir, "catalog.yaml"))
	if err != nil {
		return err
	}
	defer f.Close()
	return declcfg.WriteYAML(fbc, f)
}

type packageBuilder interface {
	PackageName() string
	Build(ctx context.Context) (*declcfg.DeclarativeConfig, error)
}

var (
	_ packageBuilder = &basicVeneerBuilder{}
	_ packageBuilder = &semverVeneerBuilder{}
	_ packageBuilder = &customBuilder{}
	_ packageBuilder = &rawBuilder{}
)

type basicVeneerBuilder struct {
	OPMBasicVeneerStrategy
	image.Registry
	workDir string
}

func (b basicVeneerBuilder) PackageName() string {
	return filepath.Base(b.workDir)
}

func (b basicVeneerBuilder) Build(ctx context.Context) (*declcfg.DeclarativeConfig, error) {
	v := basic.Veneer{Registry: b.Registry}
	f, err := os.Open(filepath.Join(b.workDir, b.InputFile))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return v.Render(ctx, f)
}

type semverVeneerBuilder struct {
	OPMSemverVeneerStrategy
	image.Registry
	workDir string
}

func (b semverVeneerBuilder) PackageName() string {
	return filepath.Base(b.workDir)
}

func (b semverVeneerBuilder) Build(ctx context.Context) (*declcfg.DeclarativeConfig, error) {
	data, err := os.ReadFile(filepath.Join(b.workDir, b.InputFile))
	if err != nil {
		return nil, fmt.Errorf("read input file %q: %v", b.InputFile, err)
	}
	v := semver.Veneer{Registry: b.Registry, Data: bytes.NewReader(data)}
	return v.Render(ctx)
}

type customBuilder struct {
	CustomStrategy
	workDir string
}

func (b customBuilder) PackageName() string {
	return filepath.Base(b.workDir)
}

func (b customBuilder) Build(ctx context.Context) (*declcfg.DeclarativeConfig, error) {
	var cmd *exec.Cmd
	switch len(b.Command) {
	case 0:
		return nil, fmt.Errorf("command is not defined")
	case 1:
		cmd = exec.CommandContext(ctx, b.Command[0])
	default:
		cmd = exec.CommandContext(ctx, b.Command[0], b.Command[1:]...)
	}
	cmd.Dir = b.workDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("run command %v: %v\n%s", b.Command, err, string(out))
	}

	return declcfg.LoadReader(bytes.NewReader(out))
}

type rawBuilder struct {
	RawStrategy
	workDir string
}

func (b rawBuilder) PackageName() string {
	return filepath.Base(b.workDir)
}

func (b rawBuilder) Build(ctx context.Context) (*declcfg.DeclarativeConfig, error) {
	outputDir := filepath.Join(b.workDir, b.Directory)

	r := action.Render{
		AllowedRefMask: action.RefDCDir,
		Refs:           []string{outputDir},
	}
	return r.Run(ctx)
}
