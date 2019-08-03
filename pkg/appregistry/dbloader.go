package appregistry

import (
	"os"

	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
	"github.com/sirupsen/logrus"
)

func NewDbLoader(dbName string, logger *logrus.Entry) *dbLoader {
	sqlLoader := sqlite.NewErrorSupressingSQLLoader(dbName)

	return &dbLoader{
		ErrorSupressingSQLLoader: sqlLoader,
		dbName:                   dbName,
		logger:                   logger,
	}
}

type dbLoader struct {
	*sqlite.ErrorSupressingSQLLoader
	dbName string

	logger *logrus.Entry
}

func (l *dbLoader) GetStore() registry.Query {
	return sqlite.NewQuerier(l.dbName)
}

// LoadDataToSQLite uses configMaploader to load the downloaded operator
// manifest(s) into a sqllite database.
func (l *dbLoader) LoadFlattenedToSQLite(manifest *RawOperatorManifestData) error {
	l.logger.Infof("using configmap loader to build sqlite database")

	data := map[string]string{
		"customResourceDefinitions": manifest.CustomResourceDefinitions,
		"clusterServiceVersions":    manifest.ClusterServiceVersions,
		"packages":                  manifest.Packages,
	}

	configMapPopulator := sqlite.NewSQLLoaderForConfigMapData(l.logger, l, data)
	if err := configMapPopulator.Populate(); err != nil {
		return err
	}

	return nil
}

func (l *dbLoader) LoadBundleDirectoryToSQLite(directory string) error {
	if _, err := os.Stat(directory); err != nil {
		l.logger.Errorf("stat failed on target directory[%s] - %v", directory, err)
		return err
	}

	loader := sqlite.NewSQLLoaderForDirectory(l, directory)
	if err := loader.Populate(); err != nil {
		return err
	}

	return nil
}
