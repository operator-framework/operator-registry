package model

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/internal/property"
)

type validator interface {
	Validate() error
}

const svgData = `PHN2ZyB2aWV3Qm94PTAgMCAxMDAgMTAwPjxjaXJjbGUgY3g9MjUgY3k9MjUgcj0yNS8+PC9zdmc+`
const pngData = `iVBORw0KGgoAAAANSUhEUgAAAAEAAAABAQMAAAAl21bKAAAAA1BMVEUAAACnej3aAAAAAXRSTlMAQObYZgAAAApJREFUCNdjYAAAAAIAAeIhvDMAAAAASUVORK5CYII=`
const jpegData = `/9j/4AAQSkZJRgABAQEAYABgAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/2wBDAQkJCQwLDBgNDRgyIRwhMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjL/wAARCAABAAEDASIAAhEBAxEB/8QAHwAAAQUBAQEBAQEAAAAAAAAAAAECAwQFBgcICQoL/8QAtRAAAgEDAwIEAwUFBAQAAAF9AQIDAAQRBRIhMUEGE1FhByJxFDKBkaEII0KxwRVS0fAkM2JyggkKFhcYGRolJicoKSo0NTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqDhIWGh4iJipKTlJWWl5iZmqKjpKWmp6ipqrKztLW2t7i5usLDxMXGx8jJytLT1NXW19jZ2uHi4+Tl5ufo6erx8vP09fb3+Pn6/8QAHwEAAwEBAQEBAQEBAQAAAAAAAAECAwQFBgcICQoL/8QAtREAAgECBAQDBAcFBAQAAQJ3AAECAxEEBSExBhJBUQdhcRMiMoEIFEKRobHBCSMzUvAVYnLRChYkNOEl8RcYGRomJygpKjU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6goOEhYaHiImKkpOUlZaXmJmaoqOkpaanqKmqsrO0tba3uLm6wsPExcbHyMnK0tPU1dbX2Nna4uPk5ebn6Onq8vP09fb3+Pn6/9oADAMBAAIRAxEAPwD3+iiigD//2Q==`

func mustBase64Decode(in string) []byte {
	out, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		panic(err)
	}
	return out
}

func TestNormalize(t *testing.T) {
	b := &Bundle{}
	pkgs := Model{
		"anakin": {
			Channels: map[string]*Channel{
				"alpha": {
					Bundles: map[string]*Bundle{
						"anakin.v0.0.1": b,
					},
				},
			},
		},
	}
	t.Run("Success/IgnoreInvalid", func(t *testing.T) {
		invalidJSON := json.RawMessage(`}`)
		b.Properties = []property.Property{{Value: invalidJSON}}
		pkgs.Normalize()
		assert.Equal(t, invalidJSON, b.Properties[0].Value)
	})

	t.Run("Success/Unchanged", func(t *testing.T) {
		unchanged := json.RawMessage(`{}`)
		b.Properties = []property.Property{{Value: unchanged}}
		pkgs.Normalize()
		assert.Equal(t, unchanged, b.Properties[0].Value)
	})

	t.Run("Success/RemoveSpacesAndHTMLEscapes", func(t *testing.T) {
		withWhitespace := json.RawMessage("{\n\"test\":\"\u003C\"   }")
		expected := json.RawMessage(`{"test":"<"}`)
		b.Properties = []property.Property{{Value: withWhitespace}}
		pkgs.Normalize()
		assert.Equal(t, expected, b.Properties[0].Value)
	})
}

