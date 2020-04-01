package synthesize

import (
	"encoding/json"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiVer "github.com/operator-framework/api/pkg/lib/version"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/operator-framework/operator-registry/pkg/registry"
)

type NonInferableCSV struct {
	Name            string                 `json:"name"`
	Version         apiVer.OperatorVersion `json:"version"`
	MinKubeVersion  string                 `json:"minKubeVersion"`
	InstallModes    []v1alpha1.InstallMode `json:"installModes"`
	Description     string                 `json:"description,omitempty"`
	DisplayName     string                 `json:"displayName,omitempty"`
	Keywords        []string               `json:"keywords,omitempty"`
	Labels          map[string]string      `json:"labels,omitempty" `
	Links           []v1alpha1.AppLink     `json:"links,omitempty"`
	Maintainers     []v1alpha1.Maintainer  `json:"maintainers,omitempty"`
	Maturity        string                 `json:"maturity,omitempty"`
	Provider        v1alpha1.AppLink       `json:"provider,omitempty"`
	Selector        *metav1.LabelSelector  `json:"selector,omitempty"`
	Icon            []v1alpha1.Icon        `json:"icon,omitempty"`
	Replaces        string                 `json:"replaces,omitempty"`
	Annotations     map[string]string      `json:"annotations,omitempty"`
	ApiDescriptions []ApiDescription       `json:"apis,omitempty"`
}

type ApiDescription struct {
	Name        string                          `json:"name"`
	Version     string                          `json:"version"`
	Kind        string                          `json:"kind"`
	DisplayName string                          `json:"displayName,omitempty"`
	Description string                          `json:"description,omitempty"`
	Resources   []v1alpha1.APIResourceReference `json:"resources,omitempty"`
	Descriptors []Descriptor                    `json:"descriptors,omitempty"`
}

func (d *ApiDescription) GetGroupVersionKind() (registry.GroupVersionKind, error) {
	group := strings.SplitN(d.Name, ".", 2)
	if len(group) != 2 || group[1] == "" {
		return registry.GroupVersionKind{}, fmt.Errorf("error getting group from description name: `%s`, "+
			"expecting form `plural.group` or `version.group`", d.Name)
	}
	return registry.GroupVersionKind{
		Group:   group[1],
		Version: d.Version,
		Kind:    d.Kind,
	}, nil
}

// ConvertToCRDDescription converts all descriptions into CRDDescription.
func (d *ApiDescription) ConvertToCRDDescription() (v1alpha1.CRDDescription, error) {
	descriptors, err := convertDescriptors(d.Descriptors)
	if err != nil {
		return v1alpha1.CRDDescription{}, err
	}

	return v1alpha1.CRDDescription{
		Name:              d.Name,
		Version:           d.Version,
		Kind:              d.Kind,
		DisplayName:       d.DisplayName,
		Description:       d.Description,
		Resources:         d.Resources,
		StatusDescriptors: descriptors.statusDescriptors,
		SpecDescriptors:   descriptors.specDescriptors,
		ActionDescriptor:  descriptors.actionDescriptors,
	}, nil
}

// ConvertToAPIServiceDescription converts all descriptions APIService Description.
// It requires name to be in the format of "version.group".
func (d *ApiDescription) ConvertToAPIServiceDescription() (v1alpha1.APIServiceDescription, error) {
	gvk, err := d.GetGroupVersionKind()
	if err != nil {
		return v1alpha1.APIServiceDescription{}, err
	}

	descriptors, err := convertDescriptors(d.Descriptors)
	if err != nil {
		return v1alpha1.APIServiceDescription{}, err
	}

	return v1alpha1.APIServiceDescription{
		Name:              d.Name,
		Group:             gvk.Group,
		Version:           d.Version,
		Kind:              d.Kind,
		DisplayName:       d.DisplayName,
		Description:       d.Description,
		Resources:         d.Resources,
		StatusDescriptors: descriptors.statusDescriptors,
		SpecDescriptors:   descriptors.specDescriptors,
		ActionDescriptor:  descriptors.actionDescriptors,
	}, nil
}

type DescriptorType string

var (
	StatusDescriptorType DescriptorType = "status"
	SpecDescriptorType   DescriptorType = "spec"
	ActionDescriptorType DescriptorType = "action"
)

type Descriptor struct {
	DescriptorType DescriptorType  `json:"descriptorType"`
	Path           string          `json:"path"`
	DisplayName    string          `json:"displayName,omitempty"`
	Description    string          `json:"description,omitempty"`
	XDescriptors   []string        `json:"x-descriptors,omitempty"`
	Value          json.RawMessage `json:"value,omitempty"`
}

type descriptors struct {
	statusDescriptors []v1alpha1.StatusDescriptor
	specDescriptors   []v1alpha1.SpecDescriptor
	actionDescriptors []v1alpha1.ActionDescriptor
}

func convertDescriptors(descriptors []Descriptor) (d descriptors, err error) {
	for _, descriptor := range descriptors {
		switch descriptor.DescriptorType {
		case StatusDescriptorType:
			d.statusDescriptors = append(d.statusDescriptors, v1alpha1.StatusDescriptor{
				Path:         descriptor.Path,
				DisplayName:  descriptor.DisplayName,
				Description:  descriptor.Description,
				XDescriptors: descriptor.XDescriptors,
				Value:        descriptor.Value,
			})
		case SpecDescriptorType:
			d.specDescriptors = append(d.specDescriptors, v1alpha1.SpecDescriptor{
				Path:         descriptor.Path,
				DisplayName:  descriptor.DisplayName,
				Description:  descriptor.Description,
				XDescriptors: descriptor.XDescriptors,
				Value:        descriptor.Value,
			})
		case ActionDescriptorType:
			d.actionDescriptors = append(d.actionDescriptors, v1alpha1.ActionDescriptor{
				Path:         descriptor.Path,
				DisplayName:  descriptor.DisplayName,
				Description:  descriptor.Description,
				XDescriptors: descriptor.XDescriptors,
				Value:        descriptor.Value,
			})
		default:
			err = fmt.Errorf("unrecognized `DescriptorType`, please use `status`, `spec`, or `action`")
		}
	}
	return
}
