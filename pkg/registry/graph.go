package registry

import (
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/goccy/go-graphviz/cgraph"
	"k8s.io/apimachinery/pkg/util/sets"
)

type Package struct {
	Name           string
	DefaultChannel string
	Channels       map[string]Channel
}

type Channel struct {
	Head  BundleKey
	Nodes map[BundleKey]map[BundleKey]struct{}
}

type BundleKey struct {
	BundlePath string
	Version    string //semver string
	CsvName    string
}

func (b *BundleKey) IsEmpty() bool {
	return b.BundlePath == "" && b.Version == "" && b.CsvName == ""
}

func (b *BundleKey) String() string {
	return fmt.Sprintf("%s %s %s", b.CsvName, b.Version, b.BundlePath)
}

func (p *Package) HasChannel(channel string) bool {
	if p.Channels == nil {
		return false
	}

	_, found := p.Channels[channel]
	return found
}

func (p *Package) HasCsv(csv string) bool {
	for _, channelGraph := range p.Channels {
		for node := range channelGraph.Nodes {
			if node.CsvName == csv {
				return true
			}
		}
	}

	return false
}

type GraphPackage struct {
	PackageName  string
	ChannelHeads []BundleKey
	Nodes        map[BundleKey]*GraphBundle
	Edges        map[BundleKey]map[BundleKey]*GraphUpgradeEdge
}

type GraphBundle struct {
	CsvName       string
	Version       string
	BundlePath    string
	Channels      sets.String
	SkipRange     semver.Range
	IsChannelHead bool
	IsVirtual     bool
}

type GraphEdgeType string

const (
	Replaces  GraphEdgeType = "replaces"
	Skips     GraphEdgeType = "skips"
	SkipRange GraphEdgeType = "skipRange"
)

type GraphUpgradeEdge struct {
	EdgeType GraphEdgeType
}

func (b GraphBundle) Key() BundleKey {
	return BundleKey{
		BundlePath: b.BundlePath,
		Version:    b.Version,
		CsvName:    b.CsvName,
	}
}

func NewGraphPackageFromEntries(packageName string, entries []ChannelEntryAnnotated) (*GraphPackage, error) {
	var err error

	pkg := &GraphPackage{
		PackageName: packageName,
		Nodes:       make(map[BundleKey]*GraphBundle),
		Edges:       make(map[BundleKey]map[BundleKey]*GraphUpgradeEdge),
	}

	// Add all nodes
	for _, e := range entries {
		entryBundle := &GraphBundle{
			CsvName:       e.BundleName,
			Version:       e.Version,
			BundlePath:    e.BundlePath,
			Channels:      sets.NewString(e.ChannelName),
			SkipRange:     func(semver.Version) bool { return false },
			IsChannelHead: e.Depth == 0,
			IsVirtual:     e.BundlePath == "",
		}
		if e.SkipRange != "" {
			entryBundle.SkipRange, err = semver.ParseRange(e.SkipRange)
			if err != nil {
				return nil, fmt.Errorf("failed to parse skipRange %q for bundle %q", e.SkipRange, e.BundleName)
			}
		}
		key := entryBundle.Key()
		node, ok := pkg.Nodes[key]
		if !ok {
			pkg.Nodes[key] = entryBundle
			continue
		}
		node.Channels = node.Channels.Union(entryBundle.Channels)
		node.IsChannelHead = node.IsChannelHead || entryBundle.IsChannelHead
	}

	// Add all replaces and skips edges
	for _, e := range entries {
		fromKey := BundleKey{
			BundlePath: e.BundlePath,
			Version:    e.Version,
			CsvName:    e.BundleName,
		}
		toKey := BundleKey{
			BundlePath: e.ReplacesBundlePath,
			Version:    e.ReplacesVersion,
			CsvName:    e.Replaces,
		}
		if toKey.CsvName == "" {
			continue
		}
		if _, ok := pkg.Edges[fromKey]; !ok {
			pkg.Edges[fromKey] = make(map[BundleKey]*GraphUpgradeEdge)
		}

		skips := sets.NewString(strings.Split(e.Skips, ",")...)
		if e.BundleReplaces == e.Replaces {
			pkg.Edges[fromKey][toKey] = &GraphUpgradeEdge{EdgeType: Replaces}
		} else if skips.Has(e.Replaces) {
			pkg.Edges[fromKey][toKey] = &GraphUpgradeEdge{EdgeType: Skips}
		} else {
			return nil, fmt.Errorf("found entry with no skips or replaces")
		}
	}

	// Add skipRange edges
	for _, from := range pkg.Nodes {
		if from.IsChannelHead {
			pkg.ChannelHeads = append(pkg.ChannelHeads, from.Key())
		}
		fromKey := from.Key()
		for _, to := range pkg.Nodes {
			toKey := to.Key()
			if to.Version == "" {
				continue
			}
			toVersion, err := semver.Parse(to.Version)
			if err != nil {
				return nil, fmt.Errorf("failed to parse version %q for bundle %q", to.Version, to.CsvName)
			}

			if from.SkipRange(toVersion) {
				if _, ok := pkg.Edges[fromKey]; !ok {
					pkg.Edges[fromKey] = make(map[BundleKey]*GraphUpgradeEdge)
				}
				if e, ok := pkg.Edges[fromKey][toKey]; !ok || e.EdgeType == Skips {
					pkg.Edges[fromKey][toKey] = &GraphUpgradeEdge{EdgeType: SkipRange}
				}
			}
		}
	}
	return pkg, nil
}

