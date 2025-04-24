package image

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry/handlers"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem" // Driver for persisting docker image data to the filesystem.
	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory"   // Driver for keeping docker image data in memory.
)

// RunDockerRegistry runs a docker registry on an available port and returns its host string if successful, otherwise it returns an error.
// If the rootDir argument isn't empty, the registry is configured to use this as the root directory for persisting image data to the filesystem.
// If the rootDir argument is empty, the registry is configured to keep image data in memory.
func RunDockerRegistry(ctx context.Context, rootDir string, middlewares ...func(http.Handler) http.Handler) *httptest.Server {
	config := &configuration.Configuration{}
	config.Log.Level = "error"

	if rootDir != "" {
		config.Storage = map[string]configuration.Parameters{"filesystem": map[string]interface{}{
			"rootdirectory": rootDir,
		}}
	} else {
		config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	}

	var dockerRegistryApp http.Handler = handlers.NewApp(ctx, config)
	for _, m := range middlewares {
		dockerRegistryApp = m(dockerRegistryApp)
	}
	server := httptest.NewTLSServer(dockerRegistryApp)
	return server
}
