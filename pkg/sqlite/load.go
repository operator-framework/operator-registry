package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/blang/semver"
	_ "github.com/mattn/go-sqlite3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/operator-framework/operator-registry/pkg/registry"
)

type SQLLoader struct {
	db       *sql.DB
	migrator Migrator
}

var _ registry.Load = &SQLLoader{}

func NewSQLLiteLoader(db *sql.DB, opts ...DbOption) (*SQLLoader, error) {
	options := defaultDBOptions()
	for _, o := range opts {
		o(options)
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON", nil); err != nil {
		return nil, err
	}

	migrator, err := options.MigratorBuilder(db)
	if err != nil {
		return nil, err
	}

	return &SQLLoader{db: db, migrator: migrator}, nil
}

func (s *SQLLoader) Migrate(ctx context.Context) error {
	if s.migrator == nil {
		return fmt.Errorf("no migrator configured")
	}
	return s.migrator.Migrate(ctx)
}

func (s *SQLLoader) AddOperatorBundle(bundle *registry.Bundle) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		tx.Rollback()
	}()

	stmt, err := tx.Prepare("insert into operatorbundle(name, csv, bundle, bundlepath, version, skiprange) values(?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	addImage, err := tx.Prepare("insert into related_image(image, operatorbundle_name) values(?,?)")
	if err != nil {
		return err
	}
	defer addImage.Close()

	csvName, bundleImage, csvBytes, bundleBytes, err := bundle.Serialize()
	if err != nil {
		return err
	}

	if csvName == "" {
		return fmt.Errorf("csv name not found")
	}

	version, err := bundle.Version()
	if err != nil {
		return err
	}
	skiprange, err := bundle.SkipRange()
	if err != nil {
		return err
	}

	if _, err := stmt.Exec(csvName, csvBytes, bundleBytes, bundleImage, version, skiprange); err != nil {
		return err
	}

	imgs, err := bundle.Images()
	if err != nil {
		return err
	}
	// TODO: bulk insert
	for img := range imgs {
		if _, err := addImage.Exec(img, csvName); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLLoader) AddPackageChannels(manifest registry.PackageManifest, updateMode registry.Mode, opts ...registry.ChannelUpdateOption) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		tx.Rollback()
	}()

	options := registry.ChannelUpdateOptions{}
	for _, o := range opts {
		o(&options)
	}

	addPackage, err := tx.Prepare("insert into package(name) values(?)")
	if err != nil {
		return err
	}
	defer addPackage.Close()

	addDefaultChannel, err := tx.Prepare("update package set default_channel = ? where name = ?")
	if err != nil {
		return err
	}
	defer addDefaultChannel.Close()

	addChannel, err := tx.Prepare("insert into channel(name, package_name, head_operatorbundle_name) values(?, ?, ?)")
	if err != nil {
		return err
	}
	defer addChannel.Close()

	addChannelEntry, err := tx.Prepare("insert into channel_entry(channel_name, package_name, operatorbundle_name, depth) values(?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer addChannelEntry.Close()

	addReplaces, err := tx.Prepare("update channel_entry set replaces = ? where entry_id = ?")
	if err != nil {
		return err
	}
	defer addReplaces.Close()

	var errs []error

	// Try to add the package. If it's not new, just update the existing entries
	if _, err := addPackage.Exec(manifest.PackageName); err != nil {
		err = s.updatePackageChannels(tx, manifest, updateMode, options.CSVToInsert)
		if err != nil {
			errs = append(errs, err)
		}

		if err := tx.Commit(); err != nil {
			errs = append(errs, err)
		}

		return utilerrors.NewAggregate(errs)
	}

	hasDefault := false
	for _, c := range manifest.Channels {
		if _, err := addChannel.Exec(c.Name, manifest.PackageName, c.CurrentCSVName); err != nil {
			errs = append(errs, err)
			continue
		}
		if c.IsDefaultChannel(manifest) {
			hasDefault = true
			if _, err := addDefaultChannel.Exec(c.Name, manifest.PackageName); err != nil {
				errs = append(errs, err)
				continue
			}
		}
	}
	if !hasDefault {
		errs = append(errs, fmt.Errorf("no default channel specified for %s", manifest.PackageName))
	}

	for _, c := range manifest.Channels {
		res, err := addChannelEntry.Exec(c.Name, manifest.PackageName, c.CurrentCSVName, 0)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		currentID, err := res.LastInsertId()
		if err != nil {
			errs = append(errs, err)
			continue
		}

		channelEntryCSVName := c.CurrentCSVName
		depth := 1

		// Since this loop depends on following 'replaces', keep track of where it's been
		replaceCycle := map[string]bool{channelEntryCSVName: true}
		for {
			// Get CSV for current entry
			channelEntryCSV, err := s.getCSV(tx, channelEntryCSVName)
			if err != nil {
				errs = append(errs, err)
				break
			}

			if err := s.addAPIs(tx, channelEntryCSV, currentID); err != nil {
				errs = append(errs, err)
			}

			// If we aren't in `replaces` mode, then we already know this is the first entry into the channel
			// otherwise we would have gone into the update function. No need to trace back and update
			// previous entries
			if updateMode != registry.ReplacesMode {
				break
			}

			skips, err := channelEntryCSV.GetSkips()
			if err != nil {
				errs = append(errs, err)
			}

			for _, skip := range skips {
				// add dummy channel entry for the skipped version
				skippedChannelEntry, err := addChannelEntry.Exec(c.Name, manifest.PackageName, skip, depth)
				if err != nil {
					errs = append(errs, err)
					continue
				}

				skippedID, err := skippedChannelEntry.LastInsertId()
				if err != nil {
					errs = append(errs, err)
					continue
				}

				// add another channel entry for the parent, which replaces the skipped
				synthesizedChannelEntry, err := addChannelEntry.Exec(c.Name, manifest.PackageName, channelEntryCSVName, depth)
				if err != nil {
					errs = append(errs, err)
					continue
				}

				synthesizedID, err := synthesizedChannelEntry.LastInsertId()
				if err != nil {
					errs = append(errs, err)
					continue
				}

				if _, err = addReplaces.Exec(skippedID, synthesizedID); err != nil {
					errs = append(errs, err)
					continue
				}

				if err := s.addAPIs(tx, channelEntryCSV, synthesizedID); err != nil {
					errs = append(errs, err)
					continue
				}

				depth++
			}

			// create real replacement chain
			replaces, err := channelEntryCSV.GetReplaces()
			if err != nil {
				errs = append(errs, err)
				break
			}

			if replaces == "" {
				// we've walked the channel until there was no replacement
				break
			}

			replacedChannelEntry, err := addChannelEntry.Exec(c.Name, manifest.PackageName, replaces, depth)
			if err != nil {
				errs = append(errs, err)
				break
			}

			// If we find 'replaces' in the circuit list then we've seen it already, break out
			if _, ok := replaceCycle[replaces]; ok {
				errs = append(errs, fmt.Errorf("Cycle detected, %s replaces %s", channelEntryCSVName, replaces))
				break
			}
			replaceCycle[replaces] = true

			replacedID, err := replacedChannelEntry.LastInsertId()
			if err != nil {
				errs = append(errs, err)
				break
			}
			if _, err = addReplaces.Exec(replacedID, currentID); err != nil {
				errs = append(errs, err)
				break
			}
			if _, err := s.getCSV(tx, replaces); err != nil {
				errs = append(errs, fmt.Errorf("%s specifies replacement that couldn't be found", c.CurrentCSVName))
				break
			}

			currentID = replacedID
			channelEntryCSVName = replaces
			depth++
		}
	}

	if err := tx.Commit(); err != nil {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}

func (s *SQLLoader) ClearNonDefaultBundles(packageName string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		tx.Rollback()
	}()

	// First find the default channel for the package
	getDefChan, err := tx.Prepare(fmt.Sprintf("select default_channel from package where name='%s'", packageName))
	if err != nil {
		return err
	}
	defer getDefChan.Close()

	defaultChannelRows, err := getDefChan.Query()
	if err != nil {
		return err
	}
	defer defaultChannelRows.Close()

	if !defaultChannelRows.Next() {
		return fmt.Errorf("no default channel found for package %s", packageName)
	}
	var defaultChannel sql.NullString
	if err := defaultChannelRows.Scan(&defaultChannel); err != nil {
		return err
	}

	// Then get the head of the default channel
	getChanHead, err := tx.Prepare(fmt.Sprintf("select head_operatorbundle_name from channel where name='%s'", defaultChannel.String))
	if err != nil {
		return err
	}
	defer getChanHead.Close()

	chanHeadRows, err := getChanHead.Query()
	if err != nil {
		return err
	}
	defer chanHeadRows.Close()

	if !chanHeadRows.Next() {
		return fmt.Errorf("no channel head found for default channel %s", defaultChannel.String)
	}
	var defChanHead sql.NullString
	if err := chanHeadRows.Scan(&defChanHead); err != nil {
		return err
	}

	// Now get all the bundles that are not the head of the default channel
	getChannelBundles, err := tx.Prepare(fmt.Sprintf("SELECT operatorbundle_name FROM channel_entry WHERE package_name='%s' AND operatorbundle_name!='%s'", packageName, defChanHead.String))
	if err != nil {
		return err
	}
	defer getChanHead.Close()

	chanBundleRows, err := getChannelBundles.Query()
	if err != nil {
		return err
	}
	defer chanBundleRows.Close()

	bundles := make(map[string]struct{}, 0)
	for chanBundleRows.Next() {
		var bundleToUpdate sql.NullString
		if err := chanBundleRows.Scan(&bundleToUpdate); err != nil {
			return err
		}
		bundles[bundleToUpdate.String] = struct{}{}
	}

	if len(bundles) > 0 {
		bundlePredicates := []string{}
		for bundle := range bundles {
			bundlePredicates = append(bundlePredicates, fmt.Sprintf("name = '%s'", bundle))
		}

		var transactionPredicate string
		if len(bundlePredicates) == 1 {
			transactionPredicate = fmt.Sprintf("WHERE %s AND bundlepath != \"\"", bundlePredicates[0])
		} else {
			transactionPredicate = fmt.Sprintf("WHERE (%s) AND bundlepath != \"\"", strings.Join(bundlePredicates, " OR "))
		}

		removeOldBundles, err := tx.Prepare(fmt.Sprintf("UPDATE operatorbundle SET bundle = null, csv = null %s", transactionPredicate))
		if err != nil {
			return err
		}

		_, err = removeOldBundles.Exec()
		if err != nil {
			return fmt.Errorf("Unable to remove previous bundles: %s", err)
		}
	}

	return tx.Commit()
}

