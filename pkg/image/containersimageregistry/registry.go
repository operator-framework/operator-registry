package containersimageregistry

import (
	"archive/tar"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/archive"
	dockerconfig "github.com/docker/cli/cli/config"
	"go.podman.io/common/pkg/auth"
	"go.podman.io/image/v5/copy"
	"go.podman.io/image/v5/docker"
	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/image/v5/image"
	"go.podman.io/image/v5/oci/layout"
	"go.podman.io/image/v5/pkg/compression"
	"go.podman.io/image/v5/pkg/docker/config"
	"go.podman.io/image/v5/signature"
	"go.podman.io/image/v5/types"
	"oras.land/oras-go/v2/content/oci"

	orimage "github.com/operator-framework/operator-registry/pkg/image"
)

var _ orimage.Registry = (*Registry)(nil)

type Registry struct {
	sourceCtx *types.SystemContext
	cache     *cacheConfig
}

var DefaultSystemContext = &types.SystemContext{OSChoice: "linux"}

func New(sourceCtx *types.SystemContext, opts ...Option) (orimage.Registry, error) {
	if sourceCtx == nil {
		sourceCtx = &types.SystemContext{}
	}
	reg := &Registry{
		sourceCtx: sourceCtx,
	}

	for _, opt := range opts {
		if err := opt(reg); err != nil {
			return nil, err
		}
	}

	if reg.cache == nil {
		var err error
		reg.cache, err = getDefaultImageCache()
		if err != nil {
			return nil, err
		}
	}

	return reg, nil
}

func NewDefault() (orimage.Registry, error) {
	return New(DefaultSystemContext)
}

type cacheConfig struct {
	baseDir  string
	preserve bool
}

func (c *cacheConfig) ociLayoutDir() string {
	return filepath.Join(c.baseDir, "oci-layout")
}
func (c *cacheConfig) blobInfoCacheDir() string {
	return filepath.Join(c.baseDir, "blob-info-cache")
}

func (c *cacheConfig) getSystemContext() *types.SystemContext {
	return &types.SystemContext{
		BlobInfoCacheDir: c.blobInfoCacheDir(),
	}
}

type Option func(*Registry) error

func getDefaultImageCache() (*cacheConfig, error) {
	if dir := os.Getenv("OLM_CACHE_DIR"); dir != "" {
		return newCacheConfig(filepath.Join(dir, "images"), true), nil
	}
	return getTemporaryImageCache()
}

func getTemporaryImageCache() (*cacheConfig, error) {
	tmpDir, err := os.MkdirTemp("", "opm-containers-image-cache-")
	if err != nil {
		return nil, err
	}
	return newCacheConfig(tmpDir, false), nil
}

func newCacheConfig(dir string, preserve bool) *cacheConfig {
	return &cacheConfig{
		baseDir:  dir,
		preserve: preserve,
	}
}

func WithTemporaryImageCache() Option {
	return func(r *Registry) error {
		var err error
		r.cache, err = getTemporaryImageCache()
		if err != nil {
			return err
		}
		return nil
	}
}

func WithInsecureSkipTLSVerify(insecureSkipTLSVerify bool) Option {
	return func(r *Registry) error {
		r.sourceCtx.DockerDaemonInsecureSkipTLSVerify = insecureSkipTLSVerify
		r.sourceCtx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(insecureSkipTLSVerify)
		r.sourceCtx.OCIInsecureSkipTLSVerify = insecureSkipTLSVerify
		return nil
	}
}

func (r *Registry) Pull(ctx context.Context, ref orimage.Reference) error {
	namedRef, err := reference.ParseNamed(ref.String())
	if err != nil {
		return err
	}
	dockerRef, err := docker.NewReference(namedRef)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(r.cache.ociLayoutDir(), 0700); err != nil {
		return err
	}
	ociLayoutRef, err := layout.NewReference(r.cache.ociLayoutDir(), ref.String())
	if err != nil {
		return err
	}

	policy, err := signature.DefaultPolicy(r.sourceCtx)
	if err != nil {
		return err
	}
	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return err
	}

	sourceCtx := r.sourceCtx
	authFile := getAuthFile(r.sourceCtx, namedRef.String())
	if authFile != "" {
		sourceCtx.AuthFilePath = authFile
	}

	if _, err := copy.Image(ctx, policyContext, ociLayoutRef, dockerRef, &copy.Options{
		SourceCtx:                             sourceCtx,
		DestinationCtx:                        r.cache.getSystemContext(),
		OptimizeDestinationImageAlreadyExists: true,

		// We use the OCI layout as a temporary storage and
		// pushing signatures for OCI images is not supported
		// so we remove the source signatures when copying.
		// Signature validation will still be performed
		// accordingly to a provided policy context.
		RemoveSignatures: true,
	}); err != nil {
		return err
	}
	return nil
}

