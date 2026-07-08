package declcfg

import (
	"fmt"
	"sort"
	"strings"

	"github.com/blang/semver/v4"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/operator-framework/operator-registry/alpha/property"
)

// Validate validates a DeclarativeConfig, collecting all errors into a
// hierarchical tree that reports every issue found (not just the first).
// The error tree follows the structure: invalid index → invalid package → errors.
func Validate(cfg DeclarativeConfig) error {
	result := newValidationError("invalid index")

	packageNames := sets.New[string]()
	defaultChannels := map[string]string{}
	pkgErrors := map[string]*validationError{}

	// pkgResult returns (or creates) a per-package validationError node.
	pkgResult := func(pkg string) *validationError {
		if _, ok := pkgErrors[pkg]; !ok {
			pkgErrors[pkg] = newValidationError(fmt.Sprintf("invalid package %q", pkg))
		}
		return pkgErrors[pkg]
	}

	// Validate packages
	for _, p := range cfg.Packages {
		if p.Name == "" {
			result.subErrors = append(result.subErrors, fmt.Errorf("config contains package with no name"))
			continue
		}

		if packageNames.Has(p.Name) {
			result.subErrors = append(result.subErrors, fmt.Errorf("duplicate package %q", p.Name))
			continue
		}
		packageNames.Insert(p.Name)

		if errs := validation.IsDNS1123Label(p.Name); len(errs) > 0 {
			result.subErrors = append(result.subErrors, fmt.Errorf("invalid package name %q: %v", p.Name, errs))
		}

		defaultChannels[p.Name] = p.DefaultChannel
	}

	// Validate channels
	packageChannels := make(map[string]sets.Set[string])
	channelDefinedEntries := map[string]sets.Set[string]{}
	channelsByPackage := map[string][]Channel{}

	for _, c := range cfg.Channels {
		if !packageNames.Has(c.Package) {
			result.subErrors = append(result.subErrors, fmt.Errorf("unknown package %q for channel %q", c.Package, c.Name))
			continue
		}

		if c.Name == "" {
			pkgResult(c.Package).subErrors = append(pkgResult(c.Package).subErrors, fmt.Errorf("package contains channel with no name"))
			continue
		}

		if _, ok := packageChannels[c.Package]; !ok {
			packageChannels[c.Package] = sets.New[string]()
		}
		if packageChannels[c.Package].Has(c.Name) {
			pkgResult(c.Package).subErrors = append(pkgResult(c.Package).subErrors, fmt.Errorf("duplicate channel %q", c.Name))
			continue
		}
		packageChannels[c.Package].Insert(c.Name)

		// Track entries defined in channel
		cde := sets.Set[string]{}
		seenEntries := sets.New[string]()
		for _, entry := range c.Entries {
			if seenEntries.Has(entry.Name) {
				pkgResult(c.Package).subErrors = append(pkgResult(c.Package).subErrors, fmt.Errorf("channel %q: duplicate entry %q", c.Name, entry.Name))
			}
			seenEntries.Insert(entry.Name)
			cde = cde.Insert(entry.Name)
		}
		channelDefinedEntries[c.Package] = channelDefinedEntries[c.Package].Union(cde)
		channelsByPackage[c.Package] = append(channelsByPackage[c.Package], c)
	}

	// Validate channel graphs
	for pkg, channels := range channelsByPackage {
		for _, ch := range channels {
			if err := validateChannelGraph(ch); err != nil {
				pkgResult(pkg).subErrors = append(pkgResult(pkg).subErrors, err)
			}
		}
	}

	// Validate bundles
	packageBundles := map[string]sets.Set[string]{}
	packageBundleVersions := map[string]map[string][]string{} // pkg -> version -> []bundleName

	for _, b := range cfg.Bundles {
		bundleResult := newValidationError(fmt.Sprintf("invalid bundle %q", b.Name))

		if b.Package == "" {
			bundleResult.subErrors = append(bundleResult.subErrors, fmt.Errorf("package name must be set"))
		}
		if b.Package != "" && !packageNames.Has(b.Package) {
			bundleResult.subErrors = append(bundleResult.subErrors, fmt.Errorf("unknown package %q", b.Package))
		}

		bundles, ok := packageBundles[b.Package]
		if !ok {
			bundles = sets.Set[string]{}
		}
		if bundles.Has(b.Name) {
			bundleResult.subErrors = append(bundleResult.subErrors, fmt.Errorf("duplicate bundle"))
		}
		bundles.Insert(b.Name)
		packageBundles[b.Package] = bundles

		props, err := property.Parse(b.Properties)
		if err != nil {
			bundleResult.subErrors = append(bundleResult.subErrors, fmt.Errorf("parse properties: %v", err))
		}

		if props != nil && len(props.Packages) != 1 {
			bundleResult.subErrors = append(bundleResult.subErrors, fmt.Errorf("must have exactly 1 %q property, found %d", property.TypePackage, len(props.Packages)))
		}

		if props != nil && len(props.Packages) == 1 {
			if b.Package != props.Packages[0].PackageName {
				bundleResult.subErrors = append(bundleResult.subErrors, fmt.Errorf("package %q does not match %q property %q", b.Package, property.TypePackage, props.Packages[0].PackageName))
			}

			// Validate version
			rawVersion := props.Packages[0].Version
			if _, err := semver.Parse(rawVersion); err != nil {
				bundleResult.subErrors = append(bundleResult.subErrors, fmt.Errorf("error parsing version %q: %v", rawVersion, err))
			}
		}

		if err := validateImagePullSpec(b.Image, "image"); err != nil {
			bundleResult.subErrors = append(bundleResult.subErrors, err)
		}
		for i, rel := range b.RelatedImages {
			if err := validateImagePullSpec(rel.Image, "relatedImages[%d].image", i); err != nil {
				bundleResult.subErrors = append(bundleResult.subErrors, err)
			}
		}

		if b.Image == "" && len(b.Objects) == 0 {
			bundleResult.subErrors = append(bundleResult.subErrors, fmt.Errorf("bundle image must be set"))
		}

		// Validate bundle version/release and name normalization
		vr, vrErr := b.VersionRelease()
		if vrErr == nil && vr != nil {
			if normalizedName := normalizeBundleName(b.Package, vr); normalizedName != "" && b.Name != normalizedName {
				bundleResult.subErrors = append(bundleResult.subErrors, fmt.Errorf("name %q does not match normalized name %q", b.Name, normalizedName))
			}
			if len(vr.Version.Build) > 0 && len(vr.Release) > 0 {
				bundleResult.subErrors = append(bundleResult.subErrors, fmt.Errorf("cannot use build metadata in version with a release version"))
			}

			// Track version for unique version check
			versionStr := versionString(vr)
			if packageBundleVersions[b.Package] == nil {
				packageBundleVersions[b.Package] = map[string][]string{}
			}
			packageBundleVersions[b.Package][versionStr] = append(packageBundleVersions[b.Package][versionStr], b.Name)
		}

		channelDefinedEntries[b.Package] = channelDefinedEntries[b.Package].Delete(b.Name)

		// Validate skip entries for channel entries referencing this bundle
		for _, ch := range cfg.Channels {
			if ch.Package != b.Package {
				continue
			}
			for _, entry := range ch.Entries {
				if entry.Name != b.Name {
					continue
				}
				for i, skip := range entry.Skips {
					if skip == "" {
						bundleResult.subErrors = append(bundleResult.subErrors, fmt.Errorf("skip[%d] is empty in channel %q", i, ch.Name))
					}
				}
				if entry.SkipRange != "" {
					if _, err := semver.ParseRange(entry.SkipRange); err != nil {
						bundleResult.subErrors = append(bundleResult.subErrors, fmt.Errorf("invalid skipRange %q in channel %q: %v", entry.SkipRange, ch.Name, err))
					}
				}
			}
		}

		// Check that bundle is in at least one channel
		found := false
		for _, ch := range cfg.Channels {
			if ch.Package != b.Package {
				continue
			}
			for _, entry := range ch.Entries {
				if entry.Name == b.Name {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			bundleResult.subErrors = append(bundleResult.subErrors, fmt.Errorf("not found in any channel entries"))
		}

		if verr := bundleResult.orNil(); verr != nil {
			if b.Package != "" && packageNames.Has(b.Package) {
				pkgResult(b.Package).subErrors = append(pkgResult(b.Package).subErrors, verr)
			} else {
				result.subErrors = append(result.subErrors, verr)
			}
		}
	}

	// Check for channel entries without bundles
	for pkg, entries := range channelDefinedEntries {
		if entries.Len() > 0 {
			pkgResult(pkg).subErrors = append(pkgResult(pkg).subErrors, fmt.Errorf("no olm.bundle blobs found in package %q for olm.channel entries %s", pkg, sets.List[string](entries)))
		}
	}

	// Validate each package has at least one channel
	for pkg := range packageNames {
		if _, ok := packageChannels[pkg]; !ok {
			pkgResult(pkg).subErrors = append(pkgResult(pkg).subErrors, fmt.Errorf("package must contain at least one channel"))
		}
	}

	// Validate default channels
	for pkg, defaultChannel := range defaultChannels {
		if defaultChannel == "" {
			if packageChannels[pkg] != nil && packageChannels[pkg].Len() > 0 {
				pkgResult(pkg).subErrors = append(pkgResult(pkg).subErrors, fmt.Errorf("default channel must be set"))
			}
		} else if packageChannels[pkg] != nil && !packageChannels[pkg].Has(defaultChannel) {
			pkgResult(pkg).subErrors = append(pkgResult(pkg).subErrors, fmt.Errorf("default channel %q not found in channels list", defaultChannel))
		}
	}

	// Validate unique bundle versions per package
	for pkg, versions := range packageBundleVersions {
		var dupes []string
		versionKeys := make([]string, 0, len(versions))
		for v := range versions {
			versionKeys = append(versionKeys, v)
		}
		sort.Strings(versionKeys)
		for _, v := range versionKeys {
			bundles := versions[v]
			if len(bundles) > 1 {
				sort.Strings(bundles)
				dupes = append(dupes, fmt.Sprintf("{%s: [%s]}", v, strings.Join(bundles, ", ")))
			}
		}
		if len(dupes) > 0 {
			pkgResult(pkg).subErrors = append(pkgResult(pkg).subErrors, fmt.Errorf("duplicate versions found in bundles: %v", dupes))
		}
	}

	// Validate deprecations
	deprecationsByPackage := sets.New[string]()
	for i, deprecation := range cfg.Deprecations {
		if deprecation.Package == "" {
			result.subErrors = append(result.subErrors, fmt.Errorf("package name must be set for deprecation item %v", i))
			continue
		}

		if !packageNames.Has(deprecation.Package) {
			result.subErrors = append(result.subErrors, fmt.Errorf("cannot apply deprecations to an unknown package %q", deprecation.Package))
			continue
		}

		if deprecationsByPackage.Has(deprecation.Package) {
			result.subErrors = append(result.subErrors, fmt.Errorf("expected a maximum of one deprecation per package: %q", deprecation.Package))
			continue
		}
		deprecationsByPackage.Insert(deprecation.Package)

		references := sets.New[PackageScopedReference]()
		for j, entry := range deprecation.Entries {
			if entry.Reference.Schema == "" {
				pkgResult(deprecation.Package).subErrors = append(pkgResult(deprecation.Package).subErrors, fmt.Errorf("schema must be set for deprecation entry [%v]", j))
				continue
			}

			if entry.Message == "" {
				pkgResult(deprecation.Package).subErrors = append(pkgResult(deprecation.Package).subErrors, fmt.Errorf("deprecation entry [%v] must have a message", j))
			}

			if references.Has(entry.Reference) {
				pkgResult(deprecation.Package).subErrors = append(pkgResult(deprecation.Package).subErrors, fmt.Errorf("duplicate deprecation entry %#v", entry.Reference))
				continue
			}
			references.Insert(entry.Reference)

			switch entry.Reference.Schema {
			case SchemaBundle:
				if !packageBundles[deprecation.Package].Has(entry.Reference.Name) {
					pkgResult(deprecation.Package).subErrors = append(pkgResult(deprecation.Package).subErrors, fmt.Errorf("cannot deprecate bundle %q: bundle not found", entry.Reference.Name))
				}
			case SchemaChannel:
				if !packageChannels[deprecation.Package].Has(entry.Reference.Name) {
					pkgResult(deprecation.Package).subErrors = append(pkgResult(deprecation.Package).subErrors, fmt.Errorf("cannot deprecate channel %q: channel not found", entry.Reference.Name))
				}
			case SchemaPackage:
				if entry.Reference.Name != "" {
					pkgResult(deprecation.Package).subErrors = append(pkgResult(deprecation.Package).subErrors, fmt.Errorf("package name must be empty for deprecated package (specified %q)", entry.Reference.Name))
				}
			default:
				pkgResult(deprecation.Package).subErrors = append(pkgResult(deprecation.Package).subErrors, fmt.Errorf("cannot deprecate object %#v referenced by entry %v: object schema unknown", entry.Reference, j))
			}
		}
	}

	// Flush per-package errors into the result tree, sorted by package name for deterministic output
	pkgNames := make([]string, 0, len(pkgErrors))
	for pkg := range pkgErrors {
		pkgNames = append(pkgNames, pkg)
	}
	sort.Strings(pkgNames)
	for _, pkg := range pkgNames {
		if verr := pkgErrors[pkg].orNil(); verr != nil {
			result.subErrors = append(result.subErrors, verr)
		}
	}

	return result.orNil()
}

// validateChannelGraph validates the upgrade graph structure of a channel.
// It checks for:
//  1. Exactly one channel head (entry with no incoming edges from replaces/skips).
//  2. No cycles in the replaces chain.
//  3. All non-skipped entries are reachable from the head via the replaces chain.
func validateChannelGraph(ch Channel) error {
	if len(ch.Entries) == 0 {
		chResult := newValidationError(fmt.Sprintf("invalid channel %q", ch.Name))
		chResult.subErrors = append(chResult.subErrors, fmt.Errorf("channel must contain at least one bundle"))
		return chResult.orNil()
	}

	chResult := newValidationError(fmt.Sprintf("invalid channel %q", ch.Name))

	entries := map[string]ChannelEntry{}
	for _, e := range ch.Entries {
		entries[e.Name] = e
	}

	allEntries := sets.New[string]()
	for _, e := range ch.Entries {
		allEntries.Insert(e.Name)
	}

	incomingEdges := sets.New[string]()
	skippedEntries := sets.New[string]()
	for _, e := range ch.Entries {
		skippedEntries.Insert(e.Skips...)
		if e.Replaces != "" && allEntries.Has(e.Replaces) {
			incomingEdges.Insert(e.Replaces)
		}
		for _, skip := range e.Skips {
			if allEntries.Has(skip) {
				incomingEdges.Insert(skip)
			}
		}
	}

	// Find head(s): entries with no incoming edges
	heads := []string{}
	for _, e := range ch.Entries {
		if !incomingEdges.Has(e.Name) {
			heads = append(heads, e.Name)
		}
	}

	if len(heads) == 0 {
		chResult.subErrors = append(chResult.subErrors, fmt.Errorf("no channel head found in graph"))
		return chResult.orNil()
	}
	if len(heads) > 1 {
		sort.Strings(heads)
		chResult.subErrors = append(chResult.subErrors, fmt.Errorf("multiple channel heads found in graph: %s", strings.Join(heads, ", ")))
		return chResult.orNil()
	}

	// Walk the replaces chain from head, checking for cycles
	head := heads[0]
	chainFrom := map[string][]string{}
	replacesChainFromHead := sets.New[string](head)
	cur, ok := entries[head]
	for ok {
		if _, exists := chainFrom[cur.Name]; !exists {
			chainFrom[cur.Name] = []string{cur.Name}
		}
		// if the replaces edge is known to be skipped, disregard it
		if skippedEntries.Has(cur.Replaces) {
			break
		}
		for k := range chainFrom {
			chainFrom[k] = append(chainFrom[k], cur.Replaces)
		}
		if replacesChainFromHead.Has(cur.Replaces) {
			chResult.subErrors = append(chResult.subErrors, fmt.Errorf("detected cycle in replaces chain of upgrade graph: %s", strings.Join(chainFrom[cur.Replaces], " -> ")))
			return chResult.orNil()
		}
		replacesChainFromHead.Insert(cur.Replaces)
		cur, ok = entries[cur.Replaces]
	}

	strandedEntries := allEntries.Difference(replacesChainFromHead).Difference(skippedEntries)
	if strandedEntries.Len() > 0 {
		stranded := sets.List[string](strandedEntries)
		chResult.subErrors = append(chResult.subErrors, fmt.Errorf("channel contains one or more stranded bundles: %s", strings.Join(stranded, ", ")))
	}

	return chResult.orNil()
}

func normalizeBundleName(packageName string, vr *VersionRelease) string {
	if len(vr.Release) > 0 {
		return fmt.Sprintf("%s-v%s-%s", packageName, vr.Version.String(), vr.Release.String())
	}
	// if no release, normalization does not apply; return empty to signal "skip check"
	return ""
}

func versionString(vr *VersionRelease) string {
	if len(vr.Release) > 0 {
		return fmt.Sprintf("%s-%s", vr.Version.String(), vr.Release.String())
	}
	return vr.Version.String()
}
