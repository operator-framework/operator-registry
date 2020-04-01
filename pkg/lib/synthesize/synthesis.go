package synthesize

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	rv1 "k8s.io/api/rbac/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apiserver/pkg/storage/names"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/operator-framework/operator-registry/pkg/registry"
)

var ObjectTypeNotFoundInBundleError = errors.New("object kind not found in bundle")

type synthesize struct {
	manifestDir           string
	metadataDir           string
	nonInferableCSV       *NonInferableCSV
	unstructuredObjectMap map[string][]unstructured.Unstructured // map[kind]: *array of unstructured objects
	existingCSVPath       string
	synthesizedCSV        *operatorsv1alpha1.ClusterServiceVersion
	apiDescriptionMap     map[registry.GroupVersionKind]*ApiDescription
	logger                *log.Entry
	dependencies          *registry.DependenciesFile
	scheme                *runtime.Scheme
}

// SynthesizeCsvFromDirectories initialize synthesize, registry and load all manifests needed for the CSV from the bundle.
func SynthesizeCsvFromDirectories(manifestDir, metadataDir string) (*synthesize, error) {
	_, err := ioutil.ReadDir(manifestDir)
	if err != nil {
		return nil, err
	}

	s := &synthesize{
		manifestDir:           manifestDir,
		metadataDir:           metadataDir,
		nonInferableCSV:       &NonInferableCSV{},
		unstructuredObjectMap: map[string][]unstructured.Unstructured{},
		logger:                log.WithFields(log.Fields{"type": "synthesize"}),
		apiDescriptionMap:     map[registry.GroupVersionKind]*ApiDescription{},
		scheme:                runtime.NewScheme(),
	}

	registerGVK(s.scheme)

	if err := s.loadUnstructuredObjects(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *synthesize) GenerateCSV() (*operatorsv1alpha1.ClusterServiceVersion, error) {
	if err := s.generateCSV(); err != nil {
		return nil, err
	}

	return s.synthesizedCSV, nil
}

func (s *synthesize) generateCSV() error {
	csvBasics, err := s.getNonInferableCSV()
	if err != nil {
		return err
	}

	installStrategy, err := s.getInstallStrategy()
	if err != nil {
		return err
	}

	crdDefinitions, apiServiceDefinitions, err := s.getAPIs()
	if err != nil {
		return err
	}

	s.synthesizedCSV = &operatorsv1alpha1.ClusterServiceVersion{
		TypeMeta: metav1.TypeMeta{
			APIVersion: operatorsv1alpha1.ClusterServiceVersionAPIVersion,
			Kind:       CSVKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: csvBasics.Name,
		},
		Spec: operatorsv1alpha1.ClusterServiceVersionSpec{
			InstallStrategy:           *installStrategy,
			CustomResourceDefinitions: *crdDefinitions,
			APIServiceDefinitions:     *apiServiceDefinitions,
			Version:                   csvBasics.Version,
			MinKubeVersion:            csvBasics.MinKubeVersion,
			Maturity:                  csvBasics.Maturity,
			DisplayName:               csvBasics.DisplayName,
			Description:               csvBasics.Description,
			Keywords:                  csvBasics.Keywords,
			Maintainers:               csvBasics.Maintainers,
			Provider:                  csvBasics.Provider,
			Links:                     csvBasics.Links,
			Icon:                      csvBasics.Icon,
			InstallModes:              csvBasics.InstallModes,
			Replaces:                  csvBasics.Replaces,
			Labels:                    csvBasics.Labels,
			Annotations:               csvBasics.Annotations,
			Selector:                  csvBasics.Selector,
		},
	}
	return nil
}

// loadUnstructuredObjects gets supported object from bundle directory and put them into the map.
// All supported types are presented in resource_getters, extend the support types by adding KnownTypes in registerGVK.
func (s *synthesize) loadUnstructuredObjects() error {
	files, err := ioutil.ReadDir(s.manifestDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		path := filepath.Join(s.manifestDir, file.Name())

		reader, err := os.Open(path)
		if err != nil {
			return err
		}

		bufObj := unstructured.Unstructured{}
		decoder := yaml.NewYAMLOrJSONDecoder(reader, 30)
		if err = decoder.Decode(&bufObj); err != nil {
			s.logger.Warnf("file `%s` is not decodable as a Kube manifest, %v", path, err)
			continue
		}

		// Skip resource kinds that are not referenced in CSV.
		if !s.scheme.Recognizes(bufObj.GroupVersionKind()) {
			continue
		}

		if bufObj.GetKind() == CSVKind {
			if s.existingCSVPath != "" {
				return fmt.Errorf("more than one ClusterServiceVersion is found in bundle")
			}
			s.existingCSVPath = path
		}
		s.appendTostructuredObjectMap(bufObj.GetKind(), bufObj)
	}

	metafiles, err := ioutil.ReadDir(s.metadataDir)
	if err != nil {
		return err
	}

	for _, file := range metafiles {
		path := filepath.Join(s.metadataDir, file.Name())
		if err := s.loadNonInferableCSV(path); err != nil {
			return err
		}
		if err := s.loadDependencies(path); err != nil {
			return err
		}
	}
	return nil
}

// loadNonInferableCSV loads the nonInferableCSV in the synthesize structure and puts all descriptions.
// Function return nil if file is not a parsable to nonInferableCSV.
// Function returns error on duplicated CRD GVK based on the descriptions or parsing error.
func (s *synthesize) loadNonInferableCSV(path string) error {
	reader, err := os.Open(path)
	if err != nil {
		return err
	}

	csvBasics := &NonInferableCSV{}
	decoder := yaml.NewYAMLOrJSONDecoder(reader, 30)
	if err = decoder.Decode(csvBasics); err != nil || csvBasics.Name == "" {
		return nil
	}

	if s.nonInferableCSV.Name != "" {
		return fmt.Errorf("non-inferable CSV information already exist")
	}
	s.nonInferableCSV = csvBasics

	for _, apiDes := range s.nonInferableCSV.ApiDescriptions {
		for _, descriptor := range apiDes.Descriptors {
			if descriptor.DescriptorType != StatusDescriptorType && descriptor.
				DescriptorType != SpecDescriptorType && descriptor.DescriptorType != ActionDescriptorType {
				return fmt.Errorf("unrecognized `DescriptorType`, please use `status`, `spec`, or `action`")
			}
		}

		gvk, err := apiDes.GetGroupVersionKind()
		if err != nil {
			return err
		}
		if _, ok := s.apiDescriptionMap[gvk]; ok {
			return fmt.Errorf("descriptors with duplicated GVK %v are found", gvk)
		}
		api := apiDes
		s.apiDescriptionMap[gvk] = &api
	}

	return nil
}

// appendTostructuredObjectMap converts the unstructured object and try to convert it into a specified interface
// before appending to the unstructuredObjectMap.
func (s *synthesize) appendTostructuredObjectMap(kind string, bufObj unstructured.Unstructured) {
	s.unstructuredObjectMap[kind] = append(s.unstructuredObjectMap[kind], bufObj)
}

// loadDependencies tries to decode an object into registry.DependenciesFile{}.
// It updates the dependencies in synthesize once decode is successful.
func (s *synthesize) loadDependencies(path string) error {
	dependenciesFile := registry.DependenciesFile{}
	if err := registry.ParseDependenciesFile(path, &dependenciesFile); err != nil {
		if strings.HasPrefix(err.Error(), "Unable to decode the dependencies file") {
			return nil
		}
		return err
	}

	s.dependencies = &dependenciesFile
	return nil
}

// getNonInferableCSV gets the NonInferableCSV object ensures that is loaded.
func (s *synthesize) getNonInferableCSV() (*NonInferableCSV, error) {
	if s.nonInferableCSV.Name != "" {
		return s.nonInferableCSV, nil
	}
	return nil, CsvInferenceFileNotFoundError
}

// getInstallStrategy converts all Deployment kind object in bundle into install strategy and extracts the
// service account name referred in the deployments. It then adds all roleRefs from (cluster)RoleBindings that mentions
// any of the service accounts, adding the rules of (cluster)roles paring with their correspondent ServceAccounts into
// (cluster)permissions.
func (s *synthesize) getInstallStrategy() (*operatorsv1alpha1.NamedInstallStrategy, error) {
	deploys, err := s.getDeployments()
	if err != nil {
		return nil, fmt.Errorf("deployments are required for synthesizing CSV, %v", err)
	}

	var deploymentSpecs []operatorsv1alpha1.StrategyDeploymentSpec
	deploySANames := make(map[string]struct{})

	for _, deploy := range deploys {
		deployName := deploy.Name
		if deployName == "" {
			deployName = names.SimpleNameGenerator.GenerateName(deploy.GetGenerateName() + "-")
		}

		deploymentSpecs = append(deploymentSpecs, operatorsv1alpha1.StrategyDeploymentSpec{Name: deployName, Spec: deploy.Spec})

		dname := deploy.Spec.Template.Spec.ServiceAccountName
		if dname == "" {
			dname = "default"
		}
		deploySANames[dname] = struct{}{}
	}

	// Get all RBAC w/ subjects matching Deployment ServiceAccounts by name only.
	var permissions []operatorsv1alpha1.StrategyDeploymentPermissions
	rolesRefAndSANameMap := make(map[rv1.RoleRef]string)
	rbs, err := s.getRoleBindings()
	if err != nil && err != ObjectTypeNotFoundInBundleError {
		return nil, err
	}
	for _, rb := range rbs {
		// ignore optional bindings since they are not required or provided.
		if annotation, ok := rb.Annotations["operators.operatorframework.io.bundle.dependency"]; ok &&
			annotation == "optional" {
			continue
		}
		for _, sub := range rb.Subjects {
			if sub.Kind != ServiceAccountKind {
				continue
			}
			// Rolebinding APIGroup defaults to "" for service account subjects, therefore not monitored.
			if _, ok := deploySANames[sub.Name]; ok {
				rolesRefAndSANameMap[rb.RoleRef] = sub.Name
			}
		}
	}

	roles, err := s.getRoles()
	if err != nil && err != ObjectTypeNotFoundInBundleError {
		return nil, err
	}
	for _, role := range roles {
		gvk := role.GroupVersionKind()
		if SAName, ok := rolesRefAndSANameMap[rv1.RoleRef{Name: role.Name, Kind: gvk.Kind, APIGroup: gvk.Group}]; ok {
			permissions = append(permissions,
				operatorsv1alpha1.StrategyDeploymentPermissions{ServiceAccountName: SAName, Rules: role.Rules})
		}
	}

	var clusterPermissions []operatorsv1alpha1.StrategyDeploymentPermissions
	clusterRolesRefAndSANameMap := make(map[rv1.RoleRef]string)
	crbs, err := s.getClusterRoleBindings()
	if err != nil && err != ObjectTypeNotFoundInBundleError {
		return nil, err
	}
	for _, crb := range crbs {
		// ignore optional bindings since they are not required or provided.
		if annotation, ok := crb.Annotations["operators.operatorframework.io.bundle.dependency"]; ok &&
			annotation == "optional" {
			continue
		}
		for _, sub := range crb.Subjects {
			if sub.Kind != ServiceAccountKind {
				continue
			}
			// Rolebinding APIGroup defaults to "" for service account subjects, therefore not monitored.
			if _, ok := deploySANames[sub.Name]; ok {
				clusterRolesRefAndSANameMap[crb.RoleRef] = sub.Name
			}
		}
	}

	clusterRoles, err := s.getClusterRoles()
	if err != nil && err != ObjectTypeNotFoundInBundleError {
		return nil, err
	}

	for _, clusterRole := range clusterRoles {
		gvk := clusterRole.GroupVersionKind()
		// APIGroup == gvk.Group because group is extracted from apiVersion which is group/version
		if SAName, ok := rolesRefAndSANameMap[rv1.RoleRef{Name: clusterRole.GetName(), Kind: gvk.Kind, APIGroup: gvk.Group}]; ok {
			clusterPermissions = append(clusterPermissions,
				operatorsv1alpha1.StrategyDeploymentPermissions{ServiceAccountName: SAName, Rules: clusterRole.Rules})
		}
	}

	return &operatorsv1alpha1.NamedInstallStrategy{
		StrategyName: operatorsv1alpha1.InstallStrategyNameDeployment,
		StrategySpec: operatorsv1alpha1.StrategyDetailsDeployment{
			DeploymentSpecs:    deploymentSpecs,
			Permissions:        permissions,
			ClusterPermissions: clusterPermissions,
		},
	}, nil
}

// getAPIs loads all CRDs/APIServices into CRD/APIServiceDescriptions respectively.
// Since APIService does not provide kind we require all provided APIService to have a descriptor.
func (s *synthesize) getAPIs() (*operatorsv1alpha1.CustomResourceDefinitions,
	*operatorsv1alpha1.APIServiceDefinitions, error) {
	// Gets every CRD included in the bundle and check its VKN against NonInferableCSV to see if a descriptor is provided.
	// Add the descriptor if it is provided, else record VKN. (VKN is equal to GVK there since name is plural.group)
	var ownedCRDs []operatorsv1alpha1.CRDDescription
	var requiredCRDs []operatorsv1alpha1.CRDDescription
	var requiredAPIServices []operatorsv1alpha1.APIServiceDescription

	crds, err := s.getCRDs()
	if err != nil && err != ObjectTypeNotFoundInBundleError {
		return nil, nil, err
	}

	for _, crd := range crds {
		for _, gvkn := range crd.getResourceGVKNs() {
			if crdDes, ok := s.apiDescriptionMap[gvkn.GroupVersionKind()]; ok {
				des, err := crdDes.ConvertToCRDDescription()
				if err != nil {
					return nil, nil, err
				}
				ownedCRDs = append(ownedCRDs, des)
				delete(s.apiDescriptionMap, gvkn.GroupVersionKind())
			} else {
				ownedCRDs = append(ownedCRDs, operatorsv1alpha1.CRDDescription{
					Name:    gvkn.Name,
					Version: gvkn.Version,
					Kind:    gvkn.Kind,
				})
			}
		}
	}

	// Include all CRDs and APIServices defined in dependency.yaml and check its GVK against NonInferableCSV to see if a
	// descriptor is provided. This includes all required APIServices and CRDs since they are treated equally by olm.
	if s.dependencies != nil {
		for _, dep := range s.dependencies.GetDependencies() {
			if dep.GetType() == registry.GVKType {
				gvk := registry.GVKDependency{}
				if err := json.Unmarshal([]byte(dep.Value), &gvk); err != nil {
					return nil, nil, fmt.Errorf("error parsing %s as GVK, %v", dep.Value, err)
				}
				requiredGVK := registry.GroupVersionKind{
					Group:   gvk.Group,
					Version: gvk.Version,
					Kind:    gvk.Kind,
				}
				// CRDs belongs to the apiextensions group name.
				if gvk.Group == apiextensions.GroupName {
					// First try to match GVK in the apiDescriptionMap and include as required CRD description.
					// If without a match, register as required CRD by its GVK.
					if apiDes, ok := s.apiDescriptionMap[requiredGVK]; ok {
						des, err := apiDes.ConvertToCRDDescription()
						if err != nil {
							return nil, nil, err
						}
						requiredCRDs = append(requiredCRDs, des)
						delete(s.apiDescriptionMap, requiredGVK)
					} else {
						requiredCRDs = append(requiredCRDs, operatorsv1alpha1.CRDDescription{
							Name:    "plural." + requiredGVK.Group,
							Version: requiredGVK.Version,
							Kind:    requiredGVK.Kind,
						})
					}
				} else {
					// First try to match GVK in the apiDescriptionMap and include as required APIService description.
					// If without a match, register as required APIService by its GVK.
					if apiDes, ok := s.apiDescriptionMap[requiredGVK]; ok {
						des, err := apiDes.ConvertToAPIServiceDescription()
						if err != nil {
							return nil, nil, err
						}
						requiredAPIServices = append(requiredAPIServices, des)
						delete(s.apiDescriptionMap, requiredGVK)
					} else {
						requiredAPIServices = append(requiredAPIServices, operatorsv1alpha1.APIServiceDescription{
							Name:    requiredGVK.Version + "." + requiredGVK.Group,
							Group:   gvk.Group,
							Version: requiredGVK.Version,
							Kind:    requiredGVK.Kind,
						})
					}
				}
			}
		}
	}

	// Everything in apiDescriptionMap either has to be specified in dependency.yaml as required or provided
	// in the bundle.
	if len(s.apiDescriptionMap) > 0 {
		return nil, nil, fmt.Errorf("not all descriptors are matched with provided or required CRDs, including: %v",
			s.apiDescriptionMap)
	}

	return &operatorsv1alpha1.CustomResourceDefinitions{
			Owned:    ownedCRDs,
			Required: requiredCRDs,
		}, &operatorsv1alpha1.APIServiceDefinitions{
			Required: requiredAPIServices,
		}, nil
}
