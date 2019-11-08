package indexer

import (
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/lib/registry"

	"github.com/sirupsen/logrus"
)

const (
	defaultDockerfileName = "index.Dockerfile"
	defaultImageTag = "operator-registry-index:latest"
	defaultDatabaseFolder = "database"
	defaultDatabaseFile = "index.db"
)

// ImageIndexer is a struct implementation of the Indexer interface
type ImageIndexer struct {
	DockerfileGenerator containertools.DockerfileGenerator
	CommandRunner containertools.CommandRunner
	LabelReader containertools.LabelReader
	ImageReader containertools.ImageReader
	RegistryAdder registry.RegistryAdder
	RegistryDeleter registry.RegistryDeleter
	ContainerTool string
	Logger *logrus.Entry
}

// AddToIndexRequest defines the parameters to send to the AddToIndex API
type AddToIndexRequest struct {
	Generate bool
	Permissive bool
	BinarySourceImage string
	FromIndex string
	OutDockerfile string
	Bundles []string
	Tag string
}

// AddToIndex is an aggregate API used to generate a registry index image with additional bundles
func (i ImageIndexer) AddToIndex(request AddToIndexRequest) error {
	databaseFile := defaultDatabaseFile

	// set a temp directory
	workingDir, err := ioutil.TempDir("./", "index_tmp")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workingDir)

	// Pull the fromIndex
	if request.FromIndex != "" {
		i.Logger.Infof("Pulling previous image %s to get metadata", request.FromIndex)

		// Get the old index image's dbLocationLabel to find this path
		labels, err := i.LabelReader.GetLabelsFromImage(request.FromIndex)
		if err != nil {
			return err
		}
		if dbLocation, ok := labels[containertools.DbLocationLabel]; ok {
			// extract the database to the file
			err = i.ImageReader.GetImageData(request.FromIndex, workingDir)
			if err != nil {
				return err
			}

			databaseFile = path.Join(workingDir, dbLocation)
		}
	} else {
		databaseFile = path.Join(workingDir, databaseFile)
	}

	// Run opm registry add on the database
	addToRegistryReq := registry.AddToRegistryRequest{
		Bundles: request.Bundles,
		InputDatabase: databaseFile,
		Permissive: request.Permissive,
		ContainerTool: i.ContainerTool,
	}

	// Add the bundles to the registry
	err = i.RegistryAdder.AddToRegistry(addToRegistryReq)
	if err != nil {
		return err
	}

	// write the dockerfile to disk if generate is set, otherwise shell out to build the image
	if request.Generate {
		err = i.generateDockerfile(request.BinarySourceImage, request.OutDockerfile, databaseFile)
		if err != nil {
			return err
		}
	} else {
		err = i.buildDockerfile(request.BinarySourceImage, workingDir, request.Tag)
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteFromIndexRequest defines the parameters to send to the DeleteFromIndex API
type DeleteFromIndexRequest struct {
	Generate bool
	Permissive bool
	BinarySourceImage string
	FromIndex string
	OutDockerfile string
	Tag string
	Operators []string
}

// DeleteFromIndex is an aggregate API used to generate a registry index image 
// without specific operators
func (i ImageIndexer) DeleteFromIndex(request DeleteFromIndexRequest) error {
	databaseFile := defaultDatabaseFile

	// set a temp directory
	workingDir, err := ioutil.TempDir("./", "index_tmp")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workingDir)

	// Pull the fromIndex
	if request.FromIndex != "" {
		i.Logger.Infof("Pulling previous image %s to get metadata", request.FromIndex)

		// Get the old index image's dbLocationLabel to find this path
		labels, err := i.LabelReader.GetLabelsFromImage(request.FromIndex)
		if err != nil {
			return err
		}
		if dbLocation, ok := labels[containertools.DbLocationLabel]; ok {
			i.Logger.Infof("Previous db location %s", dbLocation)

			// extract the database to the file
			err = i.ImageReader.GetImageData(request.FromIndex, workingDir)
			if err != nil {
				return err
			}

			databaseFile = path.Join(workingDir, dbLocation)
		}
	} else {
		databaseFile = path.Join(workingDir, databaseFile)
	}

	// Run opm registry add on the database
	deleteFromRegistryReq := registry.DeleteFromRegistryRequest{
		Packages: request.Operators,
		InputDatabase: databaseFile,
		Permissive: request.Permissive,
	}

	// Add the bundles to the registry
	err = i.RegistryDeleter.DeleteFromRegistry(deleteFromRegistryReq)
	if err != nil {
		return err
	}

	// write the dockerfile to disk if generate is set, otherwise shell out to build the image
	if request.Generate {
		err = i.generateDockerfile(request.BinarySourceImage, request.OutDockerfile, databaseFile)
		if err != nil {
			return err
		}
	} else {
		err = i.buildDockerfile(request.BinarySourceImage, workingDir, request.Tag)
		if err != nil {
			return err
		}
	}

	return nil
}

func (i ImageIndexer) generateDockerfile(binarySourceImage, outDockerfile, databaseFile string) error {
	databaseFolder := defaultDatabaseFolder

	// create the dockerfile
	dockerfile := i.DockerfileGenerator.GenerateIndexDockerfile(binarySourceImage, databaseFolder)

	// write the dockerfile to root
	err := write(dockerfile, outDockerfile, i.Logger)
	if err != nil {
		return err
	}

	// copy the index to a permanent database folder

	// create the database/ folder if it doesn't exist
	if _, err := os.Stat(defaultDatabaseFolder); os.IsNotExist(err) {
		os.Mkdir(defaultDatabaseFolder, 0777)
	}

	// Open the database file in the working dir
	from, err := os.OpenFile(databaseFile, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer from.Close()

	dbFile := path.Join(defaultDatabaseFolder, defaultDatabaseFile)

	// define the path to copy to the database/index.db file
	to, err := os.OpenFile(dbFile, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer to.Close()

	// copy to the destination directory
	_, err = io.Copy(to, from)
	if err != nil {
		return err
	}

	return nil
}

func (i ImageIndexer) buildDockerfile(binarySourceImage, workingDir, tag string) error {
	// create the dockerfile
	dockerfile := i.DockerfileGenerator.GenerateIndexDockerfile(binarySourceImage, workingDir)

	// write the dockerfile to temp file
	tempDockerfile, err := ioutil.TempFile(".", defaultDockerfileName)
	if err != nil {
		return err
	}
	defer os.Remove(tempDockerfile.Name())

	err = write(dockerfile, tempDockerfile.Name(), i.Logger)
	if err != nil {
		return err
	}

	err = build(tempDockerfile.Name(), tag, i.CommandRunner, i.Logger)
	if err != nil {
		return err
	}

	return nil
}

func build(dockerfileText, imageTag string, commandRunner containertools.CommandRunner, logger *logrus.Entry) error {
	if imageTag == "" {
		imageTag = defaultImageTag
	}

	logger.Debugf("building container image: %s", imageTag)

	err := commandRunner.Build(dockerfileText, imageTag)
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