func TestChannelHead(t *testing.T) {
	type spec struct {
		name      string
		ch        Channel
		head      *Bundle
		assertion require.ErrorAssertionFunc
	}

	head := &Bundle{
		Name:     "anakin.v0.0.3",
		Replaces: "anakin.v0.0.1",
		Skips:    []string{"anakin.v0.0.2"},
	}

	specs := []spec{
		{
			name: "Success/Valid",
			ch: Channel{Bundles: map[string]*Bundle{
				"anakin.v0.0.1": {Name: "anakin.v0.0.1"},
				"anakin.v0.0.2": {Name: "anakin.v0.0.2"},
				"anakin.v0.0.3": head,
			}},
			head:      head,
			assertion: require.NoError,
		},
		{
			name: "Error/NoChannelHead",
			ch: Channel{Bundles: map[string]*Bundle{
				"anakin.v0.0.1": {Name: "anakin.v0.0.1", Replaces: "anakin.v0.0.3"},
				"anakin.v0.0.3": head,
			}},
			assertion: require.Error,
		},
		{
			name: "Error/MultipleChannelHeads",
			ch: Channel{Bundles: map[string]*Bundle{
				"anakin.v0.0.1": {Name: "anakin.v0.0.1"},
				"anakin.v0.0.3": head,
				"anakin.v0.0.4": {Name: "anakin.v0.0.4", Replaces: "anakin.v0.0.1"},
			}},
			assertion: require.Error,
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			h, err := s.ch.Head()
			assert.Equal(t, s.head, h)
			s.assertion(t, err)
		})
	}
}

