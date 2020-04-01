package synthesize

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/operator-framework/operator-registry/pkg/registry"
)

const (
	CSVKind                = "ClusterServiceVersion"
	CRDKind                = "CustomResourceDefinition"
	SecretKind             = "Secret"
	ConfigMapKind          = "ConfigMap"
	ClusterRoleKind        = "ClusterRole"
	ClusterRoleBindingKind = "ClusterRoleBinding"
	ServiceAccountKind     = "ServiceAccount"
	ServiceKind            = "Service"
	RoleKind               = "Role"
	RoleBindingKind        = "RoleBinding"
	PrometheusRuleKind     = "PrometheusRule"
	ServiceMonitorKind     = "ServiceMonitor"
	DeploymentKind         = "Deployment"
)

// registerGVK registers the GVK of the kube resource we use in the CSV, explicitly including all versions.
func registerGVK(scheme *runtime.Scheme) {
	scheme.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.ClusterServiceVersion{})

	scheme.AddKnownTypes(apiextensionsv1.SchemeGroupVersion, &apiextensionsv1.CustomResourceDefinition{})
	scheme.AddKnownTypes(apiextensionsv1beta1.SchemeGroupVersion, &apiextensionsv1beta1.CustomResourceDefinition{})

	scheme.AddKnownTypes(corev1.SchemeGroupVersion,
		&corev1.ServiceAccount{},
		&corev1.ConfigMap{},
		&corev1.Secret{})

	scheme.AddKnownTypes(rbacv1.SchemeGroupVersion,
		&rbacv1.Role{},
		&rbacv1.RoleBinding{},
		&rbacv1.ClusterRole{},
		&rbacv1.ClusterRoleBinding{})

	scheme.AddKnownTypes(appsv1.SchemeGroupVersion, &appsv1.Deployment{})
}

func NewGVKN(gvk schema.GroupVersionKind, name string) registry.DefinitionKey {
	return registry.DefinitionKey{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind,
		Name:    name,
	}
}

type customResourceDefinition struct {
	v1      apiextensionsv1.CustomResourceDefinition
	v1beta1 apiextensionsv1beta1.CustomResourceDefinition
	gvkn    registry.DefinitionKey
}

func (crd *customResourceDefinition) getResourceGVKNs() (gvkns []registry.DefinitionKey) {
	switch crd.gvkn.Version {
	case crd.v1.GroupVersionKind().Version:
		for _, version := range crd.v1.Spec.Versions {
			gvkns = append(gvkns, registry.DefinitionKey{
				Group:   crd.v1.Spec.Group,
				Kind:    crd.v1.Spec.Names.Kind,
				Version: version.Name,
				Name:    crd.v1.GetName(),
			})
		}
	case crd.v1beta1.GroupVersionKind().Version:
		verions := make(map[string]struct{})
		for _, ver := range crd.v1beta1.Spec.Versions {
			verions[ver.Name] = struct{}{}
		}
		// Although version is deprecated, this checks if it is still used.
		verions[crd.v1beta1.Spec.Version] = struct{}{}

		for ver := range verions {
			if ver == "" {
				continue
			}

			gvkns = append(gvkns, registry.DefinitionKey{
				Group:   crd.v1beta1.Spec.Group,
				Kind:    crd.v1beta1.Spec.Names.Kind,
				Version: ver,
				Name:    crd.v1beta1.GetName(),
			})
		}
	}
	return
}

func (s *synthesize) getCRDs() ([]customResourceDefinition, error) {
	if crdIFs, ok := s.unstructuredObjectMap[CRDKind]; ok {
		var crds []customResourceDefinition
		for _, bufObj := range crdIFs {
			switch bufObj.GroupVersionKind().GroupVersion() {
			case apiextensionsv1.SchemeGroupVersion:
				crd := apiextensionsv1.CustomResourceDefinition{}
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(bufObj.UnstructuredContent(), &crd); err != nil {
					return nil, fmt.Errorf("error decoding %v as %s kind, %v", bufObj.GroupVersionKind(), bufObj.GetKind(), err)
				}
				crds = append(crds, customResourceDefinition{
					v1:   crd,
					gvkn: NewGVKN(crd.GroupVersionKind(), crd.GetName()),
				})
			case apiextensionsv1beta1.SchemeGroupVersion:
				crd := apiextensionsv1beta1.CustomResourceDefinition{}
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(bufObj.UnstructuredContent(), &crd); err != nil {
					return nil, fmt.Errorf("error decoding %v as %s kind, %v", bufObj.GroupVersionKind(), bufObj.GetKind(), err)
				}
				crds = append(crds, customResourceDefinition{
					v1beta1: crd,
					gvkn:    NewGVKN(crd.GroupVersionKind(), crd.GetName()),
				})
			default:
				return nil, fmt.Errorf("%s APIVersion not supported", bufObj.GetAPIVersion())
			}
		}
		return crds, nil
	}
	return nil, ObjectTypeNotFoundInBundleError
}

