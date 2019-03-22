package appregistry

import (
	"errors"
	"fmt"
	"strings"

	marketplace "github.com/operator-framework/operator-marketplace/pkg/client/clientset/versioned"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// TODO: this entire file can be removed once marketplace operator transitions
// to using the new flag 'registry'.

func NewOperatorSourceCRSpecifier(kubeconfig string, logger *logrus.Entry) (OperatorSourceSpecifier, error) {
	marketplaceClient, err := NewMarketplaceClient(kubeconfig, logger)
	if err != nil {
		return nil, err
	}

	return &operatorSourceCRSpecifier{
		marketplace: marketplaceClient,
	}, nil
}

type operatorSourceCRSpecifier struct {
	marketplace marketplace.Interface
}

func (p *operatorSourceCRSpecifier) Parse(specifiers []string) ([]*Source, error) {
	sources := make([]*Source, 0)
	allErrors := []error{}

	for _, specifier := range specifiers {
		source, err := p.ParseOne(specifier)
		if err != nil {
			allErrors = append(allErrors, err)
			continue
		}

		sources = append(sources, source)
	}

	err := utilerrors.NewAggregate(allErrors)
	return sources, err
}

func (p *operatorSourceCRSpecifier) ParseOne(specifier string) (*Source, error) {
	key, err := split(specifier)
	if err != nil {
		return nil, err
	}

	opsrc, err := p.marketplace.MarketplaceV1alpha1().OperatorSources(key.Namespace).Get(key.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	source := &Source{
		Endpoint:          opsrc.Spec.Endpoint,
		RegistryNamespace: opsrc.Spec.RegistryNamespace,
		Secret: types.NamespacedName{
			Namespace: opsrc.GetNamespace(),
			Name:      opsrc.Spec.AuthorizationToken.SecretName,
		},
	}

	return source, nil
}

func split(sourceName string) (*types.NamespacedName, error) {
	split := strings.Split(sourceName, "/")
	if len(split) != 2 {
		return nil, errors.New(fmt.Sprintf("OperatorSource name should be specified in this format {namespace}/{name}"))
	}

	return &types.NamespacedName{
		Namespace: split[0],
		Name:      split[1],
	}, nil
}

func NewMarketplaceClient(kubeconfig string, logger *logrus.Entry) (clientset marketplace.Interface, err error) {
	var config *rest.Config

	if kubeconfig != "" {
		logger.Infof("Loading kube client config from path %q", kubeconfig)
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		logger.Infof("Using in-cluster kube client config")
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		err = fmt.Errorf("Cannot load config for REST client: %v", err)
		return
	}

	clientset, err = marketplace.NewForConfig(config)
	return
}