func TestValidators(t *testing.T) {
	type spec struct {
		name      string
		v         validator
		assertion require.ErrorAssertionFunc
	}

	pkg, ch := makePackageChannelBundle()
	pkgIncorrectDefaultChannel, _ := makePackageChannelBundle()
	pkgIncorrectDefaultChannel.DefaultChannel = &Channel{Name: "not-found"}

	var nilIcon *Icon = nil

	specs := []spec{
		{
			name: "Model/Success/Valid",
			v: Model{
				pkg.Name: pkg,
			},
			assertion: require.NoError,
		},
		{
			name: "Model/Error/PackageKeyNameMismatch",
			v: Model{
				"foo": pkg,
			},
			assertion: require.Error,
		},
		{
			name: "Model/Error/InvalidPackage",
			v: Model{
				pkgIncorrectDefaultChannel.Name: pkgIncorrectDefaultChannel,
			},
			assertion: require.Error,
		},
		{
			name:      "Package/Success/Valid",
			v:         pkg,
			assertion: require.NoError,
		},
		{
			name:      "Package/Error/NoName",
			v:         &Package{},
			assertion: require.Error,
		},
		{
			name: "Package/Error/InvalidIcon",
			v: &Package{
				Name: "anakin",
				Icon: &Icon{Data: mustBase64Decode(svgData)},
			},
			assertion: require.Error,
		},
		{
			name: "Package/Error/NoChannels",
			v: &Package{
				Name: "anakin",
				Icon: &Icon{Data: mustBase64Decode(svgData), MediaType: "image/svg+xml"},
			},
			assertion: require.Error,
		},
		{
			name: "Package/Error/NoDefaultChannel",
			v: &Package{
				Name:     "anakin",
				Icon:     &Icon{Data: mustBase64Decode(svgData), MediaType: "image/svg+xml"},
				Channels: map[string]*Channel{"light": ch},
			},
			assertion: require.Error,
		},
		{
			name: "Package/Error/ChannelKeyNameMismatch",
			v: &Package{
				Name:           "anakin",
				Icon:           &Icon{Data: mustBase64Decode(svgData), MediaType: "image/svg+xml"},
				DefaultChannel: ch,
				Channels:       map[string]*Channel{"dark": ch},
			},
			assertion: require.Error,
		},
		{
			name: "Package/Error/InvalidChannel",
			v: &Package{
				Name:           "anakin",
				Icon:           &Icon{Data: mustBase64Decode(svgData), MediaType: "image/svg+xml"},
				DefaultChannel: ch,
				Channels:       map[string]*Channel{"light": {Name: "light"}},
			},
			assertion: require.Error,
		},
		{
			name: "Package/Error/InvalidChannelPackageLink",
			v: &Package{
				Name:           "anakin",
				Icon:           &Icon{Data: mustBase64Decode(svgData), MediaType: "image/svg+xml"},
				DefaultChannel: ch,
				Channels:       map[string]*Channel{"light": ch},
			},
			assertion: require.Error,
		},
		{
			name:      "Package/Error/DefaultChannelNotInChannelMap",
			v:         pkgIncorrectDefaultChannel,
			assertion: require.Error,
		},
		{
			name: "Icon/Success/ValidSVG",
			v: &Icon{
				Data:      mustBase64Decode(svgData),
				MediaType: "image/svg+xml",
			},
			assertion: require.NoError,
		},
		{
			name: "Icon/Success/ValidPNG",
			v: &Icon{
				Data:      mustBase64Decode(pngData),
				MediaType: "image/png",
			},
			assertion: require.NoError,
		},
		{
			name: "Icon/Success/ValidJPEG",
			v: &Icon{
				Data:      mustBase64Decode(jpegData),
				MediaType: "image/jpeg",
			},
			assertion: require.NoError,
		},
		{
			name:      "Icon/Success/Nil",
			v:         nilIcon,
			assertion: require.NoError,
		},
		//{
		//	name: "Icon/Error/NoData",
		//	v: &Icon{
		//		Data:      nil,
		//		MediaType: "image/svg+xml",
		//	},
		//	assertion: require.Error,
		//},
		//{
		//	name: "Icon/Error/NoMediaType",
		//	v: &Icon{
		//		Data:      mustBase64Decode(svgData),
		//		MediaType: "",
		//	},
		//	assertion: require.Error,
		//},
		//{
		//	name: "Icon/Error/DataIsNotImage",
		//	v: &Icon{
		//		Data:      []byte("{}"),
		//		MediaType: "application/json",
		//	},
		//	assertion: require.Error,
		//},
		//{
		//	name: "Icon/Error/DataDoesNotMatchMediaType",
		//	v: &Icon{
		//		Data:      mustBase64Decode(svgData),
		//		MediaType: "image/jpeg",
		//	},
		//	assertion: require.Error,
		//},
		{
			name:      "Channel/Success/Valid",
			v:         ch,
			assertion: require.NoError,
		},
		{
			name:      "Channel/Error/NoName",
			v:         &Channel{},
			assertion: require.Error,
		},
		{
			name: "Channel/Error/NoPackage",
			v: &Channel{
				Name: "light",
			},
			assertion: require.Error,
		},
		{
			name: "Channel/Error/NoBundles",
			v: &Channel{
				Package: pkg,
				Name:    "light",
			},
			assertion: require.Error,
		},
		{
			name: "Channel/Error/InvalidHead",
			v: &Channel{
				Package: pkg,
				Name:    "light",
				Bundles: map[string]*Bundle{
					"anakin.v0.0.0": {Name: "anakin.v0.0.0"},
					"anakin.v0.0.1": {Name: "anakin.v0.0.1"},
				},
			},
			assertion: require.Error,
		},
		{
			name: "Channel/Error/BundleKeyNameMismatch",
			v: &Channel{
				Package: pkg,
				Name:    "light",
				Bundles: map[string]*Bundle{
					"foo": {Name: "bar"},
				},
			},
			assertion: require.Error,
		},
		{
			name: "Channel/Error/InvalidBundle",
			v: &Channel{
				Package: pkg,
				Name:    "light",
				Bundles: map[string]*Bundle{
					"anakin.v0.0.0": {Name: "anakin.v0.0.0"},
				},
			},
			assertion: require.Error,
		},
		{
			name: "Channel/Error/InvalidBundleChannelLink",
			v: &Channel{
				Package: pkg,
				Name:    "light",
				Bundles: map[string]*Bundle{
					"anakin.v0.0.0": {
						Package: pkg,
						Channel: ch,
						Name:    "anakin.v0.0.0",
						Image:   "anakin-operator:v0.0.0",
					},
				},
			},
			assertion: require.Error,
		},
		{
			name: "Bundle/Success/Valid",
			v: &Bundle{
				Package:  pkg,
				Channel:  ch,
				Name:     "anakin.v0.1.0",
				Image:    "",
				Replaces: "anakin.v0.0.1",
				Skips:    []string{"anakin.v0.0.2"},
				Properties: []property.Property{
					property.MustBuildPackage("anakin", "0.1.0"),
					property.MustBuildGVK("skywalker.me", "v1alpha1", "PodRacer"),
					property.MustBuildSkips("anakin.v0.0.2"),
					property.MustBuildChannel("light", "anakin.v0.0.1"),
					property.MustBuildChannel("dark", "anakin.v0.0.1"),
				},
			},
			assertion: require.NoError,
		},
		{
			name:      "Bundle/Error/NoName",
			v:         &Bundle{},
			assertion: require.Error,
		},
		{
			name: "Bundle/Error/NoChannel",
			v: &Bundle{
				Name: "anakin.v0.1.0",
			},
			assertion: require.Error,
		},
		{
			name: "Bundle/Error/NoPackage",
			v: &Bundle{
				Channel: ch,
				Name:    "anakin.v0.1.0",
			},
			assertion: require.Error,
		},
		{
			name: "Bundle/Error/WrongPackage",
			v: &Bundle{
				Package: &Package{},
				Channel: ch,
				Name:    "anakin.v0.1.0",
			},
			assertion: require.Error,
		},
		{
			name: "Bundle/Error/ReplacesNotInChannel",
			v: &Bundle{
				Package:  pkg,
				Channel:  ch,
				Name:     "anakin.v0.1.0",
				Replaces: "anakin.v0.0.0",
			},
			assertion: require.Error,
		},
		{
			name: "Bundle/Error/InvalidProperty",
			v: &Bundle{
				Package:    pkg,
				Channel:    ch,
				Name:       "anakin.v0.1.0",
				Replaces:   "anakin.v0.0.1",
				Properties: []property.Property{{Value: json.RawMessage("")}},
			},
			assertion: require.Error,
		},
		{
			name: "Bundle/Error/EmptySkipsValue",
			v: &Bundle{
				Package:    pkg,
				Channel:    ch,
				Name:       "anakin.v0.1.0",
				Replaces:   "anakin.v0.0.1",
				Properties: []property.Property{{Type: "custom", Value: json.RawMessage("{}")}},
				Skips:      []string{""},
			},
			assertion: require.Error,
		},
		{
			name: "Bundle/Error/SkipsNotInPackage",
			v: &Bundle{
				Package:    pkg,
				Channel:    ch,
				Name:       "anakin.v0.1.0",
				Replaces:   "anakin.v0.0.1",
				Properties: []property.Property{{Type: "custom", Value: json.RawMessage("{}")}},
				Skips:      []string{"foobar"},
			},
			assertion: require.Error,
		},
		{
			name: "Bundle/Error/MissingPackage",
			v: &Bundle{
				Package:  pkg,
				Channel:  ch,
				Name:     "anakin.v0.1.0",
				Image:    "",
				Replaces: "anakin.v0.0.1",
				Skips:    []string{"anakin.v0.0.2"},
				Properties: []property.Property{
					property.MustBuildSkips("anakin.v0.0.2"),
					property.MustBuildChannel("light", "anakin.v0.0.1"),
					property.MustBuildChannel("dark", "anakin.v0.0.1"),
				},
			},
			assertion: require.Error,
		},
		{
			name: "Bundle/Error/MultiplePackages",
			v: &Bundle{
				Package:  pkg,
				Channel:  ch,
				Name:     "anakin.v0.1.0",
				Image:    "",
				Replaces: "anakin.v0.0.1",
				Skips:    []string{"anakin.v0.0.2"},
				Properties: []property.Property{
					property.MustBuildPackage("anakin", "0.1.0"),
					property.MustBuildPackage("anakin", "0.2.0"),
					property.MustBuildSkips("anakin.v0.0.2"),
					property.MustBuildChannel("light", "anakin.v0.0.1"),
					property.MustBuildChannel("dark", "anakin.v0.0.1"),
				},
			},
			assertion: require.Error,
		},
		{
			name: "RelatedImage/Success/Valid",
			v: RelatedImage{
				Name:  "foo",
				Image: "bar",
			},
			assertion: require.NoError,
		},
		{
			name: "RelatedImage/Error/NoName",
			v: RelatedImage{
				Name:  "",
				Image: "",
			},
			assertion: require.Error,
		},
		{
			name: "RelatedImage/Error/NoImage",
			v: RelatedImage{
				Name:  "foo",
				Image: "",
			},
			assertion: require.Error,
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			s.assertion(t, s.v.Validate())
		})
	}
}