func (r *Registry) Unpack(ctx context.Context, ref orimage.Reference, unpackDir string) error {
	ociLayoutRef, err := layout.NewReference(r.cache.ociLayoutDir(), ref.String())
	if err != nil {
		return fmt.Errorf("could not create oci layout reference: %w", err)
	}

	ociLayoutCtx := r.cache.getSystemContext()
	imageSource, err := ociLayoutRef.NewImageSource(ctx, ociLayoutCtx)
	if err != nil {
		return fmt.Errorf("failed to create oci image source: %v", err)
	}
	defer imageSource.Close()

	img, err := image.FromSource(ctx, ociLayoutCtx, imageSource)
	if err != nil {
		return fmt.Errorf("could not get image from oci image source: %v", err)
	}

	if err := os.MkdirAll(unpackDir, 0700); err != nil {
		return err
	}

	for _, info := range img.LayerInfos() {
		if err := func() error {
			layer, _, err := imageSource.GetBlob(ctx, info, nil)
			if err != nil {
				return fmt.Errorf("failed to get blob: %v", err)
			}
			defer layer.Close()

			decompressed, _, err := compression.AutoDecompress(layer)
			if err != nil {
				return fmt.Errorf("failed to decompress layer: %v", err)
			}

			if _, err := archive.Apply(ctx, unpackDir, decompressed, archive.WithFilter(func(th *tar.Header) (bool, error) {
				th.PAXRecords = nil
				th.Xattrs = nil //nolint:staticcheck
				th.Uid = os.Getuid()
				th.Gid = os.Getgid()
				th.Mode = 0600
				if th.FileInfo().IsDir() {
					th.Mode = 0700
				}
				return true, nil
			})); err != nil {
				return fmt.Errorf("failed to apply layer: %v", err)
			}
			return nil
		}(); err != nil {
			return err
		}
	}
	return nil
}

func (r *Registry) Labels(ctx context.Context, ref orimage.Reference) (map[string]string, error) {
	ociLayoutRef, err := layout.NewReference(r.cache.ociLayoutDir(), ref.String())
	if err != nil {
		return nil, fmt.Errorf("could not create oci layout reference: %w", err)
	}

	ociLayoutCtx := r.cache.getSystemContext()
	img, err := ociLayoutRef.NewImage(ctx, ociLayoutCtx)
	if err != nil {
		return nil, fmt.Errorf("could not load image from oci image reference: %v", err)
	}
	imgConfig, err := img.OCIConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get oci config from image: %v", err)
	}
	return imgConfig.Config.Labels, nil
}

func (r *Registry) Destroy() error {
	if !r.cache.preserve {
		return os.RemoveAll(r.cache.baseDir)
	}

	store, err := oci.NewWithContext(context.TODO(), r.cache.ociLayoutDir())
	if err != nil {
		return fmt.Errorf("open cache for garbage collection: %v", err)
	}
	if err := store.GC(context.TODO()); err != nil {
		return fmt.Errorf("garbage collection failed: %v", err)
	}
	return nil
}

// This is a slight variation on the auth.GetDefaultAuthFile function provided by containers/image.
// The reason for this variation is so that this image registry implementation can be used as a drop-in
// replacement for our existing containerd-based image registry client, and remain compatible with current
// behavior.
func getAuthFile(sourceCtx *types.SystemContext, ref string) string {
	// By default, we will use the docker config file in the standard docker config directory.
	// However, if REGISTRY_AUTH_FILE or DOCKER_CONFIG environment variables are set, we will
	// use those (in that order) instead to derive the auth config file.
	authFile := filepath.Join(dockerconfig.Dir(), dockerconfig.ConfigFileName)
	if defaultAuthFile := auth.GetDefaultAuthFile(); defaultAuthFile != "" {
		authFile = defaultAuthFile
	}

	// In order to maintain backward-compatibility with the original credential getter from
	// the containerd registry implementation, we will first try to get the credentials from
	// the auth config file we derived above, if it exists. If we find a matching credential
	// in this file, we'll use this file.
	if stat, statErr := os.Stat(authFile); statErr == nil && stat.Mode().IsRegular() {
		if _, err := config.GetCredentials(&types.SystemContext{AuthFilePath: authFile}, ref); err == nil {
			return authFile
		}
	}
	// If the auth file was unset, doesn't exist, or if we couldn't find credentials in it,
	// we'll use system defaults from containers/image (podman/skopeo) to lookup the credentials.
	if sourceCtx != nil && sourceCtx.AuthFilePath != "" {
		return sourceCtx.AuthFilePath
	}
	return ""
}
