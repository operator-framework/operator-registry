package action

import (
	"context"
	"sort"
	"time"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/connectivity"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/operator-framework/operator-registry/internal/declcfg"
	"github.com/operator-framework/operator-registry/internal/model"
	"github.com/operator-framework/operator-registry/pkg/api"
	grpc "github.com/operator-framework/operator-registry/pkg/client"
)

// CatalogSnapshot configures Run to read CatalogSources from a k8s cluster.
type CatalogSnapshot struct {
	// Non-default path to kubeconfig. This kubeconfig must contain
	// a context with permission to list CatalogSources in all namespaces.
	KubeconfigPath string

	// Client to read CatalogSources.
	client ctrlclient.Client
	// Client to query the underlying catalog server for bundles.
	newGRPCClient func(string) (*grpc.Client, error)
}

// Run reads all bundles via gRPC from all CatalogSources in all namespaces
// and adds them in CatalogSource priority order to a declarative config.
// NOTE: Run assumes the k8s config context used has permission to list
// CatalogSources across all namespaces.
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

	// k8s client for CatalogSources.
	schemeBuilder := runtime.NewSchemeBuilder(
		operatorsv1alpha1.AddToScheme,
	)
	if err := schemeBuilder.AddToScheme(scheme.Scheme); err != nil {
		return nil, err
	}
	clientOpts := ctrlclient.Options{Scheme: scheme.Scheme}
	if a.client, err = ctrlclient.New(cfg, clientOpts); err != nil {
		return nil, err
	}

	// Standard, insecure gRPC client.
	// QUESTION: does this need auth config?
	a.newGRPCClient = grpc.NewClient
	return a.run(ctx)
}

func (a *CatalogSnapshot) run(ctx context.Context) (_ *declcfg.DeclarativeConfig, err error) {
	// List CatalogSources across all namespaces.
	csList := v1alpha1.CatalogSourceList{}
	if err = a.client.List(ctx, &csList); err != nil {
		return nil, err
	}
	// Sort CatalogSources by priority so iterators are added in priority order.
	sort.Slice(csList.Items, func(i, j int) bool {
		return csList.Items[i].Spec.Priority < csList.Items[j].Spec.Priority
	})
	prioritizedIterators := make([]*grpc.BundleIterator, len(csList.Items))

	// Add gRPC bundle iterators for available catalogs to prioritized list.
	// TODO: paralellize but maintain priority.
	catalogState := model.Model{}
	for i, item := range csList.Items {
		// TODO: support non-gRPC CatalogSources.
		if item.Spec.SourceType != operatorsv1alpha1.SourceTypeGrpc {
			continue
		}

		// Only READY CatalogSources will be available to open a connection.
		csKey := ctrlclient.ObjectKeyFromObject(&item)
		connState := item.Status.GRPCConnectionState
		switch connState.LastObservedState {
		case connectivity.Ready.String():
		case connectivity.Connecting.String():
			// Wait until READY.
			wait.PollUntil(200*time.Millisecond, func() (bool, error) {
				cs := v1alpha1.CatalogSource{}
				if err := a.client.Get(ctx, csKey, &cs); err != nil {
					return false, err
				}
				return cs.Status.GRPCConnectionState.LastObservedState == connectivity.Ready.String(), nil
			}, ctx.Done())
		default:
			logrus.Debugf("CatalogSource %s has gRPC connection state %q, skipping", csKey, connState.LastObservedState)
			continue
		}

		// gRPC address is required here.
		addr := connState.Address
		if addr == "" {
			logrus.Debugf("CatalogSource %s address is empty", csKey)
			continue
		}
		// QUESTION: are these addresses accessible outside of the cluster?
		grpcclient, err := a.newGRPCClient(addr)
		if err != nil {
			return nil, err
		}
		if prioritizedIterators[i], err = grpcclient.ListBundles(ctx); err != nil {
			return nil, err
		}
	}

	// Iterate in reverse priority so lower priority bundles are overridden by AddBundle().
	for i := len(prioritizedIterators) - 1; i >= 0; i-- {
		iterator := prioritizedIterators[i]
		for apiBundle := iterator.Next(); apiBundle != nil; apiBundle = iterator.Next() {
			if err := iterator.Error(); err != nil {
				return nil, err
			}
			b, err := api.ConvertAPIBundleToModelBundle(apiBundle)
			if err != nil {
				return nil, err
			}
			// TODO: this might add the wrong default channel, since catalogState's packages
			// were not initialized with default channels.
			catalogState.AddBundle(*b)
		}
	}

	dc := declcfg.ConvertFromModel(catalogState)
	return &dc, nil
}
