package execregistry

import (
	"context"
	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/sirupsen/logrus"

	"github.com/operator-framework/operator-registry/pkg/image"
)

// Registry enables manipulation of images via exec podman/docker commands.
type Registry struct {
	log     *logrus.Entry
	cmd     containertools.CommandRunner
	skipTLS bool
}

// Adapt the cmd interface to the registry interface
var _ image.Registry = &Registry{}

func NewRegistry(tool containertools.ContainerTool, logger *logrus.Entry, skipTLS bool) (registry *Registry, err error) {
	return &Registry{
		log:     logger,
		cmd:     containertools.NewCommandRunner(tool, logger),
		skipTLS: skipTLS,
	}, nil
}

// Pull fetches and stores an image by reference.
func (r *Registry) Pull(ctx context.Context, ref image.Reference) error {
	return r.cmd.Pull(ref.String(), r.skipTLS)
}

// Unpack writes the unpackaged content of an image to a directory.
// If the referenced image does not exist in the registry, an error is returned.
func (r *Registry) Unpack(ctx context.Context, ref image.Reference, dir string) error {
	return containertools.ImageLayerReader{
		Cmd:    r.cmd,
		Logger: r.log,
	}.GetImageData(ref.String(), dir, containertools.SkipTLS(r.skipTLS))
}

// Labels gets the labels for an image reference.
func (r *Registry) Labels(ctx context.Context, ref image.Reference) (map[string]string, error) {
	return containertools.ImageLabelReader{
		Cmd:    r.cmd,
		Logger: r.log,
	}.GetLabelsFromImage(ref.String(), r.skipTLS)
}

// Destroy is no-op for exec tools
func (r *Registry) Destroy() error {
	return nil
}
