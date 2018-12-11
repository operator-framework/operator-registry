package main

import (
	"context"
	"net"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/operator-framework/operator-registry/pkg/lib/log"
	"github.com/operator-framework/operator-registry/pkg/server"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

func main() {
	var rootCmd = &cobra.Command{
		Short: "registry-server",
		Long:  `registry loads a sqlite database containing operator manifests and serves a grpc API to query it`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			if debug, _ := cmd.Flags().GetBool("debug"); debug {
				logrus.SetLevel(logrus.DebugLevel)
			}
			return nil
		},

		RunE: runCmdFunc,
	}

	rootCmd.Flags().Bool("debug", false, "enable debug logging")
	rootCmd.Flags().StringP("database", "d", "bundles.db", "relative path to sqlite db")
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
	terminationLogFile, err := os.OpenFile(terminationLogPath, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	log.AddHooks(
		&log.WriterHook{
			Writer: terminationLogFile,
			LogLevels: []logrus.Level{
				logrus.PanicLevel,
				logrus.FatalLevel,
			},
		},
		&log.WriterHook{
			Writer: os.Stderr,
			LogLevels: []logrus.Level{
				logrus.ErrorLevel,
				logrus.WarnLevel,
			},
		},
		&log.WriterHook{
			Writer: os.Stdout,
			LogLevels: []logrus.Level{
				logrus.InfoLevel,
				logrus.DebugLevel,
			},
		})

	dbName, err := cmd.Flags().GetString("database")
	if err != nil {
		return err
	}

	port, err := cmd.Flags().GetString("port")
	if err != nil {
		return err
	}

	logger := logrus.WithFields(logrus.Fields{"database": dbName, "port": port})

	store, err := sqlite.NewSQLLiteQuerier(dbName)
	if err != nil {
		logger.Fatalf("failed to load db: %v", err)
	}

	// sanity check that the db is available
	tables, err := store.ListTables(context.TODO())
	if err != nil {
		logger.Fatalf("couldn't list tables in db, incorrect config: %v", err)
	}
	if len(tables) == 0 {
		logger.Fatal("no tables found in db")
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()

	api.RegisterRegistryServer(s, server.NewRegistryServer(store))
	api.RegisterHealthServer(s, server.NewHealthServer())
	reflection.Register(s)
	logger.Info("serving registry")
	if err := s.Serve(lis); err != nil {
		logger.Fatalf("failed to serve: %v", err)
	}

	return nil
}
