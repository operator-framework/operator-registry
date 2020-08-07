package registry

import (
	"context"
	"crypto/x509"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/sirupsen/logrus"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry"
	"github.com/operator-framework/operator-registry/pkg/image/execregistry"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

type RegistryUpdater struct {
	Logger *logrus.Entry
}

type AddToRegistryRequest struct {
	Permissive    bool
	SkipTLS       bool
	CaFile        string
	InputDatabase string
	Bundles       []string
	Mode          registry.Mode
	ContainerTool containertools.ContainerTool
}

func (r RegistryUpdater) AddToRegistry(request AddToRegistryRequest) error {
	db, err := sql.Open("sqlite3", request.InputDatabase)
	if err != nil {
		return err
	}
	defer db.Close()

	dbLoader, err := sqlite.NewSQLLiteLoader(db)
	if err != nil {
		return err
	}
	if err := dbLoader.Migrate(context.TODO()); err != nil {
		return err
	}

	graphLoader, err := sqlite.NewSQLGraphLoaderFromDB(db)
	if err != nil {
		return err
	}
	dbQuerier := sqlite.NewSQLLiteQuerierFromDb(db)

	// add custom ca certs to resolver
	rootCAs, err := x509.SystemCertPool()
	if err != nil || rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	if len(request.CaFile) > 0 {
		certs, err := ioutil.ReadFile(request.CaFile)
		if err != nil {
			return fmt.Errorf("failed to append %q to RootCAs: %v", certs, err)
		}
		if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
			return fmt.Errorf("unable to add certs specified in %s", request.CaFile)
		}
	}

	var reg image.Registry
	var rerr error
	switch request.ContainerTool {
	case containertools.NoneTool:
		reg, rerr = containerdregistry.NewRegistry(containerdregistry.SkipTLS(request.SkipTLS), containerdregistry.WithRootCAs(rootCAs))
	case containertools.PodmanTool:
		fallthrough
	case containertools.DockerTool:
		reg, rerr = execregistry.NewRegistry(request.ContainerTool, r.Logger)
	}
	if rerr != nil {
		return rerr
	}
	defer func() {
		if err := reg.Destroy(); err != nil {
			r.Logger.WithError(err).Warn("error destroying local cache")
		}
	}()

	simpleRefs := make([]image.Reference, 0)
	for _, ref := range request.Bundles {
		simpleRefs = append(simpleRefs, image.SimpleReference(ref))
	}

	if err := populate(context.TODO(), dbLoader, graphLoader, dbQuerier, reg, simpleRefs, request.Mode); err != nil {
		r.Logger.Debugf("unable to populate database: %s", err)

		if !request.Permissive {
			r.Logger.WithError(err).Error("permissive mode disabled")
			return err
		} else {
			r.Logger.WithError(err).Warn("permissive mode enabled")
		}
	}

	return nil
}

func populate(ctx context.Context, loader registry.Load, graphLoader registry.GraphLoader, querier registry.Query, reg image.Registry, refs []image.Reference, mode registry.Mode) error {
	var errs []error

	unpackedImageMap := make(map[image.Reference]string, 0)
	for _, ref := range refs {
		workingDir, err := ioutil.TempDir("./", "bundle_tmp")
		if err != nil {
			errs = append(errs, err)
			continue
		}
		defer os.RemoveAll(workingDir)

		if err = reg.Pull(ctx, ref); err != nil {
			errs = append(errs, err)
			continue
		}

		if err = reg.Unpack(ctx, ref, workingDir); err != nil {
			errs = append(errs, err)
			continue
		}

		unpackedImageMap[ref] = workingDir
	}

	if len(errs) > 0 {
		return utilerrors.NewAggregate(errs)
	}

	populator := registry.NewDirectoryPopulator(loader, graphLoader, querier, unpackedImageMap)

	return populator.Populate(mode)
}

type DeleteFromRegistryRequest struct {
	Permissive    bool
	InputDatabase string
	Packages      []string
}

