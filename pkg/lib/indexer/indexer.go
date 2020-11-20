package indexer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/blang/semver"
	yaml3 "github.com/ghodss/yaml"
	"github.com/operator-framework/api/pkg/operators"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry"
	"github.com/operator-framework/operator-registry/pkg/image/execregistry"
	"github.com/operator-framework/operator-registry/pkg/lib/bundle"
	"github.com/operator-framework/operator-registry/pkg/lib/certs"
	"github.com/operator-framework/operator-registry/pkg/lib/registry"
	pregistry "github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

const (
	defaultDockerfileName     = "index.Dockerfile"
	defaultImageTag           = "operator-registry-index:latest"
	defaultDatabaseFolder     = "database"
	defaultDatabaseFile       = "index.db"
	tmpDirPrefix              = "index_tmp_"
	tmpBuildDirPrefix         = "index_build_tmp"
	concurrencyLimitForExport = 10
)

// ImageIndexer is a struct implementation of the Indexer interface
type ImageIndexer struct {
	DockerfileGenerator    containertools.DockerfileGenerator
	CommandRunner          containertools.CommandRunner
	LabelReader            containertools.LabelReader
	RegistryAdder          registry.RegistryAdder
	RegistryDeleter        registry.RegistryDeleter
	RegistryPruner         registry.RegistryPruner
	RegistryStrandedPruner registry.RegistryStrandedPruner
	RegistryDeprecator     registry.RegistryDeprecator
	BuildTool              containertools.ContainerTool
	PullTool               containertools.ContainerTool
	Logger                 *logrus.Entry
}

// AddToIndexRequest defines the parameters to send to the AddToIndex API
type AddToIndexRequest struct {
	Generate          bool
	Permissive        bool
	BinarySourceImage string
	FromIndex         string
	OutDockerfile     string
	Bundles           []string
	Tag               string
	Mode              pregistry.Mode
	CaFile            string
	SkipTLS           bool
	Overwrite         bool
}

// AddToIndex is an aggregate API used to generate a registry index image with additional bundles
func (i ImageIndexer) AddToIndex(request AddToIndexRequest) error {
	buildDir, outDockerfile, cleanup, err := buildContext(request.Generate, request.OutDockerfile)
	defer cleanup()
	if err != nil {
		return err
	}

	databasePath, err := i.extractDatabase(buildDir, request.FromIndex, request.CaFile, request.SkipTLS)
	if err != nil {
		return err
	}

	// Run opm registry add on the database
	addToRegistryReq := registry.AddToRegistryRequest{
		Bundles:       request.Bundles,
		InputDatabase: databasePath,
		Permissive:    request.Permissive,
		Mode:          request.Mode,
		SkipTLS:       request.SkipTLS,
		ContainerTool: i.PullTool,
		Overwrite:     request.Overwrite,
	}

	// Add the bundles to the registry
	err = i.RegistryAdder.AddToRegistry(addToRegistryReq)
	if err != nil {
		i.Logger.WithError(err).Debugf("unable to add bundle to registry")
		return err
	}

	// generate the dockerfile
	dockerfile := i.DockerfileGenerator.GenerateIndexDockerfile(request.BinarySourceImage, databasePath)
	err = write(dockerfile, outDockerfile, i.Logger)
	if err != nil {
		return err
	}

	if request.Generate {
		return nil
	}

	// build the dockerfile
	err = build(outDockerfile, request.Tag, i.CommandRunner, i.Logger)
	if err != nil {
		return err
	}

	return nil
}

// DeleteFromIndexRequest defines the parameters to send to the DeleteFromIndex API
type DeleteFromIndexRequest struct {
	Generate          bool
	Permissive        bool
	BinarySourceImage string
	FromIndex         string
	OutDockerfile     string
	Tag               string
	Operators         []string
	SkipTLS           bool
	CaFile            string
}

// DeleteFromIndex is an aggregate API used to generate a registry index image
// without specific operators
func (i ImageIndexer) DeleteFromIndex(request DeleteFromIndexRequest) error {
	buildDir, outDockerfile, cleanup, err := buildContext(request.Generate, request.OutDockerfile)
	defer cleanup()
	if err != nil {
		return err
	}

	databasePath, err := i.extractDatabase(buildDir, request.FromIndex, request.CaFile, request.SkipTLS)
	if err != nil {
		return err
	}

	// Run opm registry delete on the database
	deleteFromRegistryReq := registry.DeleteFromRegistryRequest{
		Packages:      request.Operators,
		InputDatabase: databasePath,
		Permissive:    request.Permissive,
	}

	// Delete the bundles from the registry
	err = i.RegistryDeleter.DeleteFromRegistry(deleteFromRegistryReq)
	if err != nil {
		return err
	}

	// generate the dockerfile
	dockerfile := i.DockerfileGenerator.GenerateIndexDockerfile(request.BinarySourceImage, databasePath)
	err = write(dockerfile, outDockerfile, i.Logger)
	if err != nil {
		return err
	}

	if request.Generate {
		return nil
	}

	// build the dockerfile
	err = build(outDockerfile, request.Tag, i.CommandRunner, i.Logger)
	if err != nil {
		return err
	}

	return nil
}

// PruneStrandedFromIndexRequest defines the parameters to send to the PruneStrandedFromIndex API
type PruneStrandedFromIndexRequest struct {
	Generate          bool
	BinarySourceImage string
	FromIndex         string
	OutDockerfile     string
	Tag               string
	CaFile            string
	SkipTLS           bool
}

// PruneStrandedFromIndex is an aggregate API used to generate a registry index image
// that has removed stranded bundles from the index
func (i ImageIndexer) PruneStrandedFromIndex(request PruneStrandedFromIndexRequest) error {
	buildDir, outDockerfile, cleanup, err := buildContext(request.Generate, request.OutDockerfile)
	defer cleanup()
	if err != nil {
		return err
	}

	databasePath, err := i.extractDatabase(buildDir, request.FromIndex, request.CaFile, request.SkipTLS)
	if err != nil {
		return err
	}

	// Run opm registry prune-stranded on the database
	pruneStrandedFromRegistryReq := registry.PruneStrandedFromRegistryRequest{
		InputDatabase: databasePath,
	}

	// Delete the stranded bundles from the registry
	err = i.RegistryStrandedPruner.PruneStrandedFromRegistry(pruneStrandedFromRegistryReq)
	if err != nil {
		return err
	}

	// generate the dockerfile
	dockerfile := i.DockerfileGenerator.GenerateIndexDockerfile(request.BinarySourceImage, databasePath)
	err = write(dockerfile, outDockerfile, i.Logger)
	if err != nil {
		return err
	}

	if request.Generate {
		return nil
	}

	// build the dockerfile
	err = build(outDockerfile, request.Tag, i.CommandRunner, i.Logger)
	if err != nil {
		return err
	}
	return nil
}

// PruneFromIndexRequest defines the parameters to send to the PruneFromIndex API
type PruneFromIndexRequest struct {
	Generate          bool
	Permissive        bool
	BinarySourceImage string
	FromIndex         string
	OutDockerfile     string
	Tag               string
	Packages          []string
	CaFile            string
	SkipTLS           bool
}

func (i ImageIndexer) PruneFromIndex(request PruneFromIndexRequest) error {
	buildDir, outDockerfile, cleanup, err := buildContext(request.Generate, request.OutDockerfile)
	defer cleanup()
	if err != nil {
		return err
	}

	databasePath, err := i.extractDatabase(buildDir, request.FromIndex, request.CaFile, request.SkipTLS)
	if err != nil {
		return err
	}

	// Run opm registry prune on the database
	pruneFromRegistryReq := registry.PruneFromRegistryRequest{
		Packages:      request.Packages,
		InputDatabase: databasePath,
		Permissive:    request.Permissive,
	}

	// Prune the bundles from the registry
	err = i.RegistryPruner.PruneFromRegistry(pruneFromRegistryReq)
	if err != nil {
		return err
	}

	// generate the dockerfile
	dockerfile := i.DockerfileGenerator.GenerateIndexDockerfile(request.BinarySourceImage, databasePath)
	err = write(dockerfile, outDockerfile, i.Logger)
	if err != nil {
		return err
	}

	if request.Generate {
		return nil
	}

	// build the dockerfile
	err = build(outDockerfile, request.Tag, i.CommandRunner, i.Logger)
	if err != nil {
		return err
	}

	return nil
}

// extractDatabase sets a temp directory for unpacking an image
func (i ImageIndexer) extractDatabase(buildDir, fromIndex, caFile string, skipTLS bool) (string, error) {
	tmpDir, err := ioutil.TempDir("./", tmpDirPrefix)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	databaseFile, err := i.getDatabaseFile(tmpDir, fromIndex, caFile, skipTLS)
	if err != nil {
		return "", err
	}
	// copy the index to the database folder in the build directory
	return copyDatabaseTo(databaseFile, filepath.Join(buildDir, defaultDatabaseFolder))
}

func (i ImageIndexer) getDatabaseFile(workingDir, fromIndex, caFile string, skipTLS bool) (string, error) {
	if fromIndex == "" {
		return path.Join(workingDir, defaultDatabaseFile), nil
	}

	// Pull the fromIndex
	i.Logger.Infof("Pulling previous image %s to get metadata", fromIndex)

	var reg image.Registry
	var rerr error
	switch i.PullTool {
	case containertools.NoneTool:
		rootCAs, err := certs.RootCAs(caFile)
		if err != nil {
			return "", fmt.Errorf("failed to get RootCAs: %v", err)
		}
		reg, rerr = containerdregistry.NewRegistry(containerdregistry.SkipTLS(skipTLS), containerdregistry.WithLog(i.Logger), containerdregistry.WithRootCAs(rootCAs))
	case containertools.PodmanTool:
		fallthrough
	case containertools.DockerTool:
		reg, rerr = execregistry.NewRegistry(i.PullTool, i.Logger, containertools.SkipTLS(skipTLS))
	}
	if rerr != nil {
		return "", rerr
	}
	defer func() {
		if err := reg.Destroy(); err != nil {
			i.Logger.WithError(err).Warn("error destroying local cache")
		}
	}()

	imageRef := image.SimpleReference(fromIndex)

	if err := reg.Pull(context.TODO(), imageRef); err != nil {
		return "", err
	}

	// Get the old index image's dbLocationLabel to find this path
	labels, err := reg.Labels(context.TODO(), imageRef)
	if err != nil {
		return "", err
	}

	dbLocation, ok := labels[containertools.DbLocationLabel]
	if !ok {
		return "", fmt.Errorf("index image %s missing label %s", fromIndex, containertools.DbLocationLabel)
	}

	if err := reg.Unpack(context.TODO(), imageRef, workingDir); err != nil {
		return "", err
	}

	return path.Join(workingDir, dbLocation), nil
}

func copyDatabaseTo(databaseFile, targetDir string) (string, error) {
	// create the containing folder if it doesn't exist
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		if err := os.MkdirAll(targetDir, 0777); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}

	// Open the database file in the working dir
	from, err := os.OpenFile(databaseFile, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return "", err
	}
	defer from.Close()

	dbFile := path.Join(targetDir, defaultDatabaseFile)

	// define the path to copy to the database/index.db file
	to, err := os.OpenFile(dbFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return "", err
	}
	defer to.Close()

	// copy to the destination directory
	_, err = io.Copy(to, from)
	return to.Name(), err
}

func buildContext(generate bool, requestedDockerfile string) (buildDir, outDockerfile string, cleanup func(), err error) {
	if generate {
		buildDir = "./"
		if len(requestedDockerfile) == 0 {
			outDockerfile = defaultDockerfileName
		} else {
			outDockerfile = requestedDockerfile
		}
		cleanup = func() {}
		return
	}

	// set a temp directory for building the new image
	buildDir, err = ioutil.TempDir(".", tmpBuildDirPrefix)
	if err != nil {
		return
	}
	cleanup = func() {
		os.RemoveAll(buildDir)
	}

	if len(requestedDockerfile) > 0 {
		outDockerfile = requestedDockerfile
		return
	}

	// generate a temp dockerfile if needed
	tempDockerfile, err := ioutil.TempFile(".", defaultDockerfileName)
	if err != nil {
		defer cleanup()
		return
	}
	outDockerfile = tempDockerfile.Name()
	cleanup = func() {
		os.RemoveAll(buildDir)
		os.Remove(outDockerfile)
	}

	return
}

func build(dockerfilePath, imageTag string, commandRunner containertools.CommandRunner, logger *logrus.Entry) error {
	if imageTag == "" {
		imageTag = defaultImageTag
	}

	logger.Debugf("building container image: %s", imageTag)

	err := commandRunner.Build(dockerfilePath, imageTag)
	if err != nil {
		return err
	}

	return nil
}

func write(dockerfileText, outDockerfile string, logger *logrus.Entry) error {
	if outDockerfile == "" {
		outDockerfile = defaultDockerfileName
	}

	logger.Infof("writing dockerfile: %s", outDockerfile)

	f, err := os.Create(outDockerfile)
	if err != nil {
		return err
	}

	_, err = f.WriteString(dockerfileText)
	if err != nil {
		return err
	}

	return nil
}

// ExportFromIndexRequest defines the parameters to send to the ExportFromIndex API
type ExportFromIndexRequest struct {
	Index         string
	Packages      []string
	DownloadPath  string
	ContainerTool containertools.ContainerTool
	CaFile        string
	SkipTLS       bool
}

// ExportFromIndex is an aggregate API used to specify operators from
// an index image
func (i ImageIndexer) ExportFromIndex(request ExportFromIndexRequest) error {
	// set a temp directory
	workingDir, err := ioutil.TempDir("./", tmpDirPrefix)
	if err != nil {
		return err
	}
	defer os.RemoveAll(workingDir)

	// extract the index database to the file
	databaseFile, err := i.getDatabaseFile(workingDir, request.Index, request.CaFile, request.SkipTLS)
	if err != nil {
		return err
	}

	db, err := sql.Open("sqlite3", databaseFile)
	if err != nil {
		return err
	}
	defer db.Close()

	dbQuerier := sqlite.NewSQLLiteQuerierFromDb(db)

	// fetch all packages from the index image if packages is empty
	if len(request.Packages) == 0 {
		request.Packages, err = dbQuerier.ListPackages(context.TODO())
		if err != nil {
			return err
		}
	}

	bundles, err := getBundlesToExport(dbQuerier, request.Packages)
	if err != nil {
		return err
	}

	i.Logger.Infof("Preparing to pull bundles %+q", bundles)

	// Creating downloadPath dir
	if err := os.MkdirAll(request.DownloadPath, 0777); err != nil {
		return err
	}

	var errs []error
	var wg sync.WaitGroup
	wg.Add(len(bundles))
	var mu = &sync.Mutex{}

	sem := make(chan struct{}, concurrencyLimitForExport)

	for bundleDir, bundleInfo := range bundles {
		go func(bundleDir bundleDirPrefix, bundleInfo bundleExportInfo) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() {
				<-sem
			}()

			// generate a random folder name if bundle version is empty
			if bundleDir.bundleVersion == "" {
				bundleDir.bundleVersion = strconv.Itoa(rand.Intn(10000))
			}
			downloadPath := filepath.Join(request.DownloadPath, bundleDir.pkgName, bundleDir.bundleVersion)
			exporter := bundle.NewExporterForBundle(bundleInfo.bundlePath, downloadPath, request.ContainerTool)
			if err := exporter.Export(); err != nil {
				err = fmt.Errorf("exporting bundle image:%s failed with %s", bundleInfo.bundlePath, err)
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}
			if err := ensureCSVFields(downloadPath, bundleInfo); err != nil {
				err = fmt.Errorf("exporting bundle image:%s : cannot ensure required fields on csv: %v", bundleInfo.bundlePath, err)
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(bundleDir, bundleInfo)
	}
	// Wait for all the go routines to finish export
	wg.Wait()

	if errs != nil {
		return utilerrors.NewAggregate(errs)
	}

	for _, packageName := range request.Packages {
		err := generatePackageYaml(dbQuerier, packageName, filepath.Join(request.DownloadPath, packageName))
		if err != nil {
			errs = append(errs, err)
		}
	}
	return utilerrors.NewAggregate(errs)
}

type bundleDirPrefix struct {
	pkgName, bundleVersion string
}

type bundleExportInfo struct {
	name       string
	bundlePath string
	replaces   string
	skips      map[string]struct{}
}

func getBundlesToExport(dbQuerier pregistry.Query, packages []string) (map[bundleDirPrefix]bundleExportInfo, error) {
	bundleMap := make(map[bundleDirPrefix]bundleExportInfo)
	graphLoader := &sqlite.SQLGraphLoader{
		Querier: dbQuerier,
	}
	for _, packageName := range packages {
		bundleGraph, err := graphLoader.Generate(packageName)
		if err != nil {
			return nil, err
		}
		versionMap := make(map[string]string)
		for _, channel := range bundleGraph.Channels {
			var replaceDepth map[string]int64
			for node, replaceNodes := range channel.Nodes {
				bundleKey := bundleDirPrefix{
					pkgName:       packageName,
					bundleVersion: node.Version,
				}
				var replaces string
				var skips map[string]struct{}
				if bundleInfo, ok := bundleMap[bundleKey]; ok {
					if bundleInfo.name != node.CsvName || bundleInfo.bundlePath != node.BundlePath {
						return nil, fmt.Errorf("cannot export bundle version %s for package %s: multiple CSVs found", node.Version, packageName)
					}
					replaces = bundleInfo.replaces
					skips = bundleInfo.skips
				}
				if skips == nil {
					skips = map[string]struct{}{}
				}
				if len(replaces) > 0 {
					skips[replaces] = struct{}{}
				}
				for replace := range replaceNodes {
					if replace.CsvName == replaces {
						continue
					}

					skips[replace.CsvName] = struct{}{}

					if _, ok := channel.Nodes[replace]; !ok {
						//synthetic entry
						continue
					}
					if len(replaces) == 0 {
						replaces = replace.CsvName
						versionMap[replaces] = replace.Version
						continue
					}

					var err error
					if replaceDepth == nil {
						replaceDepth, err = dbQuerier.GetBundleReplacesDepth(context.TODO(), packageName, node.CsvName)
					}
					if err == nil && replaceDepth[replaces] != replaceDepth[replace.CsvName] {
						if replaceDepth[replaces] > replaceDepth[replace.CsvName] {
							replaces = replace.CsvName
							versionMap[replaces] = replace.Version
						}
						continue
					}

					// 2 replaces versions at the same depth on replaces chains of different channels
					// Choose the newer semver as the replaces
					replacesSemver, err := semver.Make(versionMap[replaces])
					if err != nil {
						return nil, fmt.Errorf("failed to parse replaces version %s for %s: %v", versionMap[replaces], node.CsvName, err)
					}
					newSemver, err := semver.Make(replace.Version)
					if err != nil {
						return nil, fmt.Errorf("failed to parse replaces version %s for %s: %v", replace.Version, node.CsvName, err)
					}

					if replacesSemver.EQ(newSemver) {
						return nil, fmt.Errorf("multiple replaces on package %s, bundle version %s", packageName, node.CsvName)
					}

					if replacesSemver.LT(newSemver) {
						replaces = replace.CsvName
						versionMap[replaces] = replace.Version
					}
				}
				delete(skips, replaces)
				bundleMap[bundleKey] = bundleExportInfo{
					name:       node.CsvName,
					skips:      skips,
					replaces:   replaces,
					bundlePath: node.BundlePath,
				}
			}
		}
	}

	return bundleMap, nil
}

func generatePackageYaml(dbQuerier pregistry.Query, packageName, downloadPath string) error {
	var errs []error

	defaultChannel, err := dbQuerier.GetDefaultChannelForPackage(context.TODO(), packageName)
	if err != nil {
		return err
	}

	channelList, err := dbQuerier.ListChannels(context.TODO(), packageName)
	if err != nil {
		return err
	}

	channels := []pregistry.PackageChannel{}
	for _, ch := range channelList {
		csvName, err := dbQuerier.GetCurrentCSVNameForChannel(context.TODO(), packageName, ch)
		if err != nil {
			err = fmt.Errorf("error exporting bundle from image: %s", err)
			errs = append(errs, err)
			continue
		}
		channels = append(channels,
			pregistry.PackageChannel{
				Name:           ch,
				CurrentCSVName: csvName,
			})
	}

	manifest := pregistry.PackageManifest{
		PackageName:        packageName,
		DefaultChannelName: defaultChannel,
		Channels:           channels,
	}

	manifestBytes, err := yaml.Marshal(&manifest)
	if err != nil {
		errs = append(errs, err)
		return utilerrors.NewAggregate(errs)
	}

	err = bundle.WriteFile("package.yaml", downloadPath, manifestBytes)
	if err != nil {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}

// DeprecateFromIndexRequest defines the parameters to send to the PruneFromIndex API
type DeprecateFromIndexRequest struct {
	Generate          bool
	Permissive        bool
	BinarySourceImage string
	FromIndex         string
	OutDockerfile     string
	Bundles           []string
	Tag               string
	CaFile            string
	SkipTLS           bool
}

// DeprecateFromIndex takes a DeprecateFromIndexRequest and deprecates the requested
// bundles.
func (i ImageIndexer) DeprecateFromIndex(request DeprecateFromIndexRequest) error {
	buildDir, outDockerfile, cleanup, err := buildContext(request.Generate, request.OutDockerfile)
	defer cleanup()
	if err != nil {
		return err
	}

	databasePath, err := i.extractDatabase(buildDir, request.FromIndex, request.CaFile, request.SkipTLS)
	if err != nil {
		return err
	}

	// Run opm registry prune on the database
	deprecateFromRegistryReq := registry.DeprecateFromRegistryRequest{
		Bundles:       request.Bundles,
		InputDatabase: databasePath,
		Permissive:    request.Permissive,
	}

	// Prune the bundles from the registry
	err = i.RegistryDeprecator.DeprecateFromRegistry(deprecateFromRegistryReq)
	if err != nil {
		return err
	}

	// generate the dockerfile
	dockerfile := i.DockerfileGenerator.GenerateIndexDockerfile(request.BinarySourceImage, databasePath)
	err = write(dockerfile, outDockerfile, i.Logger)
	if err != nil {
		return err
	}

	if request.Generate {
		return nil
	}

	// build the dockerfile with requested tooling
	err = build(outDockerfile, request.Tag, i.CommandRunner, i.Logger)
	if err != nil {
		return err
	}

	return nil
}

func ensureCSVFields(bundleDir string, bundleInfo bundleExportInfo) error {
	dirContent, err := ioutil.ReadDir(bundleDir)
	if err != nil {
		return fmt.Errorf("error reading bundle directory %s, %v", bundleDir, err)
	}

	for _, f := range dirContent {
		if f.IsDir() {
			continue
		}
		unstructuredCSV := unstructured.Unstructured{}

		csvFile, err := os.OpenFile(path.Join(bundleDir, f.Name()), os.O_RDONLY, 0)
		if err != nil {
			continue
		}
		err = k8syaml.NewYAMLOrJSONDecoder(csvFile, 30).Decode(&unstructuredCSV)
		csvFile.Close()
		if err != nil {
			continue
		}

		if unstructuredCSV.GetKind() != operators.ClusterServiceVersionKind {
			continue
		}

		csv := pregistry.ClusterServiceVersion{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredCSV.UnstructuredContent(),
			&csv); err != nil {
			return err
		}
		csvReplaces, err := csv.GetReplaces()
		if err != nil {
			return err
		}
		csvSkips, err := csv.GetSkips()
		if err != nil {
			return err
		}
		sort.Strings(csvSkips)
		skipMap := make(map[string]struct{})
		for _, skip := range csvSkips {
			skipMap[skip] = struct{}{}
		}
		skips, replaces := getSkipsReplaces(bundleExportInfo{skips: skipMap, replaces: csvReplaces}, bundleInfo)
		var modified bool
		if replaces != csvReplaces {
			if err := csv.SetReplaces(bundleInfo.replaces); err != nil {
				return err
			}
			modified = true
		}
		if strings.Join(csvSkips, ",") != strings.Join(skips, ",") {
			if err := csv.SetSkips(skips); err != nil {
				return err
			}
			modified = true
		}
		if modified {
			unstructuredCSV, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&csv)
			if err != nil {
				return fmt.Errorf("unable to convert modified csv: %v", err)
			}

			// remove creationTimestamp: null
			unstructured.RemoveNestedField(unstructuredCSV, "metadata", "creationTimestamp")

			if err := updateJSONOrYAMLFile(path.Join(bundleDir, f.Name()), unstructuredCSV); err != nil {
				return err
			}
		}

		return nil
	}
	return fmt.Errorf("no ClusterServiceVersion object found in %s", bundleDir)
}

// getSkipsReplaces produces a merged skips/replaces for a bundle, using contents of its csv
// and the upgrade graph in the index.
func getSkipsReplaces(csvInfo, indexInfo bundleExportInfo) ([]string, string) {
	// merge skips lists
	for skip := range indexInfo.skips {
		if skip != indexInfo.replaces {
			csvInfo.skips[skip] = struct{}{}
		}
	}

	if _, ok := csvInfo.skips[csvInfo.replaces]; ok {
		delete(csvInfo.skips, csvInfo.replaces)
	}

	// if new replaces is pre sent on skip list defined in csv, ignore
	if _, ok := csvInfo.skips[indexInfo.replaces]; ok {
		indexInfo.replaces = ""
	}

	// Prefer the replaces on the index
	if len(indexInfo.replaces) > 0 && csvInfo.replaces != indexInfo.replaces {
		if len(csvInfo.replaces) > 0 {
			csvInfo.skips[csvInfo.replaces] = struct{}{}
		}
		csvInfo.replaces = indexInfo.replaces
	}

	skips := make([]string, 0)
	for s := range csvInfo.skips {
		skips = append(skips, s)
	}
	sort.Strings(skips)
	return skips, csvInfo.replaces
}

// Update a JSON or YAML file. The data is first converted to JSON.
// It is only converted to yaml if the original file was not in JSON format.
// Only the JSON tags on the data are used in marshalling the data
func updateJSONOrYAMLFile(file string, data interface{}) error {
	f, err := os.OpenFile(file, os.O_RDWR, 0)
	defer f.Close()
	if err != nil {
		return err
	}
	_, _, isJSON := k8syaml.GuessJSONStream(f, 30)

	rawOutput, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("unable to marshal contents for %s to JSON: %v", file, err)
	}
	if !isJSON {
		// best attempt a converting to yaml
		if rawYAML, err := yaml3.JSONToYAML(rawOutput); err == nil {
			rawOutput = rawYAML
		}
	}
	if err := f.Truncate(0); err != nil {
		return fmt.Errorf("error truncating %s", file)
	}
	if _, err := f.Seek(0, 0); err != nil {
		return fmt.Errorf("error writing to modified csv")
	}
	n, err := f.Write(rawOutput)
	if err != nil {
		return fmt.Errorf("failed to write to %s: %v", file, err)
	}
	if n < len(rawOutput) {
		return fmt.Errorf("incomplete write to %s: %d/%d bytes written", file, n, len(rawOutput))
	}
	return nil
}