func makePackageChannelBundle() (*Package, *Channel) {
	bundle1 := &Bundle{
		Name:  "anakin.v0.0.1",
		Image: "anakin-operator:v0.0.1",
		Properties: []property.Property{
			property.MustBuildPackage("anakin", "0.0.1"),
			property.MustBuildGVK("skywalker.me", "v1alpha1", "PodRacer"),
		},
	}
	bundle2 := &Bundle{
		Name:     "anakin.v0.0.2",
		Image:    "anakin-operator:v0.0.2",
		Replaces: "anakin.v0.0.1",
		Properties: []property.Property{
			property.MustBuildPackage("anakin", "0.0.2"),
			property.MustBuildGVK("skywalker.me", "v1alpha1", "PodRacer"),
		},
	}
	ch := &Channel{
		Name: "light",
		Bundles: map[string]*Bundle{
			"anakin.v0.0.1": bundle1,
			"anakin.v0.0.2": bundle2,
		},
	}
	pkg := &Package{
		Name:           "anakin",
		DefaultChannel: ch,
		Channels: map[string]*Channel{
			ch.Name: ch,
		},
	}

	bundle1.Channel, bundle2.Channel = ch, ch
	bundle1.Package, bundle2.Package, ch.Package = pkg, pkg, pkg

	return pkg, ch
}