func (r RegistryUpdater) DeleteFromRegistry(request DeleteFromRegistryRequest) error {
	db, err := sql.Open("sqlite3", request.InputDatabase)
	if err != nil {
		return err
	}
	defer db.Close()

	dbLoader, err := sqlite.NewSQLLiteLoader(db)
	if err != nil {
		return err
	}
	if err := dbLoader.Migrate(context.TODO()); err != nil {
		return err
	}

	for _, pkg := range request.Packages {
		remover := sqlite.NewSQLRemoverForPackages(dbLoader, pkg)
		if err := remover.Remove(); err != nil {
			err = fmt.Errorf("error deleting packages from database: %s", err)
			if !request.Permissive {
				logrus.WithError(err).Fatal("permissive mode disabled")
				return err
			}
			logrus.WithError(err).Warn("permissive mode enabled")
		}
	}

	// remove any stranded bundles from the database
	// TODO: This is unnecessary if the db schema can prevent this orphaned data from existing
	remover := sqlite.NewSQLStrandedBundleRemover(dbLoader)
	if err := remover.Remove(); err != nil {
		return fmt.Errorf("error removing stranded packages from database: %s", err)
	}

	return nil
}

type PruneStrandedFromRegistryRequest struct {
	InputDatabase string
}

func (r RegistryUpdater) PruneStrandedFromRegistry(request PruneStrandedFromRegistryRequest) error {
	db, err := sql.Open("sqlite3", request.InputDatabase)
	if err != nil {
		return err
	}
	defer db.Close()

	dbLoader, err := sqlite.NewSQLLiteLoader(db)
	if err != nil {
		return err
	}
	if err := dbLoader.Migrate(context.TODO()); err != nil {
		return err
	}

	remover := sqlite.NewSQLStrandedBundleRemover(dbLoader)
	if err := remover.Remove(); err != nil {
		return fmt.Errorf("error removing stranded packages from database: %s", err)
	}

	return nil
}

type PruneFromRegistryRequest struct {
	Permissive    bool
	InputDatabase string
	Packages      []string
}

func (r RegistryUpdater) PruneFromRegistry(request PruneFromRegistryRequest) error {
	db, err := sql.Open("sqlite3", request.InputDatabase)
	if err != nil {
		return err
	}
	defer db.Close()

	dbLoader, err := sqlite.NewSQLLiteLoader(db)
	if err != nil {
		return err
	}
	if err := dbLoader.Migrate(context.TODO()); err != nil {
		return err
	}

	// get all the packages
	lister := sqlite.NewSQLLiteQuerierFromDb(db)
	packages, err := lister.ListPackages(context.TODO())
	if err != nil {
		return err
	}

	// make it inexpensive to find packages
	pkgMap := make(map[string]bool)
	for _, pkg := range request.Packages {
		pkgMap[pkg] = true
	}

	// prune packages from registry
	for _, pkg := range packages {
		if _, found := pkgMap[pkg]; !found {
			remover := sqlite.NewSQLRemoverForPackages(dbLoader, pkg)
			if err := remover.Remove(); err != nil {
				err = fmt.Errorf("error deleting packages from database: %s", err)
				if !request.Permissive {
					logrus.WithError(err).Fatal("permissive mode disabled")
					return err
				}
				logrus.WithError(err).Warn("permissive mode enabled")
			}
		}
	}

	return nil
}

type DeprecateFromRegistryRequest struct {
	Permissive    bool
	InputDatabase string
	Bundles       []string
}

func (r RegistryUpdater) DeprecateFromRegistry(request DeprecateFromRegistryRequest) error {
	db, err := sql.Open("sqlite3", request.InputDatabase)
	if err != nil {
		return err
	}
	defer db.Close()

	dbLoader, err := sqlite.NewSQLLiteLoader(db)
	if err != nil {
		return err
	}
	if err := dbLoader.Migrate(context.TODO()); err != nil {
		return fmt.Errorf("unable to migrate database: %s", err)
	}

	deprecator := sqlite.NewSQLDeprecatorForBundles(dbLoader, request.Bundles)
	if err := deprecator.Deprecate(); err != nil {
		r.Logger.Debugf("unable to deprecate bundles from database: %s", err)
		if !request.Permissive {
			r.Logger.WithError(err).Error("permissive mode disabled")
			return err
		}
		r.Logger.WithError(err).Warn("permissive mode enabled")
	}

	return nil
}
