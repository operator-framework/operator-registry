package model

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/h2non/filetype/types"
	svg "github.com/h2non/go-is-svg"
	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/operator-framework/operator-registry/alpha/property"
)

type Deprecation struct {
	Message string `json:"message"`
}

func init() {
	t := types.NewType("svg", "image/svg+xml")
	filetype.AddMatcher(t, svg.Is)
	matchers.Image[types.NewType("svg", "image/svg+xml")] = svg.Is
}

type Model map[string]*Package

func (m Model) Validate() error {
	result := newValidationError("invalid index")

	for name, pkg := range m {
		if name != pkg.Name {
			result.subErrors = append(result.subErrors, fmt.Errorf("package key %q does not match package name %q", name, pkg.Name))
		}
		if err := pkg.Validate(); err != nil {
			result.subErrors = append(result.subErrors, err)
		}
	}
	return result.orNil()
}

type Package struct {
	Name           string
	Description    string
	Icon           *Icon
	DefaultChannel *Channel
	Channels       map[string]*Channel
	Deprecation    *Deprecation
}

func (m *Package) Validate() error {
	result := newValidationError(fmt.Sprintf("invalid package %q", m.Name))

	if m.Name == "" {
		result.subErrors = append(result.subErrors, errors.New("package name must not be empty"))
	}

	if err := m.Icon.Validate(); err != nil {
		result.subErrors = append(result.subErrors, err)
	}

	if m.DefaultChannel == nil {
		result.subErrors = append(result.subErrors, fmt.Errorf("default channel must be set"))
	}

	if len(m.Channels) == 0 {
		result.subErrors = append(result.subErrors, fmt.Errorf("package must contain at least one channel"))
	}

	foundDefault := false
	for name, ch := range m.Channels {
		if name != ch.Name {
			result.subErrors = append(result.subErrors, fmt.Errorf("channel key %q does not match channel name %q", name, ch.Name))
		}
		if err := ch.Validate(); err != nil {
			result.subErrors = append(result.subErrors, err)
		}
		if ch == m.DefaultChannel {
			foundDefault = true
		}
		if ch.Package != m {
			result.subErrors = append(result.subErrors, fmt.Errorf("channel %q not correctly linked to parent package", ch.Name))
		}
	}

	if err := m.validateUniqueBundleVersions(); err != nil {
		result.subErrors = append(result.subErrors, err)
	}

	if m.DefaultChannel != nil && !foundDefault {
		result.subErrors = append(result.subErrors, fmt.Errorf("default channel %q not found in channels list", m.DefaultChannel.Name))
	}

	if err := m.Deprecation.Validate(); err != nil {
		result.subErrors = append(result.subErrors, fmt.Errorf("invalid deprecation: %v", err))
	}

	return result.orNil()
}

