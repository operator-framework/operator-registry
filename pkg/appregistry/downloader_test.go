package appregistry

import (
	"errors"
	"testing"

	"github.com/operator-framework/operator-registry/pkg/apprclient"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes/fake"
)

var testPrepare = []struct {
	input                 *Input
	sourceQuerier         sourceQuerier
	expectedDownloadItems []*downloadItem
	expectedError         utilerrors.Aggregate
}{
	{
		input: &Input{
			Sources: []*Source{
				{
					Endpoint:          "quay.io",
					RegistryNamespace: "",
				},
			},
			Packages: []*Package{
				&Package{"Kubevirt", ""},
				&Package{"etcd", ""},
			},
		},
		sourceQuerier: &fakeSourceQuerier{
			map[Source][]*apprclient.RegistryMetadata{
				Source{
					Endpoint:          "quay.io",
					RegistryNamespace: "",
				}: {
					&apprclient.RegistryMetadata{Name: "Kubevirt"},
				},
			},
			map[Source]error{},
		},
		expectedDownloadItems: []*downloadItem{
			&downloadItem{
				RepositoryMetadata: &apprclient.RegistryMetadata{Name: "Kubevirt"},
				Source:             &Source{Endpoint: "quay.io", RegistryNamespace: ""},
				Release:            "",
			},
		},
		expectedError: nil,
	},
	{
		input: &Input{
			Sources: []*Source{
				{
					Endpoint:          "quay.io",
					RegistryNamespace: "",
				},
				{
					Endpoint:          "other-endpoint.io",
					RegistryNamespace: "",
				},
			},
			Packages: []*Package{
				&Package{"Kubevirt", ""},
				&Package{"etcd", ""},
			},
		},
		sourceQuerier: &fakeSourceQuerier{
			map[Source][]*apprclient.RegistryMetadata{
				Source{
					Endpoint:          "quay.io",
					RegistryNamespace: "",
				}: {
					&apprclient.RegistryMetadata{Name: "Kubevirt"},
				},
			},
			map[Source]error{
				Source{
					Endpoint:          "other-endpoint.io",
					RegistryNamespace: "",
				}: errors.New("Failed to fetch sources from other-endpoint.io"),
			},
		},
		expectedDownloadItems: []*downloadItem{
			&downloadItem{
				RepositoryMetadata: &apprclient.RegistryMetadata{Name: "Kubevirt"},
				Source:             &Source{Endpoint: "quay.io", RegistryNamespace: ""},
				Release:            "",
			},
		},
		expectedError: utilerrors.NewAggregate(
			[]error{
				errors.New("Failed to fetch sources from other-endpoint.io"),
			},
		),
	},
	{
		input: &Input{
			Sources: []*Source{
				{
					Endpoint:          "quay.io",
					RegistryNamespace: "",
				},
			},
			Packages: []*Package{
				&Package{"Kubevirt", "10.0.0"},
				&Package{"etcd", ""},
			},
		},
		sourceQuerier: &fakeSourceQuerier{
			map[Source][]*apprclient.RegistryMetadata{
				Source{
					Endpoint:          "quay.io",
					RegistryNamespace: "",
				}: {
					&apprclient.RegistryMetadata{
						Name:     "Kubevirt",
						Release:  "11.0.0",
						Releases: []string{"10.0.0", "11.0.0"},
					},
					&apprclient.RegistryMetadata{
						Name:     "etcd",
						Release:  "2.0.0",
						Releases: []string{"1.0.0", "2.0.0"},
					},
				},
			},
			map[Source]error{},
		},
		expectedDownloadItems: []*downloadItem{
			&downloadItem{
				RepositoryMetadata: &apprclient.RegistryMetadata{
					Name:     "Kubevirt",
					Release:  "11.0.0",
					Releases: []string{"10.0.0", "11.0.0"},
				},
				Source:  &Source{Endpoint: "quay.io", RegistryNamespace: ""},
				Release: "10.0.0",
			},
			&downloadItem{
				RepositoryMetadata: &apprclient.RegistryMetadata{
					Name:     "etcd",
					Release:  "2.0.0",
					Releases: []string{"1.0.0", "2.0.0"},
				},
				Source:  &Source{Endpoint: "quay.io", RegistryNamespace: ""},
				Release: "2.0.0",
			},
		},
		expectedError: nil,
	},
	{
		input: &Input{
			Sources: []*Source{
				{
					Endpoint:          "quay.io",
					RegistryNamespace: "",
				},
			},
			Packages: []*Package{
				&Package{"Kubevirt", "10.0.0"},
			},
		},
		sourceQuerier: &fakeSourceQuerier{
			map[Source][]*apprclient.RegistryMetadata{
				Source{
					Endpoint:          "quay.io",
					RegistryNamespace: "",
				}: {
					&apprclient.RegistryMetadata{
						Name:     "Kubevirt",
						Release:  "11.0.0",
						Releases: []string{"11.0.0"},
					},
				},
			},
			map[Source]error{},
		},
		expectedDownloadItems: []*downloadItem{},
		expectedError:         nil,
	},
}

type fakeSourceQuerier struct {
	mapping    map[Source][]*apprclient.RegistryMetadata
	errMapping map[Source]error
}

func (f *fakeSourceQuerier) QuerySource(source *Source) (repositories []*apprclient.RegistryMetadata, err error) {
	err, ok := f.errMapping[*source]
	if ok {
		return nil, err
	}
	return f.mapping[*source], nil
}

func TestPrepare(t *testing.T) {
	logger := logrus.WithField("test", "prepare")
	clientset := fake.NewSimpleClientset()

	for _, testItem := range testPrepare {
		d := downloader{
			logger,
			clientset,
			testItem.sourceQuerier,
			nil,
		}

		downloadItems, err := d.Prepare(testItem.input)
		// Since downloadItems are stored in a map inside of the Prepare
		// function the order isn't guaranteed
		assert.ElementsMatch(t, testItem.expectedDownloadItems, downloadItems)
		if testItem.expectedError != nil {
			assert.Equal(t, testItem.expectedError, err)
		} else {
			assert.NoError(t, err)
		}
	}
}