func SplitCRDName(crdName string) (plural, group string, err error) {
	pluralGroup := strings.SplitN(crdName, ".", 2)
	if len(pluralGroup) != 2 {
		err = fmt.Errorf("can't split bad CRD name %s", crdName)
		return
	}

	plural = pluralGroup[0]
	group = pluralGroup[1]
	return
}

func (s *SQLLoader) getCSV(tx *sql.Tx, csvName string) (*registry.ClusterServiceVersion, error) {
	getCSV, err := tx.Prepare(`
	  SELECT DISTINCT operatorbundle.csv 
	  FROM operatorbundle
	  WHERE operatorbundle.name=? LIMIT 1`)
	if err != nil {
		return nil, err
	}
	defer getCSV.Close()

	rows, err := getCSV.Query(csvName)
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		return nil, fmt.Errorf("no bundle found for csv %s", csvName)
	}
	var csvStringSQL sql.NullString
	if err := rows.Scan(&csvStringSQL); err != nil {
		return nil, err
	}

	if !csvStringSQL.Valid {
		return nil, fmt.Errorf("csv %s not stored for non-latest versions", csvName)
	}

	dec := yaml.NewYAMLOrJSONDecoder(strings.NewReader(csvStringSQL.String), 10)
	unst := &unstructured.Unstructured{}
	if err := dec.Decode(unst); err != nil {
		return nil, fmt.Errorf("can't decode %s: %s", csvStringSQL.String, err)
	}

	csv := &registry.ClusterServiceVersion{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unst.UnstructuredContent(), csv); err != nil {
		return nil, err
	}

	return csv, nil
}

