package action

import (
	"context"
	"testing"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/operator-framework/operator-registry/internal/declcfg"
	"github.com/operator-framework/operator-registry/internal/property"
)

func TestCatalogSnapShot(t *testing.T) {
	schemeBuilder := runtime.NewSchemeBuilder(
		operatorsv1alpha1.AddToScheme,
	)
	if err := schemeBuilder.AddToScheme(scheme.Scheme); err != nil {
		t.Fatal(err)
	}

	type spec struct {
		name      string
		subs      []runtime.Object
		expectDC  *declcfg.DeclarativeConfig
		assertion require.ErrorAssertionFunc
	}

	specs := []spec{
		{
			name:     "Success/NoSub",
			subs:     []runtime.Object{},
			expectDC: &declcfg.DeclarativeConfig{},
		},
		{
			name: "Success/OneSub",
			subs: []runtime.Object{
				newSubscription("sub-1", "ns", "foo", "stable", "foo.v0.1.0", ""),
			},
			expectDC: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{
					{
						Schema:         "olm.package",
						Name:           "foo",
						DefaultChannel: "stable",
					},
				},
				Bundles: []declcfg.Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.1.0",
						Package: "foo",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
							property.MustBuildPackage("foo", ""),
						},
					},
				},
			},
		},
		{
			name: "Success/MultipleSubs",
			subs: []runtime.Object{
				newSubscription("sub-1", "ns", "foo", "stable", "foo.v0.1.0", ""),
				newSubscription("sub-2", "ns", "bar", "stable", "bar.v0.1.0", ""),
				newSubscription("sub-3", "ns", "baz", "fast", "baz.v0.1.0", ""),
			},
			expectDC: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{
					{
						Schema:         "olm.package",
						Name:           "bar",
						DefaultChannel: "stable",
					},
					{
						Schema:         "olm.package",
						Name:           "baz",
						DefaultChannel: "fast",
					},
					{
						Schema:         "olm.package",
						Name:           "foo",
						DefaultChannel: "stable",
					},
				},
				Bundles: []declcfg.Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "bar.v0.1.0",
						Package: "bar",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
							property.MustBuildPackage("bar", ""),
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "baz.v0.1.0",
						Package: "baz",
						Properties: []property.Property{
							property.MustBuildChannel("fast", ""),
							property.MustBuildPackage("baz", ""),
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.1.0",
						Package: "foo",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
							property.MustBuildPackage("foo", ""),
						},
					},
				},
			},
		},
		{
			name: "Success/TwoSubsSamePackage",
			subs: []runtime.Object{
				newSubscription("foo-sub", "ns-1", "foo", "stable", "foo.v0.1.0", ""),
				newSubscription("foo-sub", "ns-2", "foo", "fast", "foo.v0.2.0-alpha.1", ""),
			},
			expectDC: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{
					{
						Schema:         "olm.package",
						Name:           "foo",
						DefaultChannel: "stable",
					},
				},
				Bundles: []declcfg.Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.1.0",
						Package: "foo",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
							property.MustBuildPackage("foo", ""),
						},
					},
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.2.0-alpha.1",
						Package: "foo",
						Properties: []property.Property{
							property.MustBuildChannel("fast", ""),
							property.MustBuildPackage("foo", ""),
						},
					},
				},
			},
		},
		{
			name: "Success/TwoSubsIdentical",
			subs: []runtime.Object{
				newSubscription("foo-sub", "ns-1", "foo", "stable", "foo.v0.1.0", ""),
				newSubscription("foo-sub", "ns-2", "foo", "stable", "foo.v0.1.0", ""),
			},
			expectDC: &declcfg.DeclarativeConfig{
				Packages: []declcfg.Package{
					{
						Schema:         "olm.package",
						Name:           "foo",
						DefaultChannel: "stable",
					},
				},
				Bundles: []declcfg.Bundle{
					{
						Schema:  "olm.bundle",
						Name:    "foo.v0.1.0",
						Package: "foo",
						Properties: []property.Property{
							property.MustBuildChannel("stable", ""),
							property.MustBuildPackage("foo", ""),
						},
					},
				},
			},
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			snapshot := CatalogSnapshot{}
			snapshot.client = fake.NewFakeClientWithScheme(scheme.Scheme, s.subs...)
			cfg, err := snapshot.run(context.TODO())
			if s.assertion == nil {
				s.assertion = require.NoError
			}
			s.assertion(t, err)
			require.Equal(t, s.expectDC, cfg)
		})
	}
}

func newSubscription(name, namespace, pkg, channel, installedCSV, currentCSV string) *operatorsv1alpha1.Subscription {
	return &operatorsv1alpha1.Subscription{
		ObjectMeta: v1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: &operatorsv1alpha1.SubscriptionSpec{
			Package: pkg,
			Channel: channel,
		},
		Status: operatorsv1alpha1.SubscriptionStatus{
			InstalledCSV: installedCSV,
			CurrentCSV:   currentCSV,
		},
	}
}
