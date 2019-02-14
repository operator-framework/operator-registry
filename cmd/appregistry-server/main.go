package main

import (
	"net"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/operator-framework/operator-registry/pkg/api"
	health "github.com/operator-framework/operator-registry/pkg/api/grpc_health_v1"
	"github.com/operator-framework/operator-registry/pkg/appregistry"
	"github.com/operator-framework/operator-registry/pkg/lib/log"
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
	rootCmd.Flags().StringP("database", "d", "bundles.db", "name of db to output")
	rootCmd.Flags().StringP("sources", "s", "", "comma separated list of OperatorSource object(s) {namespace}/{name}")
	rootCmd.Flags().StringP("packages", "o", "", "comma separated list of package(s) to be downloaded from the specified operator source(s)")
	rootCmd.Flags().StringP("port", "p", "50051", "port number to serve on")
	rootCmd.Flags().StringP("termination-log", "t", "/dev/termination-log", "path to a container termination log file")
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
		return err
	}
	kubeconfig, err := cmd.Flags().GetString("kubeconfig")
	if err != nil {
		return err
	}
	port, err := cmd.Flags().GetString("port")
	if err != nil {
		return err
	}
	sources, err := cmd.Flags().GetString("sources")
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

	logger := logrus.WithFields(logrus.Fields{"type": "appregistry", "port": port})

	loader, err := appregistry.NewLoader(kubeconfig, logger)
	if err != nil {
		logger.Fatalf("error initializing - %v", err)
	}

	store, err := loader.Load(dbName, sources, packages)
	if err != nil {
		logger.Fatalf("error loading manifest from remote registry - %v", err)
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()

	api.RegisterRegistryServer(s, server.NewRegistryServer(store))
	health.RegisterHealthServer(s, server.NewHealthServer())
	reflection.Register(s)

	logger.Info("serving registry")
	if err := s.Serve(lis); err != nil {
		logger.Fatalf("failed to serve: %v", err)
	}

	return nil
}
