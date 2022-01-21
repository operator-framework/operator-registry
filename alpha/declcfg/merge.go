package declcfg

import (
	"fmt"
	"sort"

	"github.com/imdario/mergo"
)

// Merger is an object that will complete merge actions declarative config options
type Merger interface {
	Merge(*DeclarativeConfig) error
}

var _ Merger = &PreferLastStrategy{}

type PreferLastStrategy struct{}

func (mg *PreferLastStrategy) Merge(dc *DeclarativeConfig) error {
	return mergeDCPreferLast(dc)
}

// mergeDCPreferLast merges all packages, channels, and bundles with the same unique key
// into single objects using the last element with that key.
func mergeDCPreferLast(cfg *DeclarativeConfig) error {

	// Merge packages.
	pkgsByKey := make(map[string][]Package, len(cfg.Packages))
	for i, pkg := range cfg.Packages {
		key := keyForDCObj(pkg)
		pkgsByKey[key] = append(pkgsByKey[key], cfg.Packages[i])
	}
	if len(pkgsByKey) != 0 {
		outPkgs := make([]Package, len(pkgsByKey))
		i := 0
		for _, pkgs := range pkgsByKey {
			outPkgs[i] = pkgs[len(pkgs)-1]
			i++
		}
		sortPackages(outPkgs)
		cfg.Packages = outPkgs
	}

	// Merge channels.
	chsByKey := make(map[string][]Channel, len(cfg.Channels))
	for i, ch := range cfg.Channels {
		key := keyForDCObj(ch)
		chsByKey[key] = append(chsByKey[key], cfg.Channels[i])
	}
	if len(chsByKey) != 0 {
		outChs := make([]Channel, len(chsByKey))
		i := 0
		for _, chs := range chsByKey {
			outChs[i] = chs[len(chs)-1]
			i++
		}
		sortChannels(outChs)
		cfg.Channels = outChs
	}

	// Merge bundles.
	bundlesByKey := make(map[string][]Bundle, len(cfg.Bundles))
	for i, b := range cfg.Bundles {
		key := keyForDCObj(b)
		bundlesByKey[key] = append(bundlesByKey[key], cfg.Bundles[i])
	}
	if len(bundlesByKey) != 0 {
		outBundles := make([]Bundle, len(bundlesByKey))
		i := 0
		for _, bundles := range bundlesByKey {
			outBundles[i] = bundles[len(bundles)-1]
			i++
		}
		sortBundles(outBundles)
		cfg.Bundles = outBundles
	}

	// There is no way to merge "other" schema since a unique key field is unknown.
	return nil
}

var _ Merger = &TwoWayStrategy{}

type TwoWayStrategy struct{}

func (mg *TwoWayStrategy) Merge(dc *DeclarativeConfig) error {
	return mergeDCTwoWay(dc)
}

// mergeDCTwoWay merges all packages, channels, and bundles with the same unique key
// into single objects with ascending priority.
func mergeDCTwoWay(cfg *DeclarativeConfig) error {
	var err error
	if cfg.Packages, err = mergePackages(cfg.Packages); err != nil {
		return err
	}
	if cfg.Channels, err = mergeChannels(cfg.Channels); err != nil {
		return err
	}
	if cfg.Bundles, err = mergeBundles(cfg.Bundles); err != nil {
		return err
	}
	// There is no way to merge "other" schema since a unique key field is unknown.
	return nil
}

// mergePackages merges all packages with the same name into one package object.
// Value preference is ascending: values of packages later in input are preferred.
func mergePackages(inPkgs []Package) (outPkgs []Package, err error) {
	pkgsByName := make(map[string][]Package, len(inPkgs))
	for i, pkg := range inPkgs {
		key := keyForDCObj(pkg)
		pkgsByName[key] = append(pkgsByName[key], inPkgs[i])
	}

	for _, pkgs := range pkgsByName {
		mergedPkg := pkgs[0]

		if len(pkgs) > 1 {
			for _, pkg := range pkgs[1:] {
				if err := mergo.Merge(&mergedPkg, pkg, mergo.WithOverride); err != nil {
					return nil, err
				}
			}
		}

		outPkgs = append(outPkgs, mergedPkg)
	}

	sortPackages(outPkgs)

	return outPkgs, nil
}

