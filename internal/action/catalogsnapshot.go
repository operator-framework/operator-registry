package action

import (
	"context"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/operator-framework/operator-registry/internal/declcfg"
	"github.com/operator-framework/operator-registry/internal/model"
	"github.com/operator-framework/operator-registry/internal/property"
)

// CatalogSnapshot configures Run to read Subscriptions from a k8s cluster.
type CatalogSnapshot struct {
	// Non-default path to kubeconfig. This kubeconfig must contain
	// a context with permission to list Subscriptions in all namespaces.
	KubeconfigPath string

	// Client to read Subscriptions.
	client ctrlclient.Client
}

// Run gets installed bundle data from every Subscription in every namespace
// and adds them to a declarative config.
// NOTE: Run assumes the k8s config context used has permission to list
// Subscriptions across all namespaces.
func (a *CatalogSnapshot) Run(ctx context.Context) (*declcfg.DeclarativeConfig, error) {
	// Get kubeconfig, optionally at the path specified via CLI.
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = a.KubeconfigPath
	overrides := clientcmd.ConfigOverrides{}
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &overrides)
	cfg, err := cc.ClientConfig()
	if err != nil {
		return nil, err
	}

	// k8s client for Subscriptions.
	sch := runtime.NewScheme()
	if err := operatorsv1alpha1.AddToScheme(sch); err != nil {
		return nil, err
	}
	clientOpts := ctrlclient.Options{Scheme: sch}
	if a.client, err = ctrlclient.New(cfg, clientOpts); err != nil {
		return nil, err
	}
	return a.run(ctx)
}

func (a *CatalogSnapshot) run(ctx context.Context) (_ *declcfg.DeclarativeConfig, err error) {
	// List Subscriptions across all namespaces.
	subList := v1alpha1.SubscriptionList{}
	if err = a.client.List(ctx, &subList); err != nil {
		return nil, err
	}

	latestState := model.Model{}
	for _, sub := range subList.Items {
		// If the Subscription is in the process of upgrading,
		// then CurrentCSV will be set to the latest CSV.
		// Otherwise use the currently InstalledCSV.
		name := sub.Status.InstalledCSV
		if sub.Status.CurrentCSV != "" {
			name = sub.Status.CurrentCSV
		}
		pkg := &model.Package{
			Name: sub.Spec.Package,
		}
		ch := &model.Channel{
			Name:    sub.Spec.Channel,
			Package: pkg,
		}
		b := model.Bundle{
			Name:    name,
			Channel: ch,
			Package: pkg,
		}
		b.Properties = []property.Property{
			property.MustBuildChannel(ch.Name, ""),
			property.MustBuildPackage(pkg.Name, ""),
		}
		latestState.AddBundle(b)
	}

	dc := declcfg.ConvertFromModel(latestState)
	return &dc, nil
}