func (s *SQLLoader) addAPIs(tx *sql.Tx, csv *registry.ClusterServiceVersion, channelEntryId int64) error {
	addAPI, err := tx.Prepare("insert or replace into api(group_name, version, kind, plural) values(?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer addAPI.Close()

	addAPIProvider, err := tx.Prepare("insert into api_provider(group_name, version, kind, channel_entry_id) values(?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer addAPIProvider.Close()

	addApiRequirer, err := tx.Prepare("insert into api_requirer(group_name, version, kind, channel_entry_id) values(?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer addApiRequirer.Close()

	ownedCRDs, requiredCRDs, err := csv.GetCustomResourceDefintions()
	if err != nil {
		return err
	}
	for _, crd := range ownedCRDs {
		plural, group, err := SplitCRDName(crd.Name)
		if err != nil {
			return err
		}
		if _, err := addAPI.Exec(group, crd.Version, crd.Kind, plural); err != nil {
			return err
		}
		if _, err := addAPIProvider.Exec(group, crd.Version, crd.Kind, channelEntryId); err != nil {
			return err
		}
	}
	for _, crd := range requiredCRDs {
		plural, group, err := SplitCRDName(crd.Name)
		if err != nil {
			return err
		}
		if _, err := addAPI.Exec(group, crd.Version, crd.Kind, plural); err != nil {
			return err
		}
		if _, err := addApiRequirer.Exec(group, crd.Version, crd.Kind, channelEntryId); err != nil {
			return err
		}
	}

	ownedAPIs, requiredAPIs, err := csv.GetApiServiceDefinitions()
	if err != nil {
		return err
	}
	for _, api := range ownedAPIs {
		if _, err := addAPI.Exec(api.Group, api.Version, api.Kind, api.Name); err != nil {
			return err
		}
		if _, err := addAPIProvider.Exec(api.Group, api.Version, api.Kind, channelEntryId); err != nil {
			return err
		}
	}
	for _, api := range requiredAPIs {
		if _, err := addAPI.Exec(api.Group, api.Version, api.Kind, api.Name); err != nil {
			return err
		}
		if _, err := addApiRequirer.Exec(api.Group, api.Version, api.Kind, channelEntryId); err != nil {
			return err
		}
	}
	return nil
}
func (s *SQLLoader) getCSVNames(tx *sql.Tx, packageName string) ([]string, error) {
	getID, err := tx.Prepare(`
	  SELECT DISTINCT channel_entry.operatorbundle_name
	  FROM channel_entry
	  WHERE channel_entry.package_name=?`)

	if err != nil {
		return nil, err
	}
	defer getID.Close()

	rows, err := getID.Query(packageName)
	if err != nil {
		return nil, err
	}

	var csvName string
	csvNames := []string{}
	for rows.Next() {
		err := rows.Scan(&csvName)
		if err != nil {
			return nil, err
		}
		csvNames = append(csvNames, csvName)
	}

	if err := rows.Close(); err != nil {
		return nil, err
	}

	return csvNames, nil
}

func (s *SQLLoader) rmAPIs(tx *sql.Tx, csv *registry.ClusterServiceVersion) error {
	rmAPI, err := tx.Prepare("delete from api where group_name=? AND version=? AND kind=?")
	if err != nil {
		return err
	}
	defer rmAPI.Close()

	ownedCRDs, _, err := csv.GetCustomResourceDefintions()
	for _, crd := range ownedCRDs {
		_, group, err := SplitCRDName(crd.Name)
		if err != nil {
			return err
		}
		if _, err := rmAPI.Exec(group, crd.Version, crd.Kind); err != nil {
			return err
		}
	}

	return nil
}

func (s *SQLLoader) RmPackageName(packageName string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		tx.Rollback()
	}()

	csvNames, err := s.getCSVNames(tx, packageName)
	if err != nil {
		return err
	}
	for _, csvName := range csvNames {
		csv, err := s.getCSV(tx, csvName)
		if csv != nil {
			err = s.rmBundle(tx, csvName)
			if err != nil {
				return err
			}
			err = s.rmAPIs(tx, csv)
			if err != nil {
				return err
			}
		} else {
			err = s.rmBundle(tx, csvName)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func (s *SQLLoader) rmBundle(tx *sql.Tx, csvName string) error {
	stmt, err := tx.Prepare("DELETE FROM operatorbundle WHERE operatorbundle.name=?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	if _, err := stmt.Exec(csvName); err != nil {
		return err
	}

	return nil
}

func (s *SQLLoader) AddBundlePackageChannels(manifest registry.PackageManifest, bundle registry.Bundle, updateMode registry.Mode) error {
	var errs []error
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		tx.Rollback()
	}()

	stmt, err := tx.Prepare("insert into operatorbundle(name, csv, bundle, bundlepath, version, skiprange) values(?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	addImage, err := tx.Prepare("insert into related_image(image, operatorbundle_name) values(?,?)")
	if err != nil {
		return err
	}
	defer addImage.Close()

	csvName, bundleImage, csvBytes, bundleBytes, err := bundle.Serialize()
	if err != nil {
		return err
	}

	if csvName == "" {
		return fmt.Errorf("csv name not found")
	}

	version, err := bundle.Version()
	if err != nil {
		return err
	}
	skiprange, err := bundle.SkipRange()
	if err != nil {
		return err
	}

	if _, err := stmt.Exec(csvName, csvBytes, bundleBytes, bundleImage, version, skiprange); err != nil {
		return err
	}

	imgs, err := bundle.Images()
	if err != nil {
		return err
	}
	// TODO: bulk insert
	for img := range imgs {
		if _, err := addImage.Exec(img, csvName); err != nil {
			return err
		}
	}

	// Insert operator bundles and related images
	if err := tx.Commit(); err != nil {
		return err
	}

	if err := s.AddPackageChannels(manifest, updateMode, registry.WithCSVToInsert(csvName)); err != nil {
		errs = append(errs, err)
		tx, err := s.db.Begin()
		if err != nil {
			errs = append(errs, err)
			return utilerrors.NewAggregate(errs)
		}
		defer func() {
			tx.Rollback()
		}()

		if err := s.rmBundle(tx, csvName); err != nil {
			errs = append(errs, err)
			return utilerrors.NewAggregate(errs)
		}

		if err := tx.Commit(); err != nil {
			errs = append(errs, err)
		}

		return utilerrors.NewAggregate(errs)
	}

	return nil
}

func (s *SQLLoader) updatePackageChannels(tx *sql.Tx, manifest registry.PackageManifest, updateMode registry.Mode, csvToUpdate string) error {
	updateDefaultChannel, err := tx.Prepare("update package set default_channel = ? where name = ?")
	if err != nil {
		return err
	}
	defer updateDefaultChannel.Close()

	getDefaultChannel, err := tx.Prepare(`SELECT default_channel FROM package WHERE name = ? LIMIT 1`)
	if err != nil {
		return err
	}
	defer getDefaultChannel.Close()

	updateChannel, err := tx.Prepare("update channel set head_operatorbundle_name = ? where name = ? and package_name = ?")
	if err != nil {
		return err
	}
	defer updateChannel.Close()

	addChannelEntry, err := tx.Prepare("insert into channel_entry(channel_name, package_name, operatorbundle_name, depth) values(?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer addChannelEntry.Close()

	updateChannelEntry, err := tx.Prepare("update channel_entry set depth = ? where channel_name = ? and package_name = ? and operatorbundle_name = ?")
	if err != nil {
		return err
	}
	defer updateChannelEntry.Close()

	addReplaces, err := tx.Prepare("update channel_entry set replaces = ? where entry_id = ?")
	if err != nil {
		return err
	}
	defer addReplaces.Close()

	getDepth, err := tx.Prepare(`
	  SELECT channel_entry.depth, channel_entry.entry_id
	  FROM channel_entry
	  WHERE channel_name = ? and package_name = ? and operatorbundle_name =?
	  LIMIT 1`)
	if err != nil {
		return err
	}
	defer getDepth.Close()

	getChannelEntryID, err := tx.Prepare(`
	  SELECT channel_entry.entry_id
	  FROM channel_entry
	  WHERE channel_name = ? and package_name = ? and operatorbundle_name =?
	  LIMIT 1`)
	if err != nil {
		return err
	}
	defer getChannelEntryID.Close()

	updateDepth, err := tx.Prepare("update channel_entry set depth = depth + 1 where channel_name = ? and package_name = ? and operatorbundle_name = ?")
	if err != nil {
		return err
	}
	defer updateDepth.Close()

	updateDepthById, err := tx.Prepare("update channel_entry set depth = depth + 1 where entry_id = ?")
	if err != nil {
		return err
	}
	defer updateDepth.Close()

	updateDepthByIdToValue, err := tx.Prepare("update channel_entry set depth = ? where entry_id = ?")
	if err != nil {
		return err
	}
	defer updateDepth.Close()

	removeSkipped, err := tx.Prepare("delete from channel_entry where channel_name = ? and package_name = ? and operatorbundle_name = ?")
	if err != nil {
		return err
	}
	defer removeSkipped.Close()

	getBundleIDNameFromDepthToHead, err := tx.Prepare(`
	  SELECT entry_id, operatorbundle_name
	  FROM channel_entry
	  WHERE depth < ? and channel_name = ? and package_name = ?`)
	if err != nil {
		return err
	}
	defer getBundleIDNameFromDepthToHead.Close()

	channelExists, err := tx.Prepare("select 1 from channel where name = ? and package_name = ?")
	if err != nil {
		return err
	}
	defer channelExists.Close()

	addChannel, err := tx.Prepare("insert or ignore into channel(name, package_name, head_operatorbundle_name) values(?, ?, ?)")
	if err != nil {
		return err
	}
	defer addChannel.Close()

	var errs []error

	for _, c := range manifest.Channels {
		// check if channel exists
		rows, err := channelExists.Query(c.Name, manifest.PackageName)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		// if it doesn't, add it
		if !rows.Next() {
			if _, err := addChannel.Exec(c.Name, manifest.PackageName, csvToUpdate); err != nil {
				errs = append(errs, err)
				continue
			}
		}
	}

	// For each channel, check where in update graph
	// the bundle is attempted to be inserted.
	// If not at the head of the channel then error
	for _, c := range manifest.Channels {
		if updateMode == registry.SemVerMode || updateMode == registry.SkipPatchMode {
			// get all of the existing channel entries and versions for this package and channel
			bundleChannelEntries, err := s.getBundlEntriesForChannel(tx, c.Name, manifest.PackageName)
			if err != nil {
				errs = append(errs, err)
				break
			}

			// what version of the bundle are we adding?
			newCSV, err := s.getCSV(tx, csvToUpdate)
			if err != nil {
				errs = append(errs, err)
				break
			}

			v, err := newCSV.GetVersion()
			if err != nil {
				errs = append(errs, fmt.Errorf("Unable to retrieve Version from CSV %s", csvToUpdate))
				break
			}
			versionToAdd, err := semver.Make(v)
			if err != nil {
				errs = append(errs, fmt.Errorf("Version in new CSV %s is not valid %s", csvToUpdate, v))
			}

			// insert the new entry with some default values to generate an id
			res, err := addChannelEntry.Exec(c.Name, manifest.PackageName, csvToUpdate, 0)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			currentID, err := res.LastInsertId()
			if err != nil {
				errs = append(errs, err)
				continue
			}

			// add APIs
			if err := s.addAPIs(tx, newCSV, currentID); err != nil {
				errs = append(errs, err)
				continue
			}

			newEntry := bundleChannelEntry{
				version: versionToAdd,
				entryId: int(currentID),
			}

			// keep track of whether or not we are pushing out an existing minor version
			// we don't want to increase the depth of lower entries if we are just fanning out
			skipPatchFoundVersion := false

			// Iterate over all entries to sort and determine where the new version fits into the graph
			behindEntries := make(map[int]bundleChannelEntry, 0)
			aheadEntries := make(map[int]bundleChannelEntry, 0)
			for _, entry := range bundleChannelEntries {
				// compare the existing entry to the one we want to add
				comparison := entry.version.Compare(versionToAdd)

				// if the version matches, throw an error. the bundle entry should have caught this already
				if comparison == 0 {
					errs = append(errs, fmt.Errorf("Attempting to add an existing version"))
				}

				// now let's see if the version is higher or lower than the new entry
				if updateMode == registry.SkipPatchMode {
					// if we are in skippatch mode, ignore entries that have the same major and minor version
					if entry.version.Major == versionToAdd.Major && entry.version.Minor == versionToAdd.Minor {
						skipPatchFoundVersion = true
						continue
					}
				}

				// greater than
				if comparison == 1 {
					aheadEntries[entry.entryId] = entry
				}

				// less than
				if comparison == -1 {
					behindEntries[entry.entryId] = entry
				}
			}

			// first entry in this channel. depth is already 0 and replaces is nil
			if len(aheadEntries) == 0 && len(behindEntries) == 0 {
				continue
			}

			// the new entry is not being added to the head, so address the entries in front of the new entry
			if len(aheadEntries) != 0 {
				// keep track of the deepest entry ahead of the new one.
				depth := 0
				// iterate over all of the entries ahead of the new one to update the replaces field where necessary
				for _, ahead := range aheadEntries {
					// if the graph already has the replacement chain defined (i.e. we are inserting into the middle of an existing chain)
					if ahead.replaces != 0 {
						// if there are any entries semantically higher than the new version AND they replace
						// a lower version, insert the new entry into that part of the graph and write
						if behind, isBehind := behindEntries[ahead.replaces]; isBehind {
							ahead.replaces = newEntry.entryId
							newEntry.replaces = behind.entryId // due to synthetic entries, it's possible this may happen more than once and the newEntry's replace field will select randomly

							// write the higher node's new replaces field
							if _, err := addReplaces.Exec(ahead.replaces, ahead.entryId); err != nil {
								errs = append(errs, err)
								continue
							}
						}
					} else { // if the older version didn't replace anything, assume that the new version necessarily can be replaced by it
						ahead.replaces = newEntry.entryId

						if _, err := addReplaces.Exec(ahead.replaces, ahead.entryId); err != nil {
							errs = append(errs, err)
							continue
						}
					}
					if depth < ahead.depth {
						depth = ahead.depth
					}
				}
				newEntry.depth = depth + 1
			} else {
				// set the channel head to the bundle we are adding
				if _, err := updateChannel.Exec(csvToUpdate, c.Name, manifest.PackageName); err != nil {
					errs = append(errs, err)
					continue
				}

				// insert/replace default channel. only trust when package is added to the front
				defaultChannelName := manifest.GetDefaultChannel()
				if defaultChannelName != "" {
					if _, err := updateDefaultChannel.Exec(defaultChannelName, manifest.PackageName); err != nil {
						errs = append(errs, err)
					}
				}
			}

			// the new entry is not being added to the tail, so address the entries behind the new entry
			if len(behindEntries) != 0 {
				tipId := 0
				tipVersion, _ := semver.Make("0")
				// iterate over all the entries below the new entry and push them down the graph
				for _, behind := range behindEntries {
					// don't increase the depth if the new version is just fanning out an existing one
					if !(updateMode == registry.SkipPatchMode && skipPatchFoundVersion) {
						behind.depth++

						if _, err := updateDepthById.Exec(behind.entryId); err != nil {
							errs = append(errs, err)
							continue
						}
					}

					// find highest version below new version
					if behind.version.GT(tipVersion) {
						tipId = behind.entryId
						tipVersion = behind.version
					}
				}

				if newEntry.replaces == 0 {
					newEntry.replaces = tipId
				}
			}

			// update the new entry to set the depth and replaces
			// set the new entry to one node deeper than the deepest ahead of it
			if _, err := updateDepthByIdToValue.Exec(newEntry.depth, newEntry.entryId); err != nil {
				errs = append(errs, err)
				continue
			}
			if newEntry.replaces != 0 {
				if _, err := addReplaces.Exec(newEntry.replaces, newEntry.entryId); err != nil {
					errs = append(errs, err)
					continue
				}
			}
		} else {
			// set the channel head to the bundle we are adding
			if _, err := updateChannel.Exec(c.CurrentCSVName, c.Name, manifest.PackageName); err != nil {
				errs = append(errs, err)
				continue
			}

			// don't need to check if version has been inserted for a given channel
			// because this is caught by primary key of operatorbundle table

			channelEntryCSV, err := s.getCSV(tx, c.CurrentCSVName)
			if err != nil {
				errs = append(errs, err)
				break
			}

			// check replaces
			replaces, err := channelEntryCSV.GetReplaces()
			if err != nil {
				errs = append(errs, err)
				break
			}

			// where does the replaces fall in the update graph
			rows, err := getDepth.Query(c.Name, manifest.PackageName, replaces)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			var depth int64
			var currentID int64
			var replacedIDs []int64
			skips, err := channelEntryCSV.GetSkips()
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if rows.Next() {
				err := rows.Scan(&depth, &currentID)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				// check if replaces not at the head of the channel
				if depth != 0 {
					// if not at the head of the channel, need to specify appropriate skips
					if len(skips) != int(depth) {
						errs = append(errs, fmt.Errorf("%s attempts to replace %s that is already replaced by another version", c.CurrentCSVName, replaces))
						return utilerrors.NewAggregate(errs)
					}
					skipmap := make(map[string]struct{}, 0)
					for _, sk := range skips {
						skipmap[sk] = struct{}{}
					}
					// get csv from depth to head for channel
					skipped, err := getBundleIDNameFromDepthToHead.Query(depth, c.Name, manifest.PackageName)
					if err != nil {
						errs = append(errs, err)
						continue
					}
					defer skipped.Close()

					// see if csvs match skips
					var skip string
					var replacedID int64
					for skipped.Next() {
						err := skipped.Scan(&replacedID, &skip)
						if err != nil {
							errs = append(errs, err)
							return utilerrors.NewAggregate(errs)
						}
						replacedIDs = append(replacedIDs, replacedID)
						if _, ok := skipmap[skip]; !ok {
							errs = append(errs, fmt.Errorf("%s attempts to replace %s that is already replaced by %s without specifying a skip", c.CurrentCSVName, replaces, skip))
						}
					}
					// aggregate all the errors instead of returning on first error
					if len(errs) > 0 {
						return utilerrors.NewAggregate(errs)
					}
				}
			} else {
				// specifies a replacement that is not in db
				errs = append(errs, fmt.Errorf("%s specifies a replacement %s that cannot be found", c.CurrentCSVName, replaces))
				return utilerrors.NewAggregate(errs)
			}

			if err := rows.Close(); err != nil {
				errs = append(errs, err)
				continue
			}

			// insert version into head of channel
			res, err := addChannelEntry.Exec(c.Name, manifest.PackageName, c.CurrentCSVName, 0)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			currentID, err = res.LastInsertId()
			if err != nil {
				errs = append(errs, err)
				continue
			}

			// update replacement to point to new head of channel
			var replacedID int64
			rows, err = getChannelEntryID.Query(c.Name, manifest.PackageName, replaces)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if rows.Next() {
				err := rows.Scan(&replacedID)
				if err != nil {
					errs = append(errs, err)
				}
			} // else is not possible by previous SELECT statement on replaces

			if err := rows.Close(); err != nil {
				errs = append(errs, err)
				continue
			}

			if _, err = addReplaces.Exec(replacedID, currentID); err != nil {
				errs = append(errs, err)
				continue
			}

			// remove skips from graph
			for _, skip := range skips {
				if _, err := removeSkipped.Exec(c.Name, manifest.PackageName, skip); err != nil {
					errs = append(errs, err)
					continue
				}
			}

			// add APIs
			if err := s.addAPIs(tx, channelEntryCSV, currentID); err != nil {
				errs = append(errs, err)
				continue
			}

			// update depth to depth + 1 for replaced entry
			_, err = updateDepth.Exec(c.Name, manifest.PackageName, replaces)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			// insert dummy skips entries if needed or update the graph based on skips
			depth = 1
			for _, skip := range skips {
				// add dummy channel entry for the skipped version
				skippedChannelEntry, err := addChannelEntry.Exec(c.Name, manifest.PackageName, skip, depth)
				if err != nil {
					errs = append(errs, err)
					continue
				}

				skippedID, err := skippedChannelEntry.LastInsertId()
				if err != nil {
					errs = append(errs, err)
					continue
				}

				// add another channel entry for the parent, which replaces the skipped
				synthesizedChannelEntry, err := addChannelEntry.Exec(c.Name, manifest.PackageName, c.CurrentCSVName, depth)
				if err != nil {
					errs = append(errs, err)
					continue
				}

				synthesizedID, err := synthesizedChannelEntry.LastInsertId()
				if err != nil {
					errs = append(errs, err)
					continue
				}

				if _, err = addReplaces.Exec(skippedID, synthesizedID); err != nil {
					errs = append(errs, err)
					continue
				}

				if err := s.addAPIs(tx, channelEntryCSV, synthesizedID); err != nil {
					errs = append(errs, err)
					continue
				}

				depth++
			}
		}
	}

	if errs != nil {
		return utilerrors.NewAggregate(errs)
	}
	return nil
}

type bundleChannelEntry struct {
	version  semver.Version
	entryId  int
	replaces int
	depth    int
}

func (s *SQLLoader) getBundlEntriesForChannel(tx *sql.Tx, channel, pkg string) (map[int]bundleChannelEntry, error) {
	getEntryIdsAndVersionsForChannel, err := tx.Prepare(`
		SELECT channel_entry.entry_id, channel_entry.replaces, channel_entry.depth, operatorbundle.version
		FROM operatorbundle JOIN channel_entry ON operatorbundle.name=channel_entry.operatorbundle_name 
		WHERE channel_entry.channel_name = ? AND channel_entry.package_name = ?`)
	if err != nil {
		return nil, err
	}
	defer getEntryIdsAndVersionsForChannel.Close()

	rows, err := getEntryIdsAndVersionsForChannel.Query(channel, pkg)
	if err != nil {
		return nil, err
	}

	var entryid, replaces, depth sql.NullInt32
	var version sql.NullString
	entries := make(map[int]bundleChannelEntry, 0)
	for rows.Next() {
		if err := rows.Scan(&entryid, &replaces, &depth, &version); err != nil {
			return nil, fmt.Errorf("Unable to get existing entry ids and versions for channel %s in package %s", channel, pkg)
		}
		entry := bundleChannelEntry{}
		if entryid.Valid && depth.Valid && version.Valid {
			entry.entryId = int(entryid.Int32)
			entry.depth = int(depth.Int32)

			parsedVersion, err := semver.Make(version.String)
			if err != nil {
				return nil, err
			}
			entry.version = parsedVersion
		} else {
			return nil, fmt.Errorf("Unable to get channel entries for the package %s's channel %s", pkg, channel)
		}
		if replaces.Valid {
			entry.replaces = int(replaces.Int32)
		}
		entries[entry.entryId] = entry
	}

	return entries, nil
}
