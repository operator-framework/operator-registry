package e2e_test

import (
	"io/ioutil"
	"os"
	"sort"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/operator-framework/operator-registry/internal/declcfg"
	"github.com/operator-framework/operator-registry/pkg/action"
	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry"
	"github.com/operator-framework/operator-registry/pkg/lib/bundle"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/rand"
)

var (
	pkg       = "prometheus"
	chs       = "alpha,stable"
	defaultCh = "alpha"

	path1         = "manifests/prometheus/0.14.0"
	img           = dockerHost + "/olmtest/e2e-bundle"
	tag1          = rand.String(6)
	containerTool = "docker"
)
var _ = Describe("opm", func() {
	var (
		configsDir string
		reg        *containerdregistry.Registry
	)
	BeforeEach(func() {
		tmpDir, err := ioutil.TempDir("", "configs")
		Expect(err).ToNot(HaveOccurred())

		configsDir = tmpDir
		bundles := bundleLocations{
			{img + ":" + tag1, path1},
		}
		for _, b := range bundles {
			err = inTemporaryBuildContext(func() error {
				return bundle.BuildFunc(b.path, "", b.image, containerTool, pkg, chs, defaultCh, false)
			})
			Expect(err).NotTo(HaveOccurred())
		}
		for _, b := range bundles {
			Expect(pushWith(containerTool, b.image)).NotTo(HaveOccurred())
		}

		reg, err = containerdregistry.NewRegistry()
		Expect(err).ToNot(HaveOccurred())
	})
	AfterEach(func() {
		err := os.RemoveAll(configsDir)
		Expect(err).ToNot(HaveOccurred())

		err = reg.Destroy()
		Expect(err).ToNot(HaveOccurred())
	})
	It("creates configs for bundles and adds them to a configs directory", func() {
		When("a new bundle is added that does not belong to an existing package is added", func() {

			request := action.AddConfigRequest{
				ConfigsDir: configsDir,
				Bundles:    []action.BundleExtractor{action.NewImageBundleExtractor(img+":"+tag1, reg, logrus.NewEntry(logrus.New()))},
			}

			adder := action.NewBundleAdder(logrus.NewEntry(logrus.New()))
			adder.AddToConfig(request)

			expectedConfigs, err := declcfg.LoadDir("testdata/test-configs")
			Expect(err).ToNot(HaveOccurred())

			actualConfigs, err := declcfg.LoadDir(configsDir)
			Expect(err).ToNot(HaveOccurred())

			equalsDeclarativeConfig(expectedConfigs, actualConfigs)
			Expect(err).ToNot(HaveOccurred())

			Expect(err).ToNot(HaveOccurred())
		})
	})

})

func equalsDeclarativeConfig(expected, actual *declcfg.DeclarativeConfig) {
	removeJSONWhitespace(expected)
	removeJSONWhitespace(actual)

	assert.ElementsMatch(GinkgoT(), expected.Packages, actual.Packages)
	assert.ElementsMatch(GinkgoT(), expected.Others, actual.Others)
	require.Equal(GinkgoT(), len(expected.Bundles), len(actual.Bundles))
	sort.SliceStable(expected.Bundles, func(i, j int) bool {
		return expected.Bundles[i].Name < expected.Bundles[j].Name
	})
	sort.SliceStable(actual.Bundles, func(i, j int) bool {
		return actual.Bundles[i].Name < actual.Bundles[j].Name
	})
	for i := range expected.Bundles {
		assert.ElementsMatch(GinkgoT(), expected.Bundles[i].Properties, actual.Bundles[i].Properties)
		expected.Bundles[i].Properties, actual.Bundles[i].Properties = nil, nil
		expected.Bundles[i].Image, actual.Bundles[i].Image = "", ""
		assert.Equal(GinkgoT(), expected.Bundles[i], actual.Bundles[i])
	}
	// In case new fields are added to the DeclarativeConfig struct in the future,
	// test that the rest is Equal.
	expected.Packages, actual.Packages = nil, nil
	expected.Bundles, actual.Bundles = nil, nil
	expected.Others, actual.Others = nil, nil
	assert.Equal(GinkgoT(), expected, actual)
}

func removeJSONWhitespace(cfg *declcfg.DeclarativeConfig) {
	replacer := strings.NewReplacer(" ", "", "\n", "")
	for ib := range cfg.Bundles {
		for ip := range cfg.Bundles[ib].Properties {
			cfg.Bundles[ib].Properties[ip].Value = []byte(replacer.Replace(string(cfg.Bundles[ib].Properties[ip].Value)))
		}
	}
	for io := range cfg.Others {
		for _, v := range cfg.Others[io].Blob {
			cfg.Others[io].Blob = []byte(replacer.Replace(string(v)))
		}
	}
}
