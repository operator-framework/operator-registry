package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/operator-framework/operator-registry/cmd/opm/root"
	registrylib "github.com/operator-framework/operator-registry/pkg/registry"
)

func main() {
	showAlphaHelp := os.Getenv("HELP_ALPHA") == "true"
	cmd := root.NewCmd(showAlphaHelp)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := cmd.ExecuteContext(ctx); err != nil {
		var agg utilerrors.Aggregate
		if !errors.As(err, &agg) {
			os.Exit(1)
		}
		for _, e := range agg.Errors() {
			var bundleAlreadyAddedErr registrylib.BundleImageAlreadyAddedErr
			if errors.As(e, &bundleAlreadyAddedErr) {
				os.Exit(2)
			}
			var packageVersionAlreadyAddedErr registrylib.PackageVersionAlreadyAddedErr
			if errors.As(e, &packageVersionAlreadyAddedErr) {
				os.Exit(3)
			}
		}
		os.Exit(1)
	}
}
