package ctx

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	k8scontrollerclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var ctx TestContext

// TestContext represents the environment of an executing test. It can
// be considered roughly analogous to a kubeconfig context.
type TestContext struct {
	restConfig *rest.Config

	scheme *runtime.Scheme

	// client is the controller-runtime client -- we should use this from now on
	client k8scontrollerclient.Client
}

// Ctx returns a pointer to the global test context. During parallel
// test executions, Ginkgo starts one process per test "node", and
// each node will have its own context, which may or may not point to
// the same test cluster.
func Ctx() *TestContext {
	return &ctx
}

func (ctx TestContext) Logf(f string, v ...interface{}) {
	if !strings.HasSuffix(f, "\n") {
		f += "\n"
	}
	fmt.Fprintf(GinkgoWriter, f, v...)
}

func (ctx TestContext) Scheme() *runtime.Scheme {
	return ctx.scheme
}

func (ctx TestContext) RESTConfig() *rest.Config {
	return rest.CopyConfig(ctx.restConfig)
}

func (ctx TestContext) Client() k8scontrollerclient.Client {
	return ctx.client
}

func setDerivedFields(ctx *TestContext) error {
	if ctx == nil {
		return fmt.Errorf("nil test context")
	}

	if ctx.restConfig == nil {
		return fmt.Errorf("nil RESTClient")
	}

	ctx.scheme = runtime.NewScheme()

	localSchemeBuilder := runtime.NewSchemeBuilder(
		apiextensionsv1.AddToScheme,
		kscheme.AddToScheme,
		operatorsv1alpha1.AddToScheme,
		operatorsv1.AddToScheme,
	)
	if err := localSchemeBuilder.AddToScheme(ctx.scheme); err != nil {
		return err
	}

	client, err := k8scontrollerclient.New(ctx.restConfig, k8scontrollerclient.Options{
		Scheme: ctx.scheme,
	})
	if err != nil {
		return err
	}
	ctx.client = client

	return nil
}
