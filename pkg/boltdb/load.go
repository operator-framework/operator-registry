package boltdb

import (
	"fmt"

	"github.com/asdine/storm/v3"
	"github.com/asdine/storm/v3/q"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/operator-framework/operator-registry/pkg/registry"
)

type StormLoader struct {
	db *storm.DB
}

var _ registry.Load = &StormLoader{}

func NewStormLoader(db *storm.DB) *StormLoader {
	return &StormLoader{db: db}
}

func (s *StormLoader) AddOperatorBundle(bundle *registry.Bundle) error {
	tx, err := s.db.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Add the core bundle
	opBundle, err := newOperatorBundle(bundle)
	if err != nil {
		return err
	}

	if err = tx.Save(opBundle); err != nil {
		return err
	}

	// Add provided and required APIs
	apis, err := relatedAPIs(bundle)
	if err != nil {
		return err
	}

	for _, api := range apis {
		if err = tx.Save(api); err != nil {
			return err
		}
	}

	// Add related images
	images, err := bundle.Images()
	if err != nil {
		return err
	}

	for image := range images {
		err = tx.Save(&RelatedImage{
			ImageUser: ImageUser{
				OperatorBundleName: opBundle.Name,
				Image:              image,
			},
		})

		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *StormLoader) AddBundlePackageChannels(manifest registry.PackageManifest, bundle registry.Bundle) error {
	panic("implement me")
}

func (s *StormLoader) AddPackageChannels(manifest registry.PackageManifest) error {
	tx, err := s.db.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	pkg := Package{
		Name:           manifest.PackageName,
		DefaultChannel: manifest.DefaultChannelName,
	}
	if err = tx.Save(&pkg); err != nil {
		return err
	}

	var errs []error
	for _, channel := range manifest.Channels {
		// Get and store the update graph starting at the channel head
		pkgChannel := PackageChannel{
			ChannelName: channel.Name,
			PackageName: pkg.Name,
		}
		err = tx.Save(&Channel{
			PackageChannel:         pkgChannel,
			HeadOperatorBundleName: channel.CurrentCSVName,
		})
		if err != nil {
			errs = append(errs, err)
		}

		entries, err := s.updateGraph(pkg.Name, channel.Name, channel.CurrentCSVName)
		if err != nil {
			errs = append(errs, err)
		}

		for _, entry := range entries {
			if err = tx.Save(&entry); err != nil {
				errs = append(errs, err)
				continue
			}

			// Store the latest provided
			provided, err := s.providedAPIs(entry.OperatorBundleName)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			for _, provider := range provided {
				err = tx.Save(&LatestGVKProvider{
					PackageChannelGVK: PackageChannelGVK{
						PackageChannel: pkgChannel,
						GVK:            provider.GVK,
					},
				})

				if err != nil && err != storm.ErrAlreadyExists {
					errs = append(errs, err)
				}
			}
		}

	}

	errs = append(errs, tx.Commit())

	return utilerrors.NewAggregate(errs)
}

func (s *StormLoader) RmPackageName(packageName string) error {
	panic("implement me")
}

func (s *StormLoader) ClearNonDefaultBundles(packageName string) error {
	panic("implement me")
}

func (s *StormLoader) updateGraph(pkgName, channelName, operatorBundleName string) (entries []ChannelEntry, err error) {
	var head OperatorBundle
	if err = s.db.One("Name", operatorBundleName, &head); err != nil {
		return
	}

	pkgChannel := PackageChannel{
		PackageName: pkgName,
		ChannelName: channelName,
	}

	// Traverse the graph, picking up entries along the way
	visited := map[string]struct{}{}
	for o := head; err == nil; {
		if _, ok := visited[o.Name]; ok {
			err = fmt.Errorf("update graph cycle detected, %s appears more than once", o.Name)
			return
		}

		entry := ChannelEntry{
			ChannelReplacement: ChannelReplacement{
				PackageChannel:     pkgChannel,
				OperatorBundleName: o.Name,
				Replaces:           o.Replaces,
			},
		}
		entries = append(entries, entry)

		// Mark the bundle as visited
		visited[o.Name] = struct{}{}

		// Add dummy entries for skipped CSVs
		skipReplaces := false
		for _, skip := range o.Skips {
			if skipReplaces = (skip == o.Replaces); skipReplaces {
				continue
			}

			if _, ok := visited[skip]; ok {
				err = fmt.Errorf("update graph cycle detected, %s appears more than once", skip)
				return
			}

			entry.Replaces = skip
			entries = append(entries, entry)

			visited[skip] = struct{}{}
		}

		if o.Replaces == "" || skipReplaces {
			break
		}

		if err = s.db.One("Name", o.Replaces, &o); err == storm.ErrNotFound {
			// Make the error more verbose
			err = fmt.Errorf("%s specifies replacement that couldn't be found", head.Name)
		}

	}

	return
}

func (s *StormLoader) providedAPIs(operatorBundleName string) (provided []RelatedAPI, err error) {
	err = s.db.Select(q.Eq("OperatorBundleName", operatorBundleName), q.Eq("Provides", true)).Find(&provided)
	if err == storm.ErrNotFound {
		err = nil
	}

	return
}

func newOperatorBundle(bundle *registry.Bundle) (*OperatorBundle, error) {
	// Add the core bundle
	csvName, bundleImage, csvBytes, bundleBytes, err := bundle.Serialize()
	if err != nil {
		return nil, err
	}
	if csvName == "" {
		return nil, fmt.Errorf("csv name not found")
	}
	version, err := bundle.Version()
	if err != nil {
		return nil, err
	}
	replaces, err := bundle.Replaces()
	if err != nil {
		return nil, err
	}
	skipRange, err := bundle.SkipRange()
	if err != nil {
		return nil, err
	}
	skips, err := bundle.Skips()
	if err != nil {
		return nil, err
	}

	opBundle := &OperatorBundle{
		Name:       csvName,
		Version:    version,
		Replaces:   replaces,
		SkipRange:  skipRange,
		Skips:      skips,
		CSV:        csvBytes,
		Bundle:     bundleBytes,
		BundlePath: bundleImage,
	}

	return opBundle, nil
}

func relatedAPIs(bundle *registry.Bundle) (apis []RelatedAPI, err error) {
	addAPIs := func(keys map[registry.APIKey]struct{}, provides bool) {
		for k := range keys {
			apis = append(apis, RelatedAPI{
				GVKUser: GVKUser{
					GVK: GVK{
						Group:   k.Group,
						Version: k.Version,
						Kind:    k.Kind,
					},
					OperatorBundleName: bundle.Name,
				},
				Plural:   k.Plural,
				Provides: provides,
			})
		}
	}
	var providedAPIs map[registry.APIKey]struct{}
	providedAPIs, err = bundle.ProvidedAPIs()
	if err != nil {
		return
	}
	addAPIs(providedAPIs, true)

	var requiredAPIs map[registry.APIKey]struct{}
	if requiredAPIs, err = bundle.RequiredAPIs(); err != nil {
		return
	}
	addAPIs(requiredAPIs, false)

	return
}
