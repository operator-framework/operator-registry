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
func (g *SQLGraphLoader) GraphFromEntries(channelEntries []registry.ChannelEntry) ([]registry.Channel, error) {
	var channels []registry.Channel
	var channelToBundle = make(map[string][]registry.OperatorBundle)

	for _, entry := range channelEntries {
		replace := registry.Replace{
			Version: "",
			Name:    entry.BundleName,
		}
		newBundle := registry.OperatorBundle{
			Version:    "",
			Name:       entry.BundleName,
			BundlePath: "",
			Replaces: []registry.Replace{replace},
		}

		if bundles, ok := channelToBundle[entry.ChannelName]; !ok {
			channelToBundle[entry.ChannelName] = []registry.OperatorBundle{newBundle}
		} else {
			// if newBundle is in the channel then append replaces to that newBundle
			// else insert newBundle
			bundle := getBundle(bundles, entry.BundleName)
			if bundle != nil {
				bundle.Replaces = append(bundle.Replaces, replace)
			} else {
				bundles = append(bundles, newBundle)
			}
		}
	}

	// TODO
	// 1
	// create channel struct and package struct that we are returning
	// create slice of channels one for each channel to bundle in the map
	// value in each channel is the value of the map

	// 2
	// write DB query to version number and bundle path for the bundles
	// use query to fill-in version value for the replaces we are creating

	// 3
	// fill in value of HEAD in the channel struct via query
	// write query to find head of channel 


	return channels, nil
}

func getBundle(bundles []registry.OperatorBundle, name string) *registry.OperatorBundle {
	for _, b := range bundles {
		if b.Name == name {
			return &b
		}
	}
	return nil
}