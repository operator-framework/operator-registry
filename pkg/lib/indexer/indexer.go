package indexer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/blang/semver"
	yaml3 "github.com/ghodss/yaml"
	"github.com/operator-framework/api/pkg/operators"
	"io"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"sync"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
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

	for bundleImage, bundleDir := range bundles {
		go func(bundleImage string, bundleDir bundleDirPrefix) {
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
			exporter := bundle.NewExporterForBundle(bundleImage, downloadPath, request.ContainerTool)
			if err := exporter.Export(); err != nil {
				err = fmt.Errorf("exporting bundle image:%s failed with %s", bundleImage, err)
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}
			if err := ensureCSVFields(downloadPath, bundleDir); err != nil {
				err = fmt.Errorf("exporting bundle image:%s : cannot ensure required fields on csv: %v", bundleImage, err)
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(bundleImage, bundleDir)
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
	pkgName, name, bundleVersion, replaces string
	skips                                  map[string]struct{}
}

func getBundlesToExport(dbQuerier pregistry.Query, packages []string) (map[string]bundleDirPrefix, error) {
	bundleMap := make(map[string]bundleDirPrefix)
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
				var replaces string
				var skips map[string]struct{}
				if bundleInfo, ok := bundleMap[node.BundlePath]; ok {
					replaces = bundleInfo.replaces
					skips = bundleInfo.skips
				}
				if skips == nil {
					skips = map[string]struct{}{}
				}
				skips[replaces] = struct{}{}
				for replace := range replaceNodes {
					if replace.CsvName == replaces {
						continue
					}
					if _, ok := channel.Nodes[replace]; !ok {
						//synthetic entry
						skips[replace.CsvName] = struct{}{}
						continue
					}
					if len(replaces) == 0 {
						replaces = replace.CsvName
						versionMap[replaces] = replace.Version
						continue
					}

					if replaceDepth == nil {
						replaceDepth, err = dbQuerier.GetBundleReplacesDepth(context.TODO(), packageName, node.CsvName)
					}
					if err == nil && replaceDepth[replaces] != replaceDepth[replace.CsvName] {
						if replaceDepth[replaces] < replaceDepth[replace.CsvName] {
							replaces = replace.CsvName
							versionMap[replaces] = replace.Version
						} else {
							skips[replace.CsvName] = struct{}{}
						}
						continue
					}

					replacesSemver, replacesErr := semver.Make(versionMap[replaces])
					newSemver, newErr := semver.Make(replace.Version)
					if replacesErr == nil && newErr == nil && !replacesSemver.EQ(newSemver) {
						if replacesSemver.LT(newSemver) {
							replaces = replace.CsvName
							versionMap[replaces] = replace.Version
						} else {
							skips[replace.CsvName] = struct{}{}
						}
						continue
					}
					return nil, fmt.Errorf("multiple replaces on package %s, bundle version %s: %s and %s", packageName, node.CsvName, replaces, replace.CsvName)
				}
				bundleMap[node.BundlePath] = bundleDirPrefix{
					name:          node.CsvName,
					pkgName:       packageName,
					bundleVersion: node.Version,
					skips:         skips,
					replaces:      replaces,
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

func ensureCSVFields(bundleDir string, bundleInfo bundleDirPrefix) error {
	dirContent, err := ioutil.ReadDir(bundleDir)
	if err != nil {
		return fmt.Errorf("error reading bundle directory %s, %v", bundleDir, err)
	}

	var foundCSV bool
	for _, f := range dirContent {
		if f.IsDir() {
			continue
		}
		csvFile, err := os.OpenFile(path.Join(bundleDir, f.Name()), os.O_RDWR, 0)
		if err != nil {
			continue
		}
		defer csvFile.Close()
		unstructuredCSV := unstructured.Unstructured{}

		decoder := k8syaml.NewYAMLOrJSONDecoder(csvFile, 30)
		if err = decoder.Decode(&unstructuredCSV); err != nil {
			continue
		}

		if unstructuredCSV.GetKind() != operators.ClusterServiceVersionKind {
			continue
		}

		if foundCSV {
			return fmt.Errorf("more than one ClusterServiceVersion is found in bundle")
		}

		csv := pregistry.ClusterServiceVersion{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredCSV.UnstructuredContent(),
			&csv); err != nil {
			return err
		}
		foundCSV = true
		var modified bool
		csvReplaces, err := csv.GetReplaces()
		if err != nil {
			return err
		}
		csvSkips, err := csv.GetSkips()
		if err != nil {
			return err
		}
		skipMap := make(map[string]struct{})
		for _, s := range csvSkips {
			skipMap[s] = struct{}{}
			if s == bundleInfo.replaces {
				bundleInfo.replaces = ""
			}
		}
		var updatedSkips bool
		for s := range bundleInfo.skips {
			if _, ok := skipMap[s]; !ok {
				if s == bundleInfo.replaces || s == csvReplaces {
					// don't add a skips entry already in replaces
					continue
				}
				csvSkips = append(csvSkips, s)
				skipMap[s] = struct{}{}
				updatedSkips = true
			}
		}
		if len(bundleInfo.replaces) > 0 && csvReplaces != bundleInfo.replaces {
			if err := csv.SetReplaces(bundleInfo.replaces); err != nil {
				return err
			}
			if _, ok := skipMap[csvReplaces]; !ok && len(csvReplaces) > 0 {
				csvSkips = append(csvSkips, csvReplaces)
				updatedSkips = true
			}
			modified = true
		}
		if updatedSkips {
			sort.Strings(csvSkips)
			if err := csv.SetSkips(csvSkips); err != nil {
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
			rawOutput, err := json.Marshal(unstructuredCSV)
			if err != nil {
				return fmt.Errorf("unable to marshal modified csv: %v", err)
			}

			csvFile.Seek(0, 0)
			_, _, isJSON := k8syaml.GuessJSONStream(csvFile, 30)

			// attempt to preserve original format
			if !isJSON {
				if rawYAML, err := yaml3.JSONToYAML(rawOutput); err == nil {
					rawOutput = rawYAML
				}
			}

			if err := csvFile.Truncate(0); err != nil {
				return fmt.Errorf("error clearing old csv file")
			}
			if _, err := csvFile.Seek(0, 0); err != nil {
				return fmt.Errorf("error writing to modified csv")
			}
			csvFile.Write(rawOutput)
		}
	}
	if !foundCSV {
		return fmt.Errorf("no ClusterServiceVersion object found in %s", bundleDir)
	}
	return nil
}
