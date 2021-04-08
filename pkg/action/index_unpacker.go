package action

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry"

	dircopy "github.com/otiai10/copy"
	"github.com/sirupsen/logrus"
)

const (
	defaultDockerfileName = "index.Dockerfile"
	imgUnpackTmpDirPrefix = "tmp_unpack_"
	defaultUnpackDir      = "configs"
)

type indexUnpacker struct {
	logger *logrus.Entry
	reg    *containerdregistry.Registry
	ctx    context.Context
}

type UnpackRequest struct {
	Image     string
	UnpackDir string
}

func NewIndexUnpacker(l *logrus.Entry, r *containerdregistry.Registry, ctx context.Context) indexUnpacker {
	return indexUnpacker{
		logger: l,
		reg:    r,
		ctx:    ctx,
	}
}

func (i indexUnpacker) Unpack(request UnpackRequest) error {
	i.logger.Infof("Pulling image %q to get metadata", request.Image)

	imageRef := image.SimpleReference(request.Image)
	if err := i.reg.Pull(i.ctx, imageRef); err != nil {
		return fmt.Errorf("error pulling image from remote registry. Error: %v", err)
	}
	// Get the index image's ConfigsLocation Label to find this path
	labels, err := i.reg.Labels(context.TODO(), imageRef)
	if err != nil {
		return err
	}

	configsLocation, ok := labels[containertools.ConfigsLocationLabel]
	if !ok {
		return fmt.Errorf("index image %q missing label %q", request.Image, containertools.ConfigsLocationLabel)
	}

	tmpDir, err := ioutil.TempDir("./", imgUnpackTmpDirPrefix)
	defer os.RemoveAll(tmpDir)
	if err != nil {
		return err
	}
	i.reg.Unpack(context.TODO(), imageRef, tmpDir)

	if request.UnpackDir == "" {
		request.UnpackDir = defaultUnpackDir
	}
	configDir := filepath.Join("./", request.UnpackDir)
	if err := os.MkdirAll(configDir, 0777); err != nil {
		return err
	}
	if err := dircopy.Copy(filepath.Join(tmpDir, configsLocation), configDir, dircopy.Options{}); err != nil {
		return err
	}
	i.logger.Infof("Unpacked image %q to directory %q", request.Image, request.UnpackDir)
	return nil
}
