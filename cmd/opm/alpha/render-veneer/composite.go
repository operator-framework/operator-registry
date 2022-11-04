package veneer

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/operator-framework/operator-registry/alpha/veneer/composite"
	"github.com/operator-framework/operator-registry/cmd/opm/internal/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const def_filename = "composite.yaml"

func newCompositeCmd() *cobra.Command {
	var (
		configFile      string
		packagesBaseDir string
		cacheDir        string
	)
	cmd := &cobra.Command{
		Use:   "composite [-c CONFIGFILE|--cache-dir CACHEDIR]",
		Short: "Unpack and build a composite veneer file",
		Long: `Unpack and build a composite veneer file representing coordinated actions across a 
cohort of contributions, according to the composite schema <foo-loc>.
If the config file is not provided, it will be defaulted to a file 
named "composite.yaml" in the current directory.
If the cache dir is not provided, then the program will purge the cache upon completion.  
To persist/reuse the cache, provide a cache-dir location.`,
		Run: func(cmd *cobra.Command, args []string) {
			bc, err := composite.LoadBuildConfigFile(configFile)
			if err != nil {
				log.Fatalf("load build config file %q: %v", configFile, err)
			}

			configFileDir := filepath.Dir(configFile)
			pcs, err := composite.LoadPackageConfigs(filepath.Join(configFileDir, bc.PackagesBaseDir))
			if err != nil {
				log.Fatalf("load package configs from base directory %q: %v", packagesBaseDir, err)
			}

			destroyCache := cacheDir == ""
			if destroyCache {
				cacheDir, err = os.MkdirTemp("", "composite-image-cache-")
				if err != nil {
					log.Fatalf("create temporary image cache directory: %v", err)
				}
			}

			logrus.SetOutput(io.Discard)

			reg, err := util.CreateCLIRegistry(cmd)
			if err != nil {
				log.Fatalf("creating containerd registry: %v", err)
			}

			if destroyCache {
				defer reg.Destroy()
			}

			builders, err := composite.CreateBuilders(*bc, pcs, reg)
			if err != nil {
				log.Fatalf("create builders for each catalog and package: %v", err)
			}

			for _, b := range builders {
				if err := b.Build(cmd.Context()); err != nil {
					log.Fatal(err)
				}
			}

			fmt.Println("\n\nSUMMARY:")
			for _, b := range builders {
				fmt.Printf("  Built catalog image %q with packages:\n", b.BuildConfig.Destination.OutputImage)
				for _, pkg := range b.Packages() {
					fmt.Printf("    %s\n", pkg)
				}
				fmt.Printf("\n")
			}
		},
	}
	cmd.Flags().StringVarP(&configFile, "config", "c", def_filename, "Path to composite config file.")
	cmd.Flags().StringVar(&cacheDir, "cache-dir", "", "Path of persistent image cache directory.")

	// ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	// defer cancel()
	// if err := cmd.ExecuteContext(ctx); err != nil {
	// 	log.Fatal(err)
	// }
	return cmd
}
