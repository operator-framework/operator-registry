/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"
	json "encoding/json"
	"fmt"

	v1beta1 "k8s.io/api/extensions/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	extensionsv1beta1 "k8s.io/client-go/applyconfigurations/extensions/v1beta1"
	testing "k8s.io/client-go/testing"
)

// FakeReplicaSets implements ReplicaSetInterface
type FakeReplicaSets struct {
	Fake *FakeExtensionsV1beta1
	ns   string
}

var replicasetsResource = schema.GroupVersionResource{Group: "extensions", Version: "v1beta1", Resource: "replicasets"}

var replicasetsKind = schema.GroupVersionKind{Group: "extensions", Version: "v1beta1", Kind: "ReplicaSet"}

// Get takes name of the replicaSet, and returns the corresponding replicaSet object, and an error if there is any.
func (c *FakeReplicaSets) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1beta1.ReplicaSet, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(replicasetsResource, c.ns, name), &v1beta1.ReplicaSet{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ReplicaSet), err
}

// List takes label and field selectors, and returns the list of ReplicaSets that match those selectors.
func (c *FakeReplicaSets) List(ctx context.Context, opts v1.ListOptions) (result *v1beta1.ReplicaSetList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(replicasetsResource, replicasetsKind, c.ns, opts), &v1beta1.ReplicaSetList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1beta1.ReplicaSetList{ListMeta: obj.(*v1beta1.ReplicaSetList).ListMeta}
	for _, item := range obj.(*v1beta1.ReplicaSetList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested replicaSets.
func (c *FakeReplicaSets) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(replicasetsResource, c.ns, opts))

}

// Create takes the representation of a replicaSet and creates it.  Returns the server's representation of the replicaSet, and an error, if there is any.
func (c *FakeReplicaSets) Create(ctx context.Context, replicaSet *v1beta1.ReplicaSet, opts v1.CreateOptions) (result *v1beta1.ReplicaSet, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(replicasetsResource, c.ns, replicaSet), &v1beta1.ReplicaSet{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ReplicaSet), err
}

// Update takes the representation of a replicaSet and updates it. Returns the server's representation of the replicaSet, and an error, if there is any.
func (c *FakeReplicaSets) Update(ctx context.Context, replicaSet *v1beta1.ReplicaSet, opts v1.UpdateOptions) (result *v1beta1.ReplicaSet, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(replicasetsResource, c.ns, replicaSet), &v1beta1.ReplicaSet{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ReplicaSet), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeReplicaSets) UpdateStatus(ctx context.Context, replicaSet *v1beta1.ReplicaSet, opts v1.UpdateOptions) (*v1beta1.ReplicaSet, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(replicasetsResource, "status", c.ns, replicaSet), &v1beta1.ReplicaSet{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ReplicaSet), err
}

// Delete takes name of the replicaSet and deletes it. Returns an error if one occurs.
func (c *FakeReplicaSets) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(replicasetsResource, c.ns, name), &v1beta1.ReplicaSet{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeReplicaSets) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(replicasetsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1beta1.ReplicaSetList{})
	return err
}

// Patch applies the patch and returns the patched replicaSet.
func (c *FakeReplicaSets) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.ReplicaSet, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(replicasetsResource, c.ns, name, pt, data, subresources...), &v1beta1.ReplicaSet{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ReplicaSet), err
}

// Apply takes the given apply declarative configuration, applies it and returns the applied replicaSet.
func (c *FakeReplicaSets) Apply(ctx context.Context, replicaSet *extensionsv1beta1.ReplicaSetApplyConfiguration, opts v1.ApplyOptions) (result *v1beta1.ReplicaSet, err error) {
	if replicaSet == nil {
		return nil, fmt.Errorf("replicaSet provided to Apply must not be nil")
	}
	data, err := json.Marshal(replicaSet)
	if err != nil {
		return nil, err
	}
	name := replicaSet.Name
	if name == nil {
		return nil, fmt.Errorf("replicaSet.Name must be provided to Apply")
	}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(replicasetsResource, c.ns, *name, types.ApplyPatchType, data), &v1beta1.ReplicaSet{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ReplicaSet), err
}

// ApplyStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating ApplyStatus().
func (c *FakeReplicaSets) ApplyStatus(ctx context.Context, replicaSet *extensionsv1beta1.ReplicaSetApplyConfiguration, opts v1.ApplyOptions) (result *v1beta1.ReplicaSet, err error) {
	if replicaSet == nil {
		return nil, fmt.Errorf("replicaSet provided to Apply must not be nil")
	}
	data, err := json.Marshal(replicaSet)
	if err != nil {
		return nil, err
	}
	name := replicaSet.Name
	if name == nil {
		return nil, fmt.Errorf("replicaSet.Name must be provided to Apply")
	}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(replicasetsResource, c.ns, *name, types.ApplyPatchType, data, "status"), &v1beta1.ReplicaSet{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ReplicaSet), err
}

// GetScale takes name of the replicaSet, and returns the corresponding scale object, and an error if there is any.
func (c *FakeReplicaSets) GetScale(ctx context.Context, replicaSetName string, options v1.GetOptions) (result *v1beta1.Scale, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetSubresourceAction(replicasetsResource, c.ns, "scale", replicaSetName), &v1beta1.Scale{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Scale), err
}

// UpdateScale takes the representation of a scale and updates it. Returns the server's representation of the scale, and an error, if there is any.
func (c *FakeReplicaSets) UpdateScale(ctx context.Context, replicaSetName string, scale *v1beta1.Scale, opts v1.UpdateOptions) (result *v1beta1.Scale, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(replicasetsResource, "scale", c.ns, scale), &v1beta1.Scale{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Scale), err
}

// ApplyScale takes top resource name and the apply declarative configuration for scale,
// applies it and returns the applied scale, and an error, if there is any.
func (c *FakeReplicaSets) ApplyScale(ctx context.Context, replicaSetName string, scale *extensionsv1beta1.ScaleApplyConfiguration, opts v1.ApplyOptions) (result *v1beta1.Scale, err error) {
	if scale == nil {
		return nil, fmt.Errorf("scale provided to ApplyScale must not be nil")
	}
	data, err := json.Marshal(scale)
	if err != nil {
		return nil, err
	}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(replicasetsResource, c.ns, replicaSetName, types.ApplyPatchType, data, "status"), &v1beta1.Scale{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Scale), err
}
