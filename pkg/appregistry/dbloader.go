package appregistry

import (
	"context"
	"fmt"

	"github.com/operator-framework/operator-registry/pkg/sqlite"
	"github.com/sirupsen/logrus"
)

type dbLoader struct {
	logger *logrus.Entry
}

// LoadToSQLite uses configMaploader to load the downloaded operator manifest(s)
// into a sqllite database.
func (l *dbLoader) LoadToSQLite(dbName string, manifest *RawOperatorManifestData) (store *sqlite.SQLQuerier, err error) {
	l.logger.Infof("using configmap loader to build sqlite database")

	data := map[string]string{
		"customResourceDefinitions": manifest.CustomResourceDefinitions,
		"clusterServiceVersions":    manifest.ClusterServiceVersions,
		"packages":                  manifest.Packages,
	}

	sqlLoader, err := sqlite.NewSQLLiteLoader(dbName)
	if err != nil {
		return
	}

	configMapPopulator := sqlite.NewSQLLoaderForConfigMapData(l.logger, sqlLoader, data)
	if err = configMapPopulator.Populate(); err != nil {
		return
	}

	s, err := sqlite.NewSQLLiteQuerier(dbName)
	if err != nil {
		err = fmt.Errorf("failed to load db: %v", err)
		return
	}

	// sanity check that the db is available.
	tables, err := s.ListTables(context.TODO())
	if err != nil {
		err = fmt.Errorf("couldn't list tables in db, incorrect config: %v", err)
		return
	}

	if len(tables) == 0 {
		err = fmt.Errorf("no tables found in db")
		return
	}

	store = s
	return
}
