package image

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/registry"
	_ "github.com/docker/distribution/registry/storage/driver/filesystem" // Driver for persisting docker image data to the filesystem.
	_ "github.com/docker/distribution/registry/storage/driver/inmemory"   // Driver for keeping docker image data in memory.
	"github.com/phayes/freeport"
)

// RunDockerRegistry runs a docker registry on an available port and returns its host string if successful, otherwise it returns an error.
// If the rootDir argument isn't empty, the registry is configured to use this as the root directory for persisting image data to the filesystem.
// If the rootDir argument is empty, the registry is configured to keep image data in memory.
func RunDockerRegistry(ctx context.Context, rootDir string) (string, error) {
	dockerPort, err := freeport.GetFreePort()
	if err != nil {
		return "", err
	}

	config := &configuration.Configuration{}
	config.HTTP.Addr = fmt.Sprintf(":%d", dockerPort)
	if rootDir != "" {
		config.Storage = map[string]configuration.Parameters{"filesystem": map[string]interface{}{
			"rootdirectory": rootDir,
		}}
	} else {
		config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	}
	config.HTTP.DrainTimeout = 2 * time.Second

	dockerRegistry, err := registry.NewRegistry(ctx, config)
	if err != nil {
		return "", err
	}

	go func() {
		if err := dockerRegistry.ListenAndServe(); err != nil {
			panic(fmt.Errorf("docker registry stopped listening: %v", err))
		}
	}()

	// Return the registry host string
	return fmt.Sprintf("localhost:%d", dockerPort), nil
}
