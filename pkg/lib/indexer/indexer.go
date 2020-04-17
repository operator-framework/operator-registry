package indexer

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/lib/bundle"
	"github.com/operator-framework/operator-registry/pkg/lib/registry"
	pregistry "github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/sqlite"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

const (
	defaultDockerfileName = "index.Dockerfile"
	defaultImageTag       = "operator-registry-index:latest"
	defaultDatabaseFolder = "database"
	defaultDatabaseFile   = "index.db"
	tmpDirPrefix          = "index_tmp_"
	tmpBuildDirPrefix     = "index_build_tmp"
)

// ImageIndexer is a struct implementation of the Indexer interface
type ImageIndexer struct {
	DockerfileGenerator containertools.DockerfileGenerator
	CommandRunner       containertools.CommandRunner
	LabelReader         containertools.LabelReader
	ImageReader         containertools.ImageReader
	RegistryAdder       registry.RegistryAdder
	RegistryDeleter     registry.RegistryDeleter
	ContainerTool       containertools.ContainerTool
	Logger              *logrus.Entry
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
	SkipTLS           bool
}

// AddToIndex is an aggregate API used to generate a registry index image with additional bundles
func (i ImageIndexer) AddToIndex(request AddToIndexRequest) error {
	buildDir, outDockerfile, cleanup, err := buildContext(request.Generate, request.OutDockerfile)
	defer cleanup()
	if err != nil {
		return err
	}

	// set a temp directory for unpacking an image
	// this is in its own function context so that the deferred cleanup runs before we do a docker build
	// which prevents the full contents of the previous image from being in the build context
	var databasePath string
	if err := func () error {
		tmpDir, err := ioutil.TempDir("./", tmpDirPrefix)
		if err != nil {

			return err
		}
		defer os.RemoveAll(tmpDir)

		databaseFile, err := i.getDatabaseFile(tmpDir, request.FromIndex)
		if err != nil {
			return err
		}
		// copy the index to the database folder in the build directory
		if databasePath, err = copyDatabaseTo(databaseFile, filepath.Join(buildDir, defaultDatabaseFolder)); err != nil {
			return err
		}
		return nil
	}(); err != nil {
		return err
	}

	// Run opm registry add on the database
	addToRegistryReq := registry.AddToRegistryRequest{
		Bundles:       request.Bundles,
		InputDatabase: databasePath,
		Permissive:    request.Permissive,
		Mode:          request.Mode,
		SkipTLS:       request.SkipTLS,
		ContainerTool: i.ContainerTool,
	}

	// Add the bundles to the registry
	err = i.RegistryAdder.AddToRegistry(addToRegistryReq)
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

// DeleteFromIndexRequest defines the parameters to send to the DeleteFromIndex API
type DeleteFromIndexRequest struct {
	Generate          bool
	Permissive        bool
	BinarySourceImage string
	FromIndex         string
	OutDockerfile     string
	Tag               string
	Operators         []string
}

// DeleteFromIndex is an aggregate API used to generate a registry index image
// without specific operators
func (i ImageIndexer) DeleteFromIndex(request DeleteFromIndexRequest) error {
	buildDir, outDockerfile, cleanup, err := buildContext(request.Generate, request.OutDockerfile)
	defer cleanup()
	if err != nil {
		return err
	}

	// set a temp directory for unpacking an image
	// this is in its own function context so that the deferred cleanup runs before we do a docker build
	// which prevents the full contents of the previous image from being in the build context
	var databasePath string
	if err := func () error {
		tmpDir, err := ioutil.TempDir("./", tmpDirPrefix)
		if err != nil {

			return err
		}
		defer os.RemoveAll(tmpDir)

		databaseFile, err := i.getDatabaseFile(tmpDir, request.FromIndex)
		if err != nil {
			return err
		}
		// copy the index to the database folder in the build directory
		if databasePath, err = copyDatabaseTo(databaseFile, filepath.Join(buildDir, defaultDatabaseFolder)); err != nil {
			return err
		}
		return nil
	}(); err != nil {
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

func (i ImageIndexer) getDatabaseFile(workingDir, fromIndex string) (string, error) {
	if fromIndex == "" {
		return path.Join(workingDir, defaultDatabaseFile), nil
	}

	// Pull the fromIndex
	i.Logger.Infof("Pulling previous image %s to get metadata", fromIndex)

	// Get the old index image's dbLocationLabel to find this path
	labels, err := i.LabelReader.GetLabelsFromImage(fromIndex)
	if err != nil {
		return "", err
	}
	dbLocation, ok := labels[containertools.DbLocationLabel]
	if !ok {
		return "", fmt.Errorf("Index image %s missing label %s", fromIndex, containertools.DbLocationLabel)
	}

	// extract the database to the working directory
	err = i.ImageReader.GetImageData(fromIndex, workingDir)
	if err != nil {
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
	} else {
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
	to, err := os.OpenFile(dbFile, os.O_RDWR|os.O_CREATE, 0666)
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
	Package       string
	DownloadPath  string
	ContainerTool string
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
	databaseFile, err := i.WriteIndexDBFile(request.Index, workingDir, defaultDatabaseFile)
	if err != nil {
		return err
	}

	db, err := sql.Open("sqlite3", databaseFile)
	if err != nil {
		return err
	}
	defer db.Close()

	dbQuerier := sqlite.NewSQLLiteQuerierFromDb(db)
	if err != nil {
		return err
	}

	bundles, err := getBundlesToExport(dbQuerier, request.Package)
	if err != nil {
		return err
	}
	i.Logger.Infof("Preparing to pull bundles %+q", bundles)

	// Creating downloadPath dir
	if err := os.MkdirAll(request.DownloadPath, 0777); err != nil {
		return err
	}

	var errs []error
	for _, bundleImage := range bundles {
		// try to name the folder
		folderName, err := dbQuerier.GetBundleVersion(context.TODO(), bundleImage)
		if err != nil {
			return err
		}
		if folderName == "" {
			// operator-registry does not care about the folder name
			folderName = bundleImage
		}
		exporter := bundle.NewSQLExporterForBundle(bundleImage, filepath.Join(request.DownloadPath, folderName), request.ContainerTool)
		if err := exporter.Export(); err != nil {
			err = fmt.Errorf("error exporting bundle from image: %s", err)
			errs = append(errs, err)
		}
	}
	if err != nil {
		errs = append(errs, err)
		return utilerrors.NewAggregate(errs)
	}

	err = generatePackageYaml(dbQuerier, request.Package, request.DownloadPath)
	if err != nil {
		errs = append(errs, err)
	}
	return utilerrors.NewAggregate(errs)
}

func (i ImageIndexer) WriteIndexDBFile(index, workingDir, databaseFile string) (string, error) {
	if index != "" {
		i.Logger.Infof("Pulling previous image %s to get metadata", index)

		// Get the old index image's dbLocationLabel to find this path
		labels, err := i.LabelReader.GetLabelsFromImage(index)
		if err != nil {
			return "", err
		}
		if dbLocation, ok := labels[containertools.DbLocationLabel]; ok {
			i.Logger.Infof("Previous db location %s", dbLocation)

			// extract the database to the file
			err = i.ImageReader.GetImageData(index, workingDir)
			if err != nil {
				return "", err
			}

			databaseFile = path.Join(workingDir, dbLocation)
		}
	} else {
		databaseFile = path.Join(workingDir, databaseFile)
	}
	return databaseFile, nil
}

func getBundlesToExport(dbQuerier pregistry.Query, packageName string) ([]string, error) {
	bundles, err := dbQuerier.GetBundlePathsForPackage(context.TODO(), packageName)
	if err != nil {
		return nil, err
	}
	return bundles, nil
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