// mergeChannels merges all channels with the same name and package into one channel object.
// Value preference is ascending: values of channels later in input are preferred.
func mergeChannels(inChs []Channel) (outChs []Channel, err error) {
	chsByKey := make(map[string][]Channel, len(inChs))
	entriesByKey := make(map[string]map[string][]ChannelEntry, len(inChs))
	for i, ch := range inChs {
		chKey := keyForDCObj(ch)
		chsByKey[chKey] = append(chsByKey[chKey], inChs[i])
		entries, ok := entriesByKey[chKey]
		if !ok {
			entries = make(map[string][]ChannelEntry)
			entriesByKey[chKey] = entries
		}
		for j, e := range ch.Entries {
			entries[e.Name] = append(entries[e.Name], ch.Entries[j])
		}
	}

	for chKey, chs := range chsByKey {
		mergedCh := chs[0]

		if len(chs) > 1 {
			for _, ch := range chs[1:] {
				if err := mergo.Merge(&mergedCh, ch, mergo.WithOverride); err != nil {
					return nil, err
				}
			}
		}

		mergedCh.Entries = nil
		for _, entries := range entriesByKey[chKey] {
			mergedEntry := entries[0]

			if len(entries) > 1 {
				for _, e := range entries[1:] {
					if err := mergo.Merge(&mergedEntry, e, mergo.WithOverride); err != nil {
						return nil, err
					}
				}
			}

			mergedCh.Entries = append(mergedCh.Entries, mergedEntry)
		}

		sort.Slice(mergedCh.Entries, func(i, j int) bool {
			return mergedCh.Entries[i].Name < mergedCh.Entries[j].Name
		})

		outChs = append(outChs, mergedCh)
	}

	sortChannels(outChs)

	return outChs, nil
}

// mergeBundles merges all bundles with the same name and package into one bundle object.
// Value preference is ascending: values of bundles later in input are preferred.
func mergeBundles(inBundles []Bundle) (outBundles []Bundle, err error) {
	bundlesByKey := make(map[string][]Bundle, len(inBundles))
	for i, bundle := range inBundles {
		key := keyForDCObj(bundle)
		bundlesByKey[key] = append(bundlesByKey[key], inBundles[i])
	}

	for _, bundles := range bundlesByKey {
		mergedBundle := bundles[0]

		if len(bundles) > 1 {
			for _, bundle := range bundles[1:] {
				if err := mergo.Merge(&mergedBundle, bundle, mergo.WithOverride); err != nil {
					return nil, err
				}
			}
		}

		outBundles = append(outBundles, mergedBundle)
	}

	sortBundles(outBundles)

	return outBundles, nil
}

func sortPackages(pkgs []Package) {
	sort.Slice(pkgs, func(i, j int) bool {
		return pkgs[i].Name < pkgs[j].Name
	})
}

func sortChannels(chs []Channel) {
	sort.Slice(chs, func(i, j int) bool {
		if chs[i].Package == chs[j].Package {
			return chs[i].Name < chs[j].Name
		}
		return chs[i].Package < chs[j].Package
	})
}

func sortBundles(bundles []Bundle) {
	sort.Slice(bundles, func(i, j int) bool {
		if bundles[i].Package == bundles[j].Package {
			return bundles[i].Name < bundles[j].Name
		}
		return bundles[i].Package < bundles[j].Package
	})
}

func keyForDCObj(obj interface{}) string {
	switch t := obj.(type) {
	case Package:
		// Package name is globally unique.
		return t.Name
	case Channel:
		// Channel name is unqiue per package.
		return t.Package + t.Name
	case Bundle:
		// Bundle name is unqiue per package.
		return t.Package + t.Name
	default:
		// This should never happen.
		panic(fmt.Sprintf("bug: unrecognized type %T, expected one of Package, Channel, Bundle", t))
	}
}
