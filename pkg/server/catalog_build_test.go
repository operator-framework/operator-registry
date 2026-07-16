package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing/fstest"

	"sigs.k8s.io/yaml"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
	"github.com/operator-framework/operator-registry/pkg/registry"
)

// buildCatalogFromManifestsDir builds an fs.FS containing FBC JSON for all packages
// in the given manifests directory (old package-manifest format). The result can be
// passed directly to fbcCacheFromFs.
//
// It handles two directory structures found in the manifests directory:
//   - Versioned: package/<version>/<manifests>  (etcd, prometheus)
//   - Flat:      package/<manifests>             (strimzi-kafka-operator)
//
// The etcdoperator.v0.9.2 bundle gets image="fake/etcd-operator:v0.9.2" to match
// the original test setup that applied this via a SQL UPDATE on the sqlite store.
func buildCatalogFromManifestsDir(manifestsDir string) (fs.FS, error) {
	cfg := &declcfg.DeclarativeConfig{}

	pkgDirs, err := os.ReadDir(manifestsDir)
	if err != nil {
		return nil, fmt.Errorf("reading manifests dir: %w", err)
	}

	for _, entry := range pkgDirs {
		if !entry.IsDir() {
			continue
		}
		pkgDir := filepath.Join(manifestsDir, entry.Name())
		pkgCfg, err := buildPackageFBC(pkgDir)
		if err != nil {
			return nil, fmt.Errorf("building package %s: %w", entry.Name(), err)
		}
		cfg.Packages = append(cfg.Packages, pkgCfg.Packages...)
		cfg.Channels = append(cfg.Channels, pkgCfg.Channels...)
		cfg.Bundles = append(cfg.Bundles, pkgCfg.Bundles...)
	}

	var buf bytes.Buffer
	if err := declcfg.WriteJSON(*cfg, &buf); err != nil {
		return nil, fmt.Errorf("writing FBC JSON: %w", err)
	}

	return fstest.MapFS{
		"catalog.json": &fstest.MapFile{Data: buf.Bytes()},
	}, nil
}

// rawCSVFields is a minimal struct for extracting fields from a CSV YAML file.
// sigs.k8s.io/yaml converts YAML to JSON before unmarshaling, so json: tags are used.
type rawCSVFields struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name        string            `json:"name"`
		Annotations map[string]string `json:"annotations"`
	} `json:"metadata"`
	Spec struct {
		Version  string   `json:"version"`
		Replaces string   `json:"replaces"`
		Skips    []string `json:"skips"`
		CRDs     struct {
			Owned    []rawCRDDesc `json:"owned"`
			Required []rawCRDDesc `json:"required"`
		} `json:"customresourcedefinitions"`
	} `json:"spec"`
}

type rawCRDDesc struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
	Name    string `json:"name"` // e.g. "etcdclusters.etcd.database.coreos.com"
}

// manifestInfo holds parsed info about a single YAML file in a bundle directory.
type manifestInfo struct {
	jsonData []byte // YAML-to-JSON converted bytes
	kind     string
	csv      *rawCSVFields // non-nil if kind == ClusterServiceVersion
}

