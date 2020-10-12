package main

import (
	"fmt"
	"net"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/operator-framework/operator-registry/pkg/api"
	health "github.com/operator-framework/operator-registry/pkg/api/grpc_health_v1"
	"github.com/operator-framework/operator-registry/pkg/appregistry"
	"github.com/operator-framework/operator-registry/pkg/lib/dns"
	"github.com/operator-framework/operator-registry/pkg/lib/graceful"
	"github.com/operator-framework/operator-registry/pkg/lib/log"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/server"
)

func main() {
	var rootCmd = &cobra.Command{
		Short: "appregistry-server",
		Long:  `appregistry-server downloads operator manifest(s) from remote appregistry, builds a sqlite database containing these downloaded manifest(s) and serves a grpc API to query it`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			if debug, _ := cmd.Flags().GetBool("debug"); debug {
				logrus.SetLevel(logrus.DebugLevel)
			}
			return nil
		},

		RunE: runCmdFunc,
	}

	rootCmd.Flags().Bool("debug", false, "enable debug logging")
	rootCmd.Flags().StringP("kubeconfig", "k", "", "absolute path to kubeconfig file")
	rootCmd.Flags().StringP("download-folder", "f", "downloaded", "directory where downloaded nested operator bundle(s) will be stored to be processed further")
	rootCmd.Flags().StringP("database", "d", "bundles.db", "name of db to output")
	rootCmd.Flags().StringSliceP("registry", "r", []string{}, "pipe delimited operator source - {base url with cnr prefix}|{quay registry namespace}|{secret namespace/secret name}")
	rootCmd.Flags().StringP("packages", "o", "", "comma separated list of package(s) to be downloaded from the specified operator source(s). The requested release can be appended to the package name, delimited with a colone (e.g some-pkg:1.1.0)")
	rootCmd.Flags().StringP("port", "p", "50051", "port number to serve on")
	rootCmd.Flags().StringP("termination-log", "t", "/dev/termination-log", "path to a container termination log file")
	rootCmd.Flags().Bool("strict", false, "fail on registry load errors")

	if err := rootCmd.Flags().MarkHidden("debug"); err != nil {
		logrus.Panic(err.Error())
	}

	if err := rootCmd.Execute(); err != nil {
		logrus.Panic(err.Error())
	}
}

func runCmdFunc(cmd *cobra.Command, args []string) error {
	// Immediately set up termination log
	terminationLogPath, err := cmd.Flags().GetString("termination-log")
	if err != nil {
		return err
	}
	err = log.AddDefaultWriterHooks(terminationLogPath)
	if err != nil {
		logrus.WithError(err).Warn("unable to set termination log path")
	}
	// Ensure there is a default nsswitch config
	if err := dns.EnsureNsswitch(); err != nil {
		logrus.WithError(err).Warn("unable to write default nsswitch config")
	}
	kubeconfig, err := cmd.Flags().GetString("kubeconfig")
	if err != nil {
		return err
	}
	downloadPath, err := cmd.Flags().GetString("download-folder")
	if err != nil {
		return err
	}
	port, err := cmd.Flags().GetString("port")
	if err != nil {
		return err
	}

	sources, err := cmd.Flags().GetStringSlice("registry")
	if err != nil {
		return err
	}

	packages, err := cmd.Flags().GetString("packages")
	if err != nil {
		return err
	}
	dbName, err := cmd.Flags().GetString("database")
	if err != nil {
		return err
	}
	strict, err := cmd.Flags().GetBool("strict")
	if err != nil {
		return err
	}

	logger := logrus.WithFields(logrus.Fields{"type": "appregistry", "port": port})

	loader, err := appregistry.NewLoader(kubeconfig, dbName, downloadPath, logger)
	if err != nil {
		logger.Fatalf("error initializing: %s", err)
	}

	store, err := loader.Load(sources, packages)
	if err != nil {
		err = fmt.Errorf("error loading manifests from appregistry: %s", err)
		if strict {
			logger.WithError(err).Fatal("strict mode enabled")
		}
		logger.WithError(err).Warn("strict mode disabled")
	}
	if store == nil {
		logger.Warn("using empty querier")
		store = registry.NewEmptyQuerier()
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Fatalf("failed to listen: %s", err)
	}
	s := grpc.NewServer()

	api.RegisterRegistryServer(s, server.NewRegistryServer(store))
	health.RegisterHealthServer(s, server.NewHealthServer())
	reflection.Register(s)

	logger.Info("serving registry")
	return graceful.Shutdown(logger, func() error {
		return s.Serve(lis)
	}, func() {
		s.GracefulStop()
	})
}
