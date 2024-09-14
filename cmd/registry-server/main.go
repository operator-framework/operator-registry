package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	health "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/operator-framework/operator-registry/pkg/lib/dns"
	"github.com/operator-framework/operator-registry/pkg/lib/log"
	"github.com/operator-framework/operator-registry/pkg/lib/tmp"
	"github.com/operator-framework/operator-registry/pkg/server"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

var rootCmd = &cobra.Command{
	Short: "registry-server",
	Long: `registry loads a sqlite database containing operator manifests and serves a grpc API to query it

` + sqlite.DeprecationMessage,
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		sqlite.LogSqliteDeprecation()
	},
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if debug, _ := cmd.Flags().GetBool("debug"); debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	},

	RunE: runCmdFunc,
}

func init() {
	rootCmd.Flags().Bool("debug", false, "enable debug logging")
	rootCmd.Flags().StringP("database", "d", "bundles.db", "relative path to sqlite db")
	rootCmd.Flags().StringP("port", "p", "50051", "port number to serve on")
	rootCmd.Flags().StringP("termination-log", "t", "/dev/termination-log", "path to a container termination log file")
	rootCmd.Flags().Bool("skip-migrate", false, "do  not attempt to migrate to the latest db revision when starting")
	if err := rootCmd.Flags().MarkHidden("debug"); err != nil {
		logrus.Panic(err.Error())
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		logrus.Panic(err.Error())
	}
}

func runCmdFunc(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

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
	dbName, err := cmd.Flags().GetString("database")
	if err != nil {
		return err
	}

	port, err := cmd.Flags().GetString("port")
	if err != nil {
		return err
	}

	logger := logrus.WithFields(logrus.Fields{"database": dbName, "port": port})

	// make a writable copy of the db for migrations
	tmpdb, err := tmp.CopyTmpDB(dbName)
	if err != nil {
		return err
	}
	defer os.Remove(tmpdb)

	db, err := sqlite.Open(tmpdb)
	if err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `PRAGMA soft_heap_limit=1`); err != nil {
		logger.WithError(err).Warnf("error setting soft heap limit for sqlite")
	}

	// migrate to the latest version
	shouldSkipMigrate, err := cmd.Flags().GetBool("skip-migrate")
	if err != nil {
		return err
	}
	if err := migrate(ctx, shouldSkipMigrate, db); err != nil {
		logger.WithError(err).Warnf("couldn't migrate db")
	}

	store := sqlite.NewSQLLiteQuerierFromDb(db, sqlite.OmitManifests(true))

	// sanity check that the db is available
	tables, err := store.ListTables(ctx)
	if err != nil {
		logger.WithError(err).Warnf("couldn't list tables in db")
	}
	if len(tables) == 0 {
		logger.Warn("no tables found in db")
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Fatalf("failed to listen: %s", err)
	}
	s := grpc.NewServer()

	api.RegisterRegistryServer(s, server.NewRegistryServer(store))
	health.RegisterHealthServer(s, server.NewHealthServer())
	reflection.Register(s)

	go func() {
		<-ctx.Done()
		logger.Info("shutting down server")
		s.GracefulStop()
	}()

	logger.Info("serving registry")
	return s.Serve(lis)
}

func migrate(ctx context.Context, shouldSkipMigrate bool, db *sql.DB) error {
	if shouldSkipMigrate {
		return nil
	}

	migrator, err := sqlite.NewSQLLiteMigrator(db)
	if err != nil {
		return err
	}
	if migrator == nil {
		return fmt.Errorf("failed to load migrator")
	}

	return migrator.Migrate(ctx)
}
