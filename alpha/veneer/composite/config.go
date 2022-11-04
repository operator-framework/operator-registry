package composite

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"
)

func LoadBuildConfigFile(path string) (*BuildConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	bc := &BuildConfig{}
	if err := yaml.Unmarshal(data, bc, func(decoder *json.Decoder) *json.Decoder {
		decoder.DisallowUnknownFields()
		return decoder
	}); err != nil {
		return nil, err
	}
	return bc, nil
}

type BuildConfig struct {
	PackagesBaseDir string               `json:"packagesBaseDir"`
	Catalogs        []CatalogBuildConfig `json:"catalogs"`
}

type CatalogBuildConfig struct {
	Name        string             `json:"name"`
	Destination CatalogDestination `json:"destination"`
}

type CatalogDestination struct {
	BaseImage   string            `json:"baseImage"`
	ExtraLabels map[string]string `json:"extraLabels"`
	OutputImage string            `json:"outputImage"`
}

func LoadPackageConfigs(baseDir string) ([]PackageConfig, error) {
	pkgConfigFiles, err := filepath.Glob(filepath.Join(baseDir, "*", "config.yaml"))
	if err != nil {
		return nil, err
	}

	pcs := make([]PackageConfig, 0, len(pkgConfigFiles))
	for _, pkgConfigFile := range pkgConfigFiles {

		pkgConfigData, err := os.ReadFile(pkgConfigFile)
		if err != nil {
			return nil, err
		}
		pc := &PackageConfig{}
		if err := yaml.Unmarshal(pkgConfigData, pc); err != nil {
			return nil, fmt.Errorf("parse file %q: %v", pkgConfigFile, err)
		}
		pc.directory = filepath.Dir(pkgConfigFile)
		pc.packageName = filepath.Base(pc.directory)
		for _, c := range pc.Catalogs {
			c.packageName = pc.packageName
		}
		pcs = append(pcs, *pc)
	}
	return pcs, nil
}

type PackageConfig struct {
	directory   string
	packageName string

	Catalogs []PackageBuildConfig `json:"catalogs"`
}

type PackageBuildConfig struct {
	packageName string

	BuildConfigs     []string              `json:"buildConfigs"`
	WorkingDirectory string                `json:"workingDirectory"`
	BuildStrategy    OperatorBuildStrategy `json:"buildStrategy"`
}

const (
	BuildStrategyNameOPMBasicVeneer  = "opmBasicVeneer"
	BuildStrategyNameOPMSemverVeneer = "opmSemverVeneer"
	BuildStrategyNameCustom          = "custom"
	BuildStrategyNameRaw             = "raw"
)

type OperatorBuildStrategy struct {
	Name            string                   `json:"name"`
	OPMBasicVeneer  *OPMBasicVeneerStrategy  `json:"opmBasicVeneer"`
	OPMSemverVeneer *OPMSemverVeneerStrategy `json:"opmSemverVeneer"`
	Custom          *CustomStrategy          `json:"custom"`
	Raw             *RawStrategy             `json:"raw"`
}

type OPMBasicVeneerStrategy struct {
	InputFile string `json:"input"`
}

type OPMSemverVeneerStrategy struct {
	InputFile string `json:"input"`
}

type CustomStrategy struct {
	Command []string `json:"command"`
}

type RawStrategy struct {
	Directory string `json:"dir"`
}
