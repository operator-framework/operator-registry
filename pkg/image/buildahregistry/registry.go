package buildahregistry

import (
	"context"
	"github.com/containers/storage"
	"path"

	"github.com/containers/buildah"
	"github.com/containers/image/v5/types"
	"github.com/sirupsen/logrus"
)

type Registry struct {
	storage.Store

	CacheDir string

	log      *logrus.Entry

	close func() error
}

// Pull fetches and stores an image by reference.
func (r *Registry) Pull(ctx context.Context, ref string) error {
	img, err := buildah.Pull(ctx, ref, buildah.PullOptions{
		SignaturePolicyPath: path.Join(r.CacheDir, "policy.json"),
		ReportWriter:        r.log.Writer(),
		Store:               r.Store,
		SystemContext:       &types.SystemContext{
			// TODO: auth stuff goes here too
			// TODO: if we're okay with buildah's cobra args, there's a function to build this from the standard args
			SignaturePolicyPath:     path.Join(r.CacheDir, "policy.json"),
			OCIInsecureSkipTLSVerify:          true,
			DockerInsecureSkipTLSVerify:       types.OptionalBoolTrue,

		},
		BlobDirectory:       r.CacheDir,
		AllTags:             false,
		RemoveSignatures:    false,
		MaxRetries:          0,
		RetryDelay:          0,
	})

	r.log.Info(img)
	return err
}

// Unpack writes the unpackaged content of an image to a directory.
// If the referenced image does not exist in the registry, an error is returned.
func (r *Registry) Unpack(ctx context.Context, ref, dir string) error {
	return nil
}

func (r *Registry) Close() error {
	return r.close()
}