// buildPackageFBC builds the FBC DeclarativeConfig for a single package directory.
func buildPackageFBC(pkgDir string) (*declcfg.DeclarativeConfig, error) {
	pkgManifest, err := readPackageManifest(pkgDir)
	if err != nil {
		return nil, err
	}

	// collectDir gathers all manifests from a single directory.
	// Returns: list of CSVs and list of CRD JSON blobs found in that directory.
	collectDir := func(dir string) ([]*manifestInfo, [][]byte, error) {
		var csvInfos []*manifestInfo
		var crdJSONs [][]byte
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, nil, err
		}
		for _, e := range entries {
			if e.IsDir() || (!isYAML(e.Name())) {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				return nil, nil, err
			}
			jsonData, err := yaml.YAMLToJSON(raw)
			if err != nil {
				continue // skip unparseable files (e.g. package yamls)
			}

			var obj struct {
				Kind string `json:"kind"`
			}
			if err := json.Unmarshal(jsonData, &obj); err != nil || obj.Kind == "" {
				continue
			}

			info := &manifestInfo{jsonData: jsonData, kind: obj.Kind}

			switch obj.Kind {
			case "ClusterServiceVersion":
				var csv rawCSVFields
				if err := json.Unmarshal(jsonData, &csv); err != nil {
					return nil, nil, fmt.Errorf("parsing CSV %s: %w", e.Name(), err)
				}
				info.csv = &csv
				csvInfos = append(csvInfos, info)
			default:
				crdJSONs = append(crdJSONs, jsonData)
			}
		}
		return csvInfos, crdJSONs, nil
	}

	// Collect manifests. Walk subdirectories for versioned packages (etcd, prometheus)
	// and also the package root for flat packages (strimzi).
	type dirResult struct {
		csvs [](*manifestInfo)
		crds [][]byte
	}
	var allDirs []dirResult

	topEntries, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, err
	}

	// Always collect from the package dir root (handles flat structure).
	rootCSVs, rootCRDs, err := collectDir(pkgDir)
	if err != nil {
		return nil, err
	}
	if len(rootCSVs) > 0 || len(rootCRDs) > 0 {
		allDirs = append(allDirs, dirResult{csvs: rootCSVs, crds: rootCRDs})
	}

	// Collect from version subdirectories.
	for _, e := range topEntries {
		if !e.IsDir() {
			continue
		}
		csvs, crds, err := collectDir(filepath.Join(pkgDir, e.Name()))
		if err != nil {
			return nil, err
		}
		if len(csvs) > 0 || len(crds) > 0 {
			allDirs = append(allDirs, dirResult{csvs: csvs, crds: crds})
		}
	}

	// Build a map: csvName -> FBC Bundle, and csvName -> rawCSVFields
	bundleMap := map[string]declcfg.Bundle{}
	csvFieldMap := map[string]*rawCSVFields{}

	for _, dir := range allDirs {
		// All CRDs in a directory are shared among all CSVs in that same directory.
		for _, csvInfo := range dir.csvs {
			csv := csvInfo.csv
			name := csv.Metadata.Name

			var props []property.Property

			// olm.bundle.object: CSV manifest
			props = append(props, property.MustBuildBundleObject(csvInfo.jsonData))

			// olm.bundle.object: CRD manifests
			for _, crdJSON := range dir.crds {
				props = append(props, property.MustBuildBundleObject(crdJSON))
			}

			// olm.package
			props = append(props, property.MustBuild(&property.Package{
				PackageName: pkgManifest.PackageName,
				Version:     csv.Spec.Version,
			}))

			// olm.gvk from owned CRDs
			for _, owned := range csv.Spec.CRDs.Owned {
				if owned.Kind != "" {
					group := crdGroup(owned.Group, owned.Name)
					props = append(props, property.MustBuildGVK(group, owned.Version, owned.Kind))
				}
			}

			// olm.gvk.required from required CRDs
			for _, req := range csv.Spec.CRDs.Required {
				if req.Kind != "" {
					group := crdGroup(req.Group, req.Name)
					props = append(props, property.MustBuildGVKRequired(group, req.Version, req.Kind))
				}
			}

			// Extra properties from olm.properties annotation
			if olmPropsJSON, ok := csv.Metadata.Annotations["olm.properties"]; ok {
				var extraProps []property.Property
				if err := json.Unmarshal([]byte(olmPropsJSON), &extraProps); err == nil {
					props = append(props, extraProps...)
				}
			}

			// Stable sort so the property order is deterministic.
			sort.SliceStable(props, func(i, j int) bool {
				if props[i].Type != props[j].Type {
					return props[i].Type < props[j].Type
				}
				return string(props[i].Value) < string(props[j].Value)
			})

			image := ""
			if name == "etcdoperator.v0.9.2" {
				image = "fake/etcd-operator:v0.9.2"
			}

			if _, dup := bundleMap[name]; dup {
				return nil, fmt.Errorf("duplicate bundle name %q found in multiple directories", name)
			}
			bundleMap[name] = declcfg.Bundle{
				Schema:     "olm.bundle",
				Name:       name,
				Package:    pkgManifest.PackageName,
				Image:      image,
				Properties: props,
			}
			csvFieldMap[name] = csv
		}
	}

	// Build FBC channels from the package manifest.
	channels, err := buildFBCChannels(pkgManifest, csvFieldMap)
	if err != nil {
		return nil, err
	}

	// Collect bundles in deterministic order.
	var bundles []declcfg.Bundle
	for _, b := range bundleMap {
		bundles = append(bundles, b)
	}
	sort.Slice(bundles, func(i, j int) bool {
		return bundles[i].Name < bundles[j].Name
	})

	return &declcfg.DeclarativeConfig{
		Packages: []declcfg.Package{{
			Schema:         "olm.package",
			Name:           pkgManifest.PackageName,
			DefaultChannel: pkgManifest.GetDefaultChannel(),
		}},
		Channels: channels,
		Bundles:  bundles,
	}, nil
}

