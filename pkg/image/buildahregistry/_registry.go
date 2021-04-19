// +build ignore
package buildahregistry

import (
	"context"
	"path"
	"time"

	"github.com/containers/buildah"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"github.com/sirupsen/logrus"

	"github.com/operator-framework/operator-registry/pkg/image"
)

// Registry enables manipulation of images via containerd modules.
// TODO: Finish buildah Registry implementation.
type Registry struct {
	storage.Store

	CacheDir string
	SkipTLS  bool

	log *logrus.Entry
}

var _ image.Registry = &Registry{}

// Pull fetches and stores an image by reference.
func (r *Registry) Pull(ctx context.Context, ref image.Reference) error {
	img, err := buildah.Pull(ctx, ref.String(), buildah.PullOptions{
		SignaturePolicyPath: path.Join(r.CacheDir, "policy.json"),
		ReportWriter:        r.log.Writer(),
		Store:               r.Store,
		SystemContext: &types.SystemContext{
			// TODO: auth stuff goes here too
			// TODO: if we're okay with buildah's cobra args, there's a function to build this from the standard args
			SignaturePolicyPath:         path.Join(r.CacheDir, "policy.json"),
			OCIInsecureSkipTLSVerify:    r.SkipTLS,
			DockerInsecureSkipTLSVerify: types.NewOptionalBool(r.SkipTLS),
		},
		BlobDirectory:    r.CacheDir,
		AllTags:          false,
		RemoveSignatures: false,
		MaxRetries:       5,
		RetryDelay:       1 * time.Second,
	})

	r.log.Info(img)
	return err
}

// Unpack writes the unpackaged content of an image to a directory.
// If the referenced image does not exist in the registry, an error is returned.
func (r *Registry) Unpack(ctx context.Context, ref image.Reference, dir string) error {
	img, err := r.Image(ref.String())
	if err != nil {
		return err
	}

	r.log.Infof("img: %v", img)

	return nil
}
