package action

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/model"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

type ReverseMigrate struct {
	CatalogRef string
	OutputFile string

	Registry image.Registry
}

func (rm ReverseMigrate) Run(ctx context.Context) error {
	r := Render{
		Refs:           []string{rm.CatalogRef},
		AllowedRefMask: RefDCDir | RefDCImage,
		Registry:       rm.Registry,

		skipSqliteDeprecationLog: true,
	}
	cfg, err := r.Run(ctx)
	if err != nil {
		return fmt.Errorf("render catalog image: %w", err)
	}

	m, err := declcfg.ConvertToModel(*cfg)
	if err != nil {
		return err
	}

	if _, err := os.Stat(rm.OutputFile); err == nil {
		return fmt.Errorf("cannot reverse-migrate into existing database")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	db, err := sqlite.Open(rm.OutputFile)
	if err != nil {
		return err
	}
	defer db.Close()

	migrator, err := sqlite.NewSQLLiteMigrator(db)
	if err != nil {
		return err
	}
	if migrator == nil {
		return fmt.Errorf("failed to load migrator")
	}

	if err := migrator.Migrate(ctx); err != nil {
		return err
	}

	channelEntryID := 0
	if _, err := db.Exec("PRAGMA defer_foreign_keys = true"); err != nil {
		return fmt.Errorf("defer foreign keys: %v", err)
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %v", err)
	}
	defer func() {
		tx.Rollback()
	}()

	addedBundles := map[string]*model.Bundle{}
	for _, pkg := range m {
		if _, err := tx.Exec("INSERT INTO package (name, default_channel, add_mode) VALUES (?, ?, ?)", pkg.Name, pkg.DefaultChannel.Name, "fbc"); err != nil {
			return fmt.Errorf("insert package for %q: %v", pkg.Name, err)
		}
		for _, ch := range pkg.Channels {
			head, err := ch.Head()
			if err != nil {
				return fmt.Errorf("get head of channel %q in package %q: %v", ch.Name, pkg.Name, err)
			}
			if _, err := tx.Exec("INSERT INTO channel (name, package_name, head_operatorbundle_name) VALUES (?, ?, ?)", ch.Name, pkg.Name, head.Name); err != nil {
				return fmt.Errorf("insert channel %q for package %q: %v", ch.Name, pkg.Name, err)
			}
			cur := head
			depth := 0
			for cur != nil {
				next := ch.Bundles[cur.Replaces]
				if next != nil {
					if _, err := tx.Exec("INSERT INTO channel_entry (entry_id, channel_name, package_name, operatorbundle_name, replaces, depth) VALUES (?, ?, ?, ?, ?, ?)", channelEntryID, ch.Name, pkg.Name, cur.Name, depth+1, depth); err != nil {
						return fmt.Errorf("insert replaces channel entry %q for package %q, channel %q: %v", cur.Name, pkg.Name, ch.Name, err)
					}
				} else if cur.Replaces != "" {
					if _, err := tx.Exec("INSERT INTO channel_entry (entry_id, channel_name, package_name, operatorbundle_name, replaces, depth) VALUES (?, ?, ?, ?, ?, ?)", channelEntryID, ch.Name, pkg.Name, cur.Name, depth+1, depth); err != nil {
						return fmt.Errorf("insert replaces channel entry %q for package %q, channel %q: %v", cur.Name, pkg.Name, ch.Name, err)
					}
					depth += 1
					channelEntryID += 1
					if _, err := tx.Exec("INSERT INTO channel_entry (entry_id, channel_name, package_name, operatorbundle_name, depth) VALUES (?, ?, ?, ?, ?)", channelEntryID, ch.Name, pkg.Name, cur.Replaces, depth); err != nil {
						return fmt.Errorf("insert tail channel entry %q for package %q, channel %q: %v", cur.Name, pkg.Name, ch.Name, err)
					}
				} else {
					if _, err := tx.Exec("INSERT INTO channel_entry (entry_id, channel_name, package_name, operatorbundle_name, depth) VALUES (?, ?, ?, ?, ?)", channelEntryID, ch.Name, pkg.Name, cur.Name, depth); err != nil {
						return fmt.Errorf("insert tail channel entry %q for package %q, channel %q: %v", cur.Name, pkg.Name, ch.Name, err)
					}
				}
				depth += 1
				channelEntryID += 1
				cur = next
			}

			cur = head
			for cur != nil {
				for _, skip := range cur.Skips {
					if _, err := tx.Exec("INSERT INTO channel_entry (entry_id, channel_name, package_name, operatorbundle_name, depth) VALUES (?, ?, ?, ?, ?)", channelEntryID, ch.Name, pkg.Name, skip, depth); err != nil {
						return fmt.Errorf("insert 'skip to' channel entry %q for package %q, channel %q: %v", skip, pkg.Name, ch.Name, err)
					}
					channelEntryID += 1
					if _, err := tx.Exec("INSERT INTO channel_entry (entry_id, channel_name, package_name, operatorbundle_name, replaces, depth) VALUES (?, ?, ?, ?, ?, ?)", channelEntryID, ch.Name, pkg.Name, cur.Name, channelEntryID-1, depth); err != nil {
						return fmt.Errorf("insert 'skip from' channel entry %q for package %q, channel %q: %v", cur.Name, pkg.Name, ch.Name, err)
					}
					channelEntryID += 1
				}
				cur = ch.Bundles[cur.Replaces]
			}

			for _, b := range ch.Bundles {
				if existing, ok := addedBundles[b.Name]; ok {
					if !equivalentReverseMigrateBundles(*existing, *b) {
						return fmt.Errorf("cannot produce sqlite-equivalent of FBC: found unsupported differences between channels for bundle %q", b.Name)
					}

					continue
				}
				addedBundles[b.Name] = b

				for _, ri := range b.RelatedImages {
					if _, err := tx.Exec("INSERT INTO related_image (image, operatorbundle_name) VALUES (?, ?)", ri.Image, b.Name); err != nil {
						return fmt.Errorf("insert related image %q for package %q, channel %q, bundle %q: %v", ri.Image, pkg.Name, ch.Name, b.Name, err)
					}
				}
				for _, p := range b.Properties {
					if p.Type == "olm.bundle.object" {
						continue
					}
					if _, err := tx.Exec("INSERT INTO properties (type, value, operatorbundle_name, operatorbundle_version, operatorbundle_path) VALUES (?, ?, ?, ?, ?)", p.Type, string(p.Value), b.Name, b.Version, b.Image); err != nil {
						return fmt.Errorf("insert property %q for package %q, channel %q, bundle %q: %v", p.Type, pkg.Name, ch.Name, b.Name, err)
					}
					if p.Type == "olm.deprecated" {
						if _, err := tx.Exec("INSERT INTO deprecated (operatorbundle_name) VALUES (?)", b.Name); err != nil {
							return fmt.Errorf("insert deprecation for package %q, channel %q, bundle %q: %v", pkg.Name, ch.Name, b.Name, err)
						}
					}
				}
				// TODO QUESTION: Do we need to ever insert a substitutesFor value?
				//    I think, no. FBC doesn't use the subsFor concept in its API.
				//    The expectation is that the FBC already contains all of the
				//    graph edges and changes to account for substitutions, and that
				//    any further substitutions will be performed via the FBC and
				//    then another reverse-migrate will occur to convert to sqlite.
				if _, err := tx.Exec("INSERT INTO operatorbundle (name, csv, bundle, bundlepath, skiprange, version, replaces, skips, substitutesfor) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)", b.Name, b.CsvJSON, strings.Join(b.Objects, "\n"), b.Image, b.SkipRange, b.Version, b.Replaces, strings.Join(b.Skips, ","), ""); err != nil {
					return fmt.Errorf("insert operatorbundle for package %q, channel %q, bundle %q: %v", pkg.Name, ch.Name, b.Name, err)
				}

				for _, gvk := range b.PropertiesP.GVKs {
					if _, err := tx.Exec("INSERT OR IGNORE INTO api (group_name, version, kind, plural) VALUES (?, ?, ?, ?)\n", gvk.Group, gvk.Version, gvk.Kind, ""); err != nil {
						return fmt.Errorf("insert api group: %q, version: %q, kind: %q: %v", gvk.Group, gvk.Version, gvk.Kind, err)
					}
					if _, err := tx.Exec("INSERT INTO api_provider (group_name, version, kind, operatorbundle_name, operatorbundle_version, operatorbundle_path) VALUES (?, ?, ?, ?, ?, ?)", gvk.Group, gvk.Version, gvk.Kind, b.Name, b.Version, b.Image); err != nil {
						return fmt.Errorf("insert api provider (group: %q, version: %q, kind: %q) for package %q, channel %q, bundle %q: %v", gvk.Group, gvk.Version, gvk.Kind, pkg.Name, ch.Name, b.Name, err)
					}
				}
				for _, gvk := range b.PropertiesP.GVKsRequired {
					if _, err := tx.Exec("INSERT OR IGNORE INTO api (group_name, version, kind, plural) VALUES (?, ?, ?, ?)\n", gvk.Group, gvk.Version, gvk.Kind, ""); err != nil {
						return fmt.Errorf("insert api group: %q, version: %q, kind: %q: %v", gvk.Group, gvk.Version, gvk.Kind, err)
					}
					if _, err := tx.Exec("INSERT INTO api_requirer (group_name, version, kind, operatorbundle_name, operatorbundle_version, operatorbundle_path) VALUES (?, ?, ?, ?, ?, ?)", gvk.Group, gvk.Version, gvk.Kind, b.Name, b.Version, b.Image); err != nil {
						return fmt.Errorf("insert api requirer (group: %q, version: %q, kind: %q) for package %q, channel %q, bundle %q: %v", gvk.Group, gvk.Version, gvk.Kind, pkg.Name, ch.Name, b.Name, err)
					}
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %v", err)
	}
	return nil
}

func equivalentReverseMigrateBundles(a, b model.Bundle) bool {
	a.Channel, b.Channel = nil, nil
	a.Replaces, b.Replaces = "", ""
	a.Skips, b.Skips = nil, nil
	return reflect.DeepEqual(a, b)
}
