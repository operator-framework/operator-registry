package sqlite

import (
	"context"
	"github.com/operator-framework/operator-registry/pkg/registry"
)

// GraphLoader generates a graph
// GraphLoader supports multiple different loading schemes
// GraphLoader from SQL, GraphLoader from old format (filesystem), GraphLoader from SQL + input bundles
type GraphLoader interface {
	Generate() (*registry.Package, error)
}

type SQLGraphLoader struct {
	Querier     *SQLQuerier
	PackageName string
}

// TODO
type ChannelEntryNode struct {
	PackageName        string
	ChannelName        string
	BundleName         string
	BundlePath         string
	Version            string
	Replaces           string
	ReplacesVersion    string
	ReplacesBundlePath string
}

func NewSQLGraphLoader(dbFilename, name string) (*SQLGraphLoader, error) {
	querier, err := NewSQLLiteQuerier(dbFilename)
	if err != nil {
		return nil, err
	}

	return &SQLGraphLoader{
		Querier:     querier,
		PackageName: name,
	}, nil
}

func (g *SQLGraphLoader) Generate() (*registry.Package, error) {
	ctx := context.TODO()
	defaultChannel, err := g.Querier.GetDefaultPackage(ctx, g.PackageName)
	if err != nil {
		return nil, err
	}

	channelEntries, err := g.Querier.GetChannelEntriesFromPackage(ctx, defaultChannel)
	if err != nil {
		return nil, err
	}

	channels, err := g.GraphFromEntries(channelEntries)
	if err != nil {
		return nil, err
	}

	return &registry.Package{
		Name:           g.PackageName,
		DefaultChannel: defaultChannel,
		Channels:       channels,
	}, nil
}

// TODO get bundle path and version

// GraphFromEntries builds the graph from a set of channel entries
func (g *SQLGraphLoader) GraphFromEntries(channelEntries []ChannelEntryNode) ([]registry.Channel, error) {
	var channels []registry.Channel
	var channelToBundles = make(map[string][]registry.OperatorBundle)

	for _, entry := range channelEntries {
		replaces := registry.BundleRef{
			BundlePath: entry.BundlePath,
			Version:    entry.ReplacesVersion,
			CsvName:    entry.Replaces,
		}
		newBundle := registry.OperatorBundle{
			Version:         entry.Version,
			CsvName:         entry.BundleName,
			BundlePath:      entry.BundlePath,
			ReplacesBundles: []registry.OperatorBundle{},
			Replaces:        []registry.BundleRef{replaces},
		}

		if bundles, ok := channelToBundles[entry.ChannelName]; !ok {
			channelToBundles[entry.ChannelName] = []registry.OperatorBundle{newBundle}
		} else {
			// if newBundle is in the channel then append replaces to that newBundle
			// else insert newBundle
			bundle := getBundle(bundles, entry.BundleName)
			if bundle != nil {
				bundle.Replaces = append(bundle.Replaces, replaces)
			} else {
				bundles = append(bundles, newBundle)
			}
		}
	}

	// bundleref to operatorbundle
	for _, bundles := range channelToBundles {
		for _, bundle := range bundles {
			for _, ref := range bundle.Replaces {
				replacesBundle := getBundle(bundles, ref.CsvName)
				if replacesBundle != nil {
					bundle.ReplacesBundles = append(bundle.ReplacesBundles, *replacesBundle)
				}
			}
		}
	}

	for chName, bundles := range channelToBundles {
		head := getHeadBundleRefForChannel(bundles)

		channel := registry.Channel{
			Name:            chName,
			OperatorBundles: bundles,
			Head:            *head,
		}

		channels = append(channels, channel)
	}

	return channels, nil
}

func getBundle(bundles []registry.OperatorBundle, name string) *registry.OperatorBundle {
	for _, b := range bundles {
		if b.CsvName == name {
			return &b
		}
	}
	return nil
}

func getHeadBundleRefForChannel(bundles []registry.OperatorBundle) *registry.BundleRef {
	b, bundles := bundles[0], bundles[1:]
	candidate := registry.BundleRef{
		CsvName:    b.CsvName,
		BundlePath: b.BundlePath,
		Version:    b.Version,
	}
	for _, b := range bundles {
		for _, ref := range b.Replaces {
			if ref.CsvName == candidate.CsvName {
				candidate = registry.BundleRef{
					CsvName:    b.CsvName,
					BundlePath: b.BundlePath,
					Version:    b.Version,
				}
			}
		}
	}

	return &candidate
}