func (s *synthesize) getClusterRoles() ([]rbacv1.ClusterRole, error) {
	if crIF, ok := s.unstructuredObjectMap[ClusterRoleKind]; ok {
		var crs []rbacv1.ClusterRole
		for _, bufObj := range crIF {
			cr := rbacv1.ClusterRole{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(bufObj.UnstructuredContent(), &cr); err != nil {
				return nil, fmt.Errorf("error decoding %v as %s kind, %v", bufObj.GroupVersionKind(), bufObj.GetKind(), err)
			}
			crs = append(crs, cr)
		}
		return crs, nil
	}
	return nil, ObjectTypeNotFoundInBundleError
}

func (s *synthesize) getClusterRoleBindings() ([]rbacv1.ClusterRoleBinding, error) {
	if crbIF, ok := s.unstructuredObjectMap[ClusterRoleBindingKind]; ok {
		var crbs []rbacv1.ClusterRoleBinding
		for _, bufObj := range crbIF {
			crb := rbacv1.ClusterRoleBinding{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(bufObj.UnstructuredContent(), &crb); err != nil {
				return nil, fmt.Errorf("error decoding %v as %s kind, %v", bufObj.GroupVersionKind(), bufObj.GetKind(), err)
			}
			crbs = append(crbs, crb)
		}
		return crbs, nil
	}
	return nil, ObjectTypeNotFoundInBundleError
}

func (s *synthesize) getServices() ([]corev1.Service, error) {
	if serviceIF, ok := s.unstructuredObjectMap[ServiceKind]; ok {
		var services []corev1.Service
		for _, bufObj := range serviceIF {
			service := corev1.Service{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(bufObj.UnstructuredContent(), &service); err != nil {
				return nil, fmt.Errorf("error decoding %v as %s kind, %v", bufObj.GroupVersionKind(), bufObj.GetKind(), err)
			}
			services = append(services, service)
		}
		return services, nil
	}
	return nil, ObjectTypeNotFoundInBundleError
}

func (s *synthesize) getSAs() ([]corev1.ServiceAccount, error) {
	if saIF, ok := s.unstructuredObjectMap[ServiceAccountKind]; ok {
		var sas []corev1.ServiceAccount
		for _, bufObj := range saIF {
			sa := corev1.ServiceAccount{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(bufObj.UnstructuredContent(), &sa); err != nil {
				return nil, fmt.Errorf("error decoding %v as %s kind, %v", bufObj.GroupVersionKind(), bufObj.GetKind(), err)
			}
			sas = append(sas, sa)
		}
		return sas, nil
	}
	return nil, ObjectTypeNotFoundInBundleError
}

func (s *synthesize) getRoles() ([]rbacv1.Role, error) {
	if roleIF, ok := s.unstructuredObjectMap[RoleKind]; ok {
		var roles []rbacv1.Role
		for _, bufObj := range roleIF {
			role := rbacv1.Role{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(bufObj.UnstructuredContent(), &role); err != nil {
				return nil, fmt.Errorf("error decoding %v as %s kind, %v", bufObj.GroupVersionKind(), bufObj.GetKind(), err)
			}
			roles = append(roles, role)
		}
		return roles, nil
	}
	return nil, ObjectTypeNotFoundInBundleError
}

func (s *synthesize) getRoleBindings() ([]rbacv1.RoleBinding, error) {
	if rbIF, ok := s.unstructuredObjectMap[RoleBindingKind]; ok {
		var rbs []rbacv1.RoleBinding
		for _, bufObj := range rbIF {
			rb := rbacv1.RoleBinding{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(bufObj.UnstructuredContent(), &rb); err != nil {
				return nil, fmt.Errorf("error decoding %v as %s kind, %v", bufObj.GroupVersionKind(), bufObj.GetKind(), err)
			}
			rbs = append(rbs, rb)
		}
		return rbs, nil
	}
	return nil, ObjectTypeNotFoundInBundleError
}

func (s *synthesize) getDeployments() ([]appsv1.Deployment, error) {
	if deployIF, ok := s.unstructuredObjectMap[DeploymentKind]; ok {
		var deploys []appsv1.Deployment
		for _, bufObj := range deployIF {
			deploy := appsv1.Deployment{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(bufObj.UnstructuredContent(), &deploy); err != nil {
				return nil, fmt.Errorf("error decoding %v as %s kind, %v", bufObj.GroupVersionKind(), bufObj.GetKind(), err)
			}
			deploys = append(deploys, deploy)
		}
		return deploys, nil
	}
	return nil, ObjectTypeNotFoundInBundleError
}