func (pkg GraphPackage) AddToGraph(graph *cgraph.Graph) error {
	pGraph := graph.SubGraph(fmt.Sprintf("cluster_%s", pkg.PackageName), 1)
	pGraph.SetLabel(fmt.Sprintf("package: %s", pkg.PackageName))

	for _, n := range pkg.Nodes {
		nodeName := fmt.Sprintf("%s_%s", pkg.PackageName, n.CsvName)
		node, err := pGraph.CreateNode(nodeName)
		if err != nil {
			return err
		}
		node.SetShape("record")
		node.SetWidth(4)
		node.SetLabel(fmt.Sprintf("{%s|{channels|{%s}}}", n.CsvName, strings.Join(n.Channels.List(), "|")))
		if n.IsVirtual {
			node.SetStyle(cgraph.DashedNodeStyle)
		}
		if n.IsChannelHead {
			node.SetPenWidth(3.0)
		}
	}

	for fromKey, edges := range pkg.Edges {
		for toKey, edge := range edges {
			from := pkg.Nodes[fromKey]
			fromName := fmt.Sprintf("%s_%s", pkg.PackageName, from.CsvName)
			fromNode, err := pGraph.Node(fromName)
			if err != nil {
				return nil
			}

			to := pkg.Nodes[toKey]
			toName := fmt.Sprintf("%s_%s", pkg.PackageName, to.CsvName)
			toNode, err := pGraph.Node(toName)
			if err != nil {
				return nil
			}

			edgeName := fmt.Sprintf("%s_%s_%s", edge.EdgeType, fromNode.Name(), toNode.Name())
			gEdge, err := pGraph.CreateEdge(edgeName, fromNode, toNode)
			if err != nil {
				return err
			}
			gEdge.SetFontSize(9.0)
			switch edge.EdgeType {
			case Replaces:
				gEdge.SetLabel("replaces")
				gEdge.SetStyle(cgraph.BoldEdgeStyle)
			case Skips:
				gEdge.SetLabel("skips")
				gEdge.SetStyle(cgraph.DottedEdgeStyle)
			case SkipRange:
				gEdge.SetLabel("skipRange")
				gEdge.SetStyle(cgraph.DashedEdgeStyle)
			}
		}
	}
	return nil
}