func TestAddBundle(t *testing.T) {
	type spec struct {
		name               string
		model              Model
		bundle             Bundle
		numPkgIncrease     bool
		numBundlesIncrease bool
		pkgBundleAddedTo   string
	}
	pkg, _ := makePackageChannelBundle()
	m := Model{}
	m[pkg.Name] = pkg

	bundle1 := Bundle{
		Name:     "darth.vader.v0.0.1",
		Replaces: "anakin.v0.0.1",
		Skips:    []string{"anakin.v0.0.2"},
		Package:  &Package{Name: pkg.Name},
	}
	ch1 := &Channel{
		Name: "darkness",
		Bundles: map[string]*Bundle{
			"vader.v0.0.1": &bundle1,
		},
	}
	bundle1.Channel = ch1

	bundle2 := Bundle{
		Name:     "kylo.ren.v0.0.1",
		Replaces: "darth.vader.v0.0.1",
		Skips:    []string{"anakin.v0.0.2"},
		Package: &Package{
			Name:        "Empire",
			Description: "The Empire Will Rise Again",
			Icon: &Icon{
				MediaType: "gif",
				Data:      []byte("palpatineLaughing"),
			},
			Channels: make(map[string]*Channel),
		},
	}
	ch2 := &Channel{
		Name: "darkeness",
		Bundles: map[string]*Bundle{
			"kylo.ren.v0.0.1": &bundle2,
		},
	}
	bundle2.Channel = ch2
	bundle2.Package.Channels[ch2.Name] = ch2

	specs := []spec{
		{
			name:               "AddingToExistingPackage",
			bundle:             bundle1,
			model:              m,
			numPkgIncrease:     false,
			numBundlesIncrease: true,
			pkgBundleAddedTo:   bundle1.Package.Name,
		},
		{
			name:               "AddingNewPackage",
			bundle:             bundle2,
			model:              m,
			numPkgIncrease:     true,
			numBundlesIncrease: false,
			pkgBundleAddedTo:   "",
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			existingPkgCount := len(s.model)
			existingBundleCount := 0
			if s.pkgBundleAddedTo != "" {
				existingBundleCount = getNoOfBundles(m, s.pkgBundleAddedTo)
			}
			s.model.AddBundle(s.bundle)
			if s.numPkgIncrease {
				assert.Equal(t, len(s.model), existingPkgCount+1)
			}
			if s.numBundlesIncrease {
				assert.Equal(t, getNoOfBundles(m, s.pkgBundleAddedTo), existingBundleCount+1)
			}
		})
	}
}

func getNoOfBundles(m Model, pkg string) int {
	count := 0
	mpkg := m[pkg]
	for _, ch := range mpkg.Channels {
		count += len(ch.Bundles)
	}
	return count
}