func (m *Package) validateUniqueBundleVersions() error {
	versionsMap := map[string]semver.Version{}
	bundlesWithVersion := map[string]sets.Set[string]{}
	for _, ch := range m.Channels {
		for _, b := range ch.Bundles {
			versionsMap[b.Version.String()] = b.Version
			if bundlesWithVersion[b.Version.String()] == nil {
				bundlesWithVersion[b.Version.String()] = sets.New[string]()
			}
			bundlesWithVersion[b.Version.String()].Insert(b.Name)
		}
	}

	versionsSlice := maps.Values(versionsMap)
	semver.Sort(versionsSlice)

	var errs []error
	for _, v := range versionsSlice {
		bundles := sets.List(bundlesWithVersion[v.String()])
		if len(bundles) > 1 {
			errs = append(errs, fmt.Errorf("{%s: [%s]}", v, strings.Join(bundles, ", ")))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("duplicate versions found in bundles: %v", errs)
	}
	return nil
}

type Icon struct {
	Data      []byte `json:"base64data"`
	MediaType string `json:"mediatype"`
}

func (i *Icon) Validate() error {
	if i == nil {
		return nil
	}
	// TODO(joelanford): Should we check that data and mediatype are set,
	//   and detect the media type of the data and compare it to the
	//   mediatype listed in the icon field? Currently, some production
	//   index databases are failing these tests, so leaving this
	//   commented out for now.
	result := newValidationError("invalid icon")
	//if len(i.Data) == 0 {
	//	result.subErrors = append(result.subErrors, errors.New("icon data must be set if icon is defined"))
	//}
	//if len(i.MediaType) == 0 {
	//	result.subErrors = append(result.subErrors, errors.New("icon mediatype must be set if icon is defined"))
	//}
	//if len(i.Data) > 0 {
	//	if err := i.validateData(); err != nil {
	//		result.subErrors = append(result.subErrors, err)
	//	}
	//}
	return result.orNil()
}

// nolint:unused
func (i *Icon) validateData() error {
	if !filetype.IsImage(i.Data) {
		return errors.New("icon data is not an image")
	}
	t, err := filetype.Match(i.Data)
	if err != nil {
		return err
	}
	if t.MIME.Value != i.MediaType {
		return fmt.Errorf("icon media type %q does not match detected media type %q", i.MediaType, t.MIME.Value)
	}
	return nil
}

type Channel struct {
	Package     *Package
	Name        string
	Bundles     map[string]*Bundle
	Deprecation *Deprecation
	// NOTICE: The field Properties of the type Channel is for internal use only.
	//   DO NOT use it for any public-facing functionalities.
	//   This API is in alpha stage and it is subject to change.
	Properties []property.Property
}

// TODO(joelanford): This function determines the channel head by finding the bundle that has 0
//
//	incoming edges, based on replaces and skips. It also expects to find exactly one such bundle.
//	Is this the correct algorithm?
func (c Channel) Head() (*Bundle, error) {
	incoming := map[string]int{}
	for _, b := range c.Bundles {
		if b.Replaces != "" {
			incoming[b.Replaces]++
		}
		for _, skip := range b.Skips {
			incoming[skip]++
		}
	}
	var heads []*Bundle
	for _, b := range c.Bundles {
		if _, ok := incoming[b.Name]; !ok {
			heads = append(heads, b)
		}
	}
	if len(heads) == 0 {
		return nil, fmt.Errorf("no channel head found in graph")
	}
	if len(heads) > 1 {
		var headNames []string
		for _, head := range heads {
			headNames = append(headNames, head.Name)
		}
		sort.Strings(headNames)
		return nil, fmt.Errorf("multiple channel heads found in graph: %s", strings.Join(headNames, ", "))
	}
	return heads[0], nil
}

func (c *Channel) Validate() error {
	result := newValidationError(fmt.Sprintf("invalid channel %q", c.Name))

	if c.Name == "" {
		result.subErrors = append(result.subErrors, errors.New("channel name must not be empty"))
	}

	if c.Package == nil {
		result.subErrors = append(result.subErrors, errors.New("package must be set"))
	}

	if len(c.Bundles) == 0 {
		result.subErrors = append(result.subErrors, fmt.Errorf("channel must contain at least one bundle"))
	}

	if len(c.Bundles) > 0 {
		if err := c.validateReplacesChain(); err != nil {
			result.subErrors = append(result.subErrors, err)
		}
	}

	for name, b := range c.Bundles {
		if name != b.Name {
			result.subErrors = append(result.subErrors, fmt.Errorf("bundle key %q does not match bundle name %q", name, b.Name))
		}
		if err := b.Validate(); err != nil {
			result.subErrors = append(result.subErrors, err)
		}
		if b.Channel != c {
			result.subErrors = append(result.subErrors, fmt.Errorf("bundle %q not correctly linked to parent channel", b.Name))
		}
	}

	if err := c.Deprecation.Validate(); err != nil {
		result.subErrors = append(result.subErrors, fmt.Errorf("invalid deprecation: %v", err))
	}

	return result.orNil()
}

type node struct {
	name       string
	version    semver.Version
	replacedBy map[string]*node
	replaces   *node
	skippedBy  map[string]*node
	skips      map[string]*node
	skipRange  string
	hasEntry   bool
}

type graph struct {
	nodes map[string]*node
}

func newGraph(c *Channel) *graph {
	nodes := map[string]*node{}
	for _, b := range c.Bundles {
		nodes[b.Name] = &node{
			name:       b.Name,
			version:    b.Version,
			skipRange:  b.SkipRange,
			replacedBy: make(map[string]*node),
			skippedBy:  make(map[string]*node),
			skips:      make(map[string]*node),
		}
	}

	for _, b := range c.Bundles {
		n := nodes[b.Name]

		if b.Replaces != "" {
			replaces, ok := nodes[b.Replaces]
			if !ok {
				replaces = &node{
					name:       b.Replaces,
					replacedBy: make(map[string]*node),
					hasEntry:   false,
				}
				nodes[b.Replaces] = replaces
			}
			n.replaces = replaces
			n.replaces.replacedBy[n.name] = n
		}

		for _, skipName := range b.Skips {
			skip, ok := nodes[skipName]
			if !ok {
				skip = &node{
					name:      skipName,
					skippedBy: make(map[string]*node),
					skips:     make(map[string]*node),
					hasEntry:  false,
				}
			}
			skip.skippedBy[b.Name] = n
			n.skips[skipName] = skip
		}
	}

	return &graph{
		nodes: nodes,
	}
}

func (g *graph) validate() error {
	result := newValidationError("invalid upgrade graph")
	if err := g.validateNoCycles(); err != nil {
		result.subErrors = append(result.subErrors, err)
	}
	if err := g.validateNoStranded(); err != nil {
		result.subErrors = append(result.subErrors, err)
	}
	return result.orNil()
}

func (g *graph) validateNoCycles() error {
	result := newValidationError("cycles found in graph")
	allCycles := [][]*node{}
	for _, n := range g.nodes {
		ancestors := map[string]*node{}
		maps.Copy(ancestors, n.replacedBy)
		maps.Copy(ancestors, n.skippedBy)
		allCycles = append(allCycles, paths([]*node{n}, ancestors, n)...)
	}
	dedupPaths(&allCycles)
	for _, cycle := range allCycles {
		cycleStr := strings.Join(mapSlice(cycle, nodeName), " -> ")
		result.subErrors = append(result.subErrors, errors.New(cycleStr))
	}

	return result.orNil()
}

func (g *graph) validateNoStranded() error {
	head, err := g.head()
	if err != nil {
		return err
	}
	all := sets.New[*node](maps.Values(g.nodes)...)
	chain := sets.New[*node]()
	skipped := sets.New[*node]()

	cur := head
	for cur != nil && !skipped.Has(cur) && !chain.Has(cur) {
		chain.Insert(cur)
		skipped.Insert(maps.Values(cur.skips)...)
		cur = cur.replaces
	}

	stranded := all.Difference(chain).Difference(skipped)
	if stranded.Len() > 0 {
		strandedNames := mapSlice(stranded.UnsortedList(), func(n *node) string {
			return n.name
		})
		slices.Sort(strandedNames)
		return fmt.Errorf("channel contains one or more stranded bundles: %s", strings.Join(strandedNames, ", "))
	}

	return nil
}

func (g *graph) head() (*node, error) {
	heads := []*node{}
	for _, n := range g.nodes {
		if len(n.replacedBy) == 0 && len(n.skippedBy) == 0 {
			heads = append(heads, n)
		}
	}
	if len(heads) == 0 {
		return nil, fmt.Errorf("no channel head found in graph")
	}
	if len(heads) > 1 {
		var headNames []string
		for _, head := range heads {
			headNames = append(headNames, head.name)
		}
		sort.Strings(headNames)
		return nil, fmt.Errorf("multiple channel heads found in graph: %s", strings.Join(headNames, ", "))
	}
	return heads[0], nil
}

func nodeName(n *node) string {
	return n.name
}

func mapSlice[I, O any](s []I, fn func(I) O) []O {
	result := make([]O, 0, len(s))
	for _, i := range s {
		result = append(result, fn(i))
	}
	return result
}

func paths(existingPath []*node, froms map[string]*node, to *node) [][]*node {
	if len(froms) == 0 {
		// we never found a path to "to"
		return nil
	}
	var allPaths [][]*node
	for _, f := range froms {
		path := append(slices.Clone(existingPath), f)
		if f == to {
			// we found "to"!
			allPaths = append(allPaths, path)
		} else {
			allPaths = append(allPaths, paths(path, f.replacedBy, to)...)
		}
	}
	return allPaths
}

// dedupPaths removes rotations of the same cycle.
// For example there are three paths:
//  1. a -> b -> c -> a
//  2. b -> c -> a -> b
//  3. c -> a -> b -> c
//
// These are all the same cycle, so we want to choose just one of them.
// dedupPaths chooses to keep the one whose first node has the highest version.
func dedupPaths(paths *[][]*node) {
	slices.SortFunc(*paths, func(a, b []*node) int {
		if v := cmp.Compare(len(a), len(b)); v != 0 {
			return v
		}
		return b[0].version.Compare(a[0].version)
	})
	deleteIndices := sets.New[int]()
	for i, path := range *paths {
		for j, other := range (*paths)[i+1:] {
			if isSameRotation(path, other) {
				deleteIndices.Insert(j + i + 1)
			}
		}
	}

	toDelete := sets.List(deleteIndices)
	slices.Reverse(toDelete)
	for _, i := range toDelete {
		(*paths) = slices.Delete(*paths, i, i+1)
	}
}

func isSameRotation(a, b []*node) bool {
	if len(a) != len(b) {
		return false
	}
	aStr := strings.Join(mapSlice(a[:len(a)-1], nodeName), " -> ")
	bStr := strings.Join(mapSlice(b[:len(b)-1], nodeName), " -> ")
	aPlusA := aStr + " -> " + aStr
	return strings.Contains(aPlusA, bStr)
}

// validateReplacesChain checks the replaces chain of a channel.
// Specifically the following rules must be followed:
//  1. There must be exactly 1 channel head.
//  2. Beginning at the head, the replaces chain traversal must reach all entries.
//     Unreached entries are considered "stranded" and cause a channel to be invalid.
//  3. Skipped entries are always leaf nodes. We never follow replaces or skips edges
//     of skipped entries during replaces chain traversal.
//  4. There must be no cycles in the replaces chain.
//  5. The tail entry in the replaces chain is permitted to replace a non-existent entry.
func (c *Channel) validateReplacesChain() error {
	g := newGraph(c)
	return g.validate()
}

type Bundle struct {
	Package       *Package
	Channel       *Channel
	Name          string
	Image         string
	Replaces      string
	Skips         []string
	SkipRange     string
	Properties    []property.Property
	RelatedImages []RelatedImage
	Deprecation   *Deprecation

	// These fields are present so that we can continue serving
	// the GRPC API the way packageserver expects us to in a
	// backwards-compatible way.
	Objects []string
	CsvJSON string

	// These fields are used to compare bundles in a diff.
	PropertiesP *property.Properties
	Version     semver.Version
}

func (b *Bundle) Validate() error {
	result := newValidationError(fmt.Sprintf("invalid bundle %q", b.Name))

	if b.Name == "" {
		result.subErrors = append(result.subErrors, errors.New("name must be set"))
	}
	if b.Channel == nil {
		result.subErrors = append(result.subErrors, errors.New("channel must be set"))
	}
	if b.Package == nil {
		result.subErrors = append(result.subErrors, errors.New("package must be set"))
	}
	if b.Channel != nil && b.Package != nil && b.Package != b.Channel.Package {
		result.subErrors = append(result.subErrors, errors.New("package does not match channel's package"))
	}
	props, err := property.Parse(b.Properties)
	if err != nil {
		result.subErrors = append(result.subErrors, err)
	}
	for i, skip := range b.Skips {
		if skip == "" {
			result.subErrors = append(result.subErrors, fmt.Errorf("skip[%d] is empty", i))
		}
	}
	// TODO(joelanford): Validate related images? It looks like some
	//   CSVs in production databases use incorrect fields ([name,value]
	//   instead of [name,image]), which results in empty image values.
	//   Example is in redhat-operators: 3scale-operator.v0.5.5
	//for i, relatedImage := range b.RelatedImages {
	//	if err := relatedImage.Validate(); err != nil {
	//		result.subErrors = append(result.subErrors, WithIndex(i, err))
	//	}
	//}

	if props != nil && len(props.Packages) != 1 {
		result.subErrors = append(result.subErrors, fmt.Errorf("must be exactly one property with type %q", property.TypePackage))
	}

	if b.Image == "" && len(b.Objects) == 0 {
		result.subErrors = append(result.subErrors, errors.New("bundle image must be set"))
	}

	if err := b.Deprecation.Validate(); err != nil {
		result.subErrors = append(result.subErrors, fmt.Errorf("invalid deprecation: %v", err))
	}

	return result.orNil()
}

type RelatedImage struct {
	Name  string
	Image string
}

func (i RelatedImage) Validate() error {
	result := newValidationError("invalid related image")
	if i.Image == "" {
		result.subErrors = append(result.subErrors, fmt.Errorf("image must be set"))
	}
	return result.orNil()
}

func (m Model) Normalize() {
	for _, pkg := range m {
		for _, ch := range pkg.Channels {
			for _, b := range ch.Bundles {
				for i := range b.Properties {
					// Ensure property value is encoded in a standard way.
					if normalized, err := property.Build(&b.Properties[i]); err == nil {
						b.Properties[i] = *normalized
					}
				}
			}
		}
	}
}

func (m Model) AddBundle(b Bundle) {
	if _, present := m[b.Package.Name]; !present {
		m[b.Package.Name] = b.Package
	}
	p := m[b.Package.Name]
	b.Package = p

	if ch, ok := p.Channels[b.Channel.Name]; ok {
		b.Channel = ch
		ch.Bundles[b.Name] = &b
	} else {
		newCh := &Channel{
			Name:    b.Channel.Name,
			Package: p,
			Bundles: make(map[string]*Bundle),
		}
		b.Channel = newCh
		newCh.Bundles[b.Name] = &b
		p.Channels[newCh.Name] = newCh
	}

	if p.DefaultChannel == nil {
		p.DefaultChannel = b.Channel
	}
}

func (d *Deprecation) Validate() error {
	if d == nil {
		return nil
	}
	if d.Message == "" {
		return errors.New("message must be set")
	}
	return nil
}
