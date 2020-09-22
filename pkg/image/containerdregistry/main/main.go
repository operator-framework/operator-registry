package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	_ "crypto/sha256"
	_ "crypto/sha512"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/namespaces"
	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/registry/handlers"
	"github.com/docker/distribution/registry/listener"
	"github.com/docker/distribution/uuid"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/lib/certs"

	_ "github.com/docker/distribution/registry/storage/driver/inmemory"
	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry"
	"github.com/sirupsen/logrus"
)

func main() {
	// var quit = make(chan os.Signal, 1)
	// signal.Notify(quit, syscall.SIGTERM)
	// doneChan, err := StartRegistry(context.TODO())
	// if err != nil {
	// 	fmt.Errorf("error starting registry: %v", err)
	// }

	// time.Sleep(3 * time.Second)

	logger := logrus.WithFields(logrus.Fields{})
	rootCAs, err := certs.RootCAs("")
	if err != nil {
		fmt.Printf("failed to get RootCAs: %v\n", err)
		return
	}
	reg, rerr := containerdregistry.NewRegistry(containerdregistry.SkipTLS(true), containerdregistry.WithLog(logger), containerdregistry.WithRootCAs(rootCAs), containerdregistry.WithResolverConfigDir(filepath.Join(os.Getenv("HOME"), ".docker")))
	if rerr != nil {
		fmt.Println(rerr)
		return
	}
	defer func() {
		if err := reg.Destroy(); err != nil {
			logger.WithError(err).Warn("error destroying the local cache")
		}
	}()
	ctx := namespaces.WithNamespace(context.TODO(), namespaces.Default)

	// remoteTag := image.SimpleReference("localhost:5000/test-tag2:latest")
	//	remoteTag := image.SimpleReference("quay.io/ankitathomas/containerbuild:buildah")
	remoteTag := image.SimpleReference("dockerhub.io/athoma24/ubuntu:v1")
	remoteTag2 := image.SimpleReference("dockerhub.io/athoma24/ubuntu:v2")
	err = reg.Pull(ctx, remoteTag)
	if err != nil {
		fmt.Printf("failed to create image: %v\n", err)
		return
	}

	img, _ := reg.Images().Get(ctx, remoteTag.String())
	img2 := images.Image{
		Name:   remoteTag2.String(),
		Target: img.Target,
	}
	if _, err = reg.Images().Create(ctx, img2); err != nil {
		if errdefs.IsAlreadyExists(err) {
			_, err = reg.Images().Update(ctx, img2)
		}
	}

	// err = reg.NewImage(ctx, remoteTag, containerdregistry.OmitTimestamp()) //, containerdregistry.WithBaseImage(remoteTag))
	// if err != nil {
	// 	fmt.Printf("failed to create image: %v\n", err)
	// 	return
	// }

	// err = reg.Pack(ctx, remoteTag, "dir", containerdregistry.OmitTimestamp())
	// if err != nil {
	// 	fmt.Printf("error adding layer: %v\n", err)
	// 	return
	// }

	// err = reg.Unpack(ctx, remoteTag, "bundledir")
	// if err != nil {
	// 	fmt.Printf("error unpacking created image: %v\n", err)
	// 	return
	// }

	// err = reg.Export(ctx, remoteTag, "ocibundledir.2")
	// if err != nil {
	// 	fmt.Printf("error exporting created image: %v\n", err)
	// 	return
	// }

	err = reg.Push(ctx, remoteTag2)
	if err != nil {
		fmt.Printf("error pushing created image: %v\n", err)
		return
	}

	reg.Content().Delete(ctx, img.Target.Digest)
	reg.Images().Delete(ctx, remoteTag2.String())

	err = reg.Pull(ctx, remoteTag2)
	if err != nil {
		fmt.Printf("failed to create image: %v\n", err)
		return
	}

	fmt.Println("ok!")
	// err = reg.NewImage(ctx, remoteTag, containerdregistry.OmitTimestamp(), containerdregistry.WithBaseImage(remoteTag))
	// if err != nil {
	// 	fmt.Printf("failed to create image: %v\n", err)
	// 	return
	// }

	// err = reg.Export(ctx, localTag, "ocibundledir-2")
	// if err != nil {
	// 	fmt.Printf("error exporting created image: %v\n", err)
	// 	return
	// }

	// err = reg.Unpack(ctx, remoteTag, "bundledir-2")
	// if err != nil {
	// 	fmt.Printf("error unpacking created image: %v\n", err)
	// 	return
	// }

	// f, _ := os.Create(path.Join("bundledir-2", "dir", "file2"))
	// f.Close()

	// err = reg.Pack(ctx, remoteTag, "bundledir-2", containerdregistry.OmitTimestamp(), containerdregistry.SquashLayers())
	// if err != nil {
	// 	fmt.Printf("error adding layer: %v\n", err)
	// 	return
	// }

	// err = reg.Unpack(ctx, remoteTag, "bundledir-3")
	// if err != nil {
	// 	fmt.Printf("error unpacking created image: %v\n", err)
	// 	return
	// }

	// for {
	// 	select {
	// 	case <-quit:
	// 		doneChan <- struct{}{}
	// 	}
	// }
}

// StartRegistry starts a local docker registry on port 5000
func StartRegistry(parent context.Context) (chan struct{}, error) {
	var err error
	var doneChan = make(chan struct{}, 1)
	config := &configuration.Configuration{
		Storage: map[string]configuration.Parameters{
			"inmemory": map[string]interface{}{},
		},
	}
	config.HTTP.Addr = ":5000"
	config.HTTP.DrainTimeout = time.Duration(10) * time.Second
	logger := logrus.StandardLogger()
	// inject a logger into the uuid library. warns us if there is a problem
	// with uuid generation under low entropy.
	uuid.Loggerf = logger.Warnf

	ctx := context.Background()
	app := handlers.NewApp(ctx, config)
	handler := alive(app)
	handler = panicHandler(handler)

	server := &http.Server{
		Handler: handler,
	}

	ln, err := listener.NewListener(config.HTTP.Net, config.HTTP.Addr)
	if err != nil {
		return nil, err
	}

	logger.Infof("listening on %v", ln.Addr())
	serveErr := make(chan error)

	go func() {
		serveErr <- server.Serve(ln)
	}()

	go func() {
		var done bool
		for !done {
			select {
			case <-parent.Done():
				done = true
				if parent.Err() != nil {
					logger.Errorf("Error running server: %v", err)
				}
				break
			case <-doneChan:
				done = true
				break
			}
		}
		var err error
		logger.Infof("Attemptimg to stop server, draining connections for %s", config.HTTP.DrainTimeout.String())
		c, cancel := context.WithTimeout(context.Background(), config.HTTP.DrainTimeout)
		defer cancel()
		serveErr <- server.Shutdown(c)
		for {
			select {
			case <-c.Done():
				if c.Err() != nil {
					logger.Error(err)
				}
			case err = <-serveErr:
				if err != nil {
					logger.Error(err)
				}
			case <-time.After(config.HTTP.DrainTimeout):
				logger.Errorf("Timed out waiting for server to stop")
			}
		}
	}()
	return doneChan, nil
}

func alive(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Cache-Control", "no-cache")
			w.WriteHeader(http.StatusOK)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

func panicHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Panic(fmt.Sprintf("%v", err))
			}
		}()
		handler.ServeHTTP(w, r)
	})
}