// buildFBCChannels constructs FBC channel objects from a package manifest plus
// per-CSV replaces/skips info gathered from the bundle YAML files.
func buildFBCChannels(pkgManifest *registry.PackageManifest, csvFields map[string]*rawCSVFields) ([]declcfg.Channel, error) {
	var channels []declcfg.Channel

	for _, ch := range pkgManifest.Channels {
		entries, err := traverseReplaces(ch.CurrentCSVName, csvFields)
		if err != nil {
			return nil, fmt.Errorf("channel %s: %w", ch.Name, err)
		}
		channels = append(channels, declcfg.Channel{
			Schema:  "olm.channel",
			Name:    ch.Name,
			Package: pkgManifest.PackageName,
			Entries: entries,
		})
	}

	// Deterministic order.
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].Name < channels[j].Name
	})
	return channels, nil
}

// traverseReplaces walks the replaces chain from head, building channel entries.
// Skip targets that exist in csvFields are also traversed so they get proper entries.
// Skip targets without manifests are not enqueued (they appear only in Skips fields).
func traverseReplaces(head string, csvFields map[string]*rawCSVFields) ([]declcfg.ChannelEntry, error) { //nolint:unparam
	visited := map[string]bool{}
	var entries []declcfg.ChannelEntry

	queue := []string{head}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if visited[name] {
			continue
		}
		visited[name] = true

		csv, ok := csvFields[name]
		if !ok {
			// Replaces target not present in manifests; include as a stub entry.
			entries = append(entries, declcfg.ChannelEntry{Name: name})
			continue
		}

		entry := declcfg.ChannelEntry{Name: name}
		if csv.Spec.Replaces != "" {
			entry.Replaces = csv.Spec.Replaces
			if !visited[csv.Spec.Replaces] {
				queue = append(queue, csv.Spec.Replaces)
			}
		}
		if len(csv.Spec.Skips) > 0 {
			entry.Skips = csv.Spec.Skips
			for _, s := range csv.Spec.Skips {
				if !visited[s] && csvFields[s] != nil {
					queue = append(queue, s)
				}
			}
		}
		if sr, ok := csv.Metadata.Annotations["olm.skipRange"]; ok {
			entry.SkipRange = sr
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// readPackageManifest finds and parses the *.package.yaml file in a package directory.
func readPackageManifest(pkgDir string) (*registry.PackageManifest, error) {
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !isYAML(e.Name()) {
			continue
		}
		path := filepath.Join(pkgDir, e.Name())
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		pm, err := registry.DecodePackageManifest(f)
		f.Close()
		if err != nil {
			continue // not a package manifest
		}
		if pm.PackageName != "" {
			return pm, nil
		}
	}
	return nil, fmt.Errorf("no package manifest found in %s", pkgDir)
}

func isYAML(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".yaml" || ext == ".yml"
}

// crdGroup returns the group for a CRD. If an explicit group is provided it is
// returned as-is; otherwise the group is extracted from the CRD name field, which
// follows the convention "<plural>.<group>" (e.g. "etcdclusters.etcd.database.coreos.com").
func crdGroup(explicitGroup, crdName string) string {
	if explicitGroup != "" {
		return explicitGroup
	}
	if idx := strings.IndexByte(crdName, '.'); idx >= 0 {
		return crdName[idx+1:]
	}
	return ""
}
