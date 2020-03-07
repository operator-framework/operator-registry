package registry

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/sirupsen/logrus"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/image/unprivileged"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

type RegistryUpdater struct {
	Logger *logrus.Entry
}

type AddToRegistryRequest struct {
	Permissive    bool
	SkipTLS       bool
	InputDatabase string
	Bundles       []string
}

func (r RegistryUpdater) AddToRegistry(request AddToRegistryRequest) error {
	var errs []error

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

	reg, err := unprivileged.NewRegistry(
		unprivileged.SkipTLS(request.SkipTLS),
	)
	if err != nil {
		return err
	}
	defer func() {
		if err := reg.Close(); err != nil {
			r.Logger.WithError(err).Warn("error closing local image registry")
		}
	}()

	// TODO(njhale): Parallelize this once bundle add is commutative
	for _, ref := range request.Bundles {
		if err := populate(context.TODO(), dbLoader, reg, ref); err != nil {
			err = fmt.Errorf("error loading bundle from image: %s", err)
			if !request.Permissive {
				r.Logger.WithError(err).Error("permissive mode disabled")
				errs = append(errs, err)
			} else {
				r.Logger.WithError(err).Warn("permissive mode enabled")
			}
		}
	}

	return utilerrors.NewAggregate(errs) // nil if no errors
}

func populate(ctx context.Context, loader registry.Load, reg image.Registry, ref string) error {
	workingDir, err := ioutil.TempDir("./", "bundle_tmp")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workingDir)

	if err = reg.Pull(ctx, ref); err != nil {
		return err
	}

	if err = reg.Unpack(ctx, ref, workingDir); err != nil {
		return err
	}

	populator := registry.NewDirectoryPopulator(loader, workingDir, ref)

	return populator.Populate()
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

	return nil
}
