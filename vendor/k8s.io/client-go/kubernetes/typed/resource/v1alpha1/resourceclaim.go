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

package v1alpha1

import (
	"context"
	json "encoding/json"
	"fmt"
	"time"

	v1alpha1 "k8s.io/api/resource/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	resourcev1alpha1 "k8s.io/client-go/applyconfigurations/resource/v1alpha1"
	scheme "k8s.io/client-go/kubernetes/scheme"
	rest "k8s.io/client-go/rest"
)

// ResourceClaimsGetter has a method to return a ResourceClaimInterface.
// A group's client should implement this interface.
type ResourceClaimsGetter interface {
	ResourceClaims(namespace string) ResourceClaimInterface
}

// ResourceClaimInterface has methods to work with ResourceClaim resources.
type ResourceClaimInterface interface {
	Create(ctx context.Context, resourceClaim *v1alpha1.ResourceClaim, opts v1.CreateOptions) (*v1alpha1.ResourceClaim, error)
	Update(ctx context.Context, resourceClaim *v1alpha1.ResourceClaim, opts v1.UpdateOptions) (*v1alpha1.ResourceClaim, error)
	UpdateStatus(ctx context.Context, resourceClaim *v1alpha1.ResourceClaim, opts v1.UpdateOptions) (*v1alpha1.ResourceClaim, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.ResourceClaim, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.ResourceClaimList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ResourceClaim, err error)
	Apply(ctx context.Context, resourceClaim *resourcev1alpha1.ResourceClaimApplyConfiguration, opts v1.ApplyOptions) (result *v1alpha1.ResourceClaim, err error)
	ApplyStatus(ctx context.Context, resourceClaim *resourcev1alpha1.ResourceClaimApplyConfiguration, opts v1.ApplyOptions) (result *v1alpha1.ResourceClaim, err error)
	ResourceClaimExpansion
}

// resourceClaims implements ResourceClaimInterface
type resourceClaims struct {
	client rest.Interface
	ns     string
}

// newResourceClaims returns a ResourceClaims
func newResourceClaims(c *ResourceV1alpha1Client, namespace string) *resourceClaims {
	return &resourceClaims{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the resourceClaim, and returns the corresponding resourceClaim object, and an error if there is any.
func (c *resourceClaims) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ResourceClaim, err error) {
	result = &v1alpha1.ResourceClaim{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("resourceclaims").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ResourceClaims that match those selectors.
func (c *resourceClaims) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ResourceClaimList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.ResourceClaimList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("resourceclaims").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested resourceClaims.
func (c *resourceClaims) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("resourceclaims").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a resourceClaim and creates it.  Returns the server's representation of the resourceClaim, and an error, if there is any.
func (c *resourceClaims) Create(ctx context.Context, resourceClaim *v1alpha1.ResourceClaim, opts v1.CreateOptions) (result *v1alpha1.ResourceClaim, err error) {
	result = &v1alpha1.ResourceClaim{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("resourceclaims").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(resourceClaim).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a resourceClaim and updates it. Returns the server's representation of the resourceClaim, and an error, if there is any.
func (c *resourceClaims) Update(ctx context.Context, resourceClaim *v1alpha1.ResourceClaim, opts v1.UpdateOptions) (result *v1alpha1.ResourceClaim, err error) {
	result = &v1alpha1.ResourceClaim{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("resourceclaims").
		Name(resourceClaim.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(resourceClaim).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *resourceClaims) UpdateStatus(ctx context.Context, resourceClaim *v1alpha1.ResourceClaim, opts v1.UpdateOptions) (result *v1alpha1.ResourceClaim, err error) {
	result = &v1alpha1.ResourceClaim{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("resourceclaims").
		Name(resourceClaim.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(resourceClaim).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the resourceClaim and deletes it. Returns an error if one occurs.
func (c *resourceClaims) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("resourceclaims").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *resourceClaims) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("resourceclaims").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched resourceClaim.
func (c *resourceClaims) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ResourceClaim, err error) {
	result = &v1alpha1.ResourceClaim{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("resourceclaims").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}

// Apply takes the given apply declarative configuration, applies it and returns the applied resourceClaim.
func (c *resourceClaims) Apply(ctx context.Context, resourceClaim *resourcev1alpha1.ResourceClaimApplyConfiguration, opts v1.ApplyOptions) (result *v1alpha1.ResourceClaim, err error) {
	if resourceClaim == nil {
		return nil, fmt.Errorf("resourceClaim provided to Apply must not be nil")
	}
	patchOpts := opts.ToPatchOptions()
	data, err := json.Marshal(resourceClaim)
	if err != nil {
		return nil, err
	}
	name := resourceClaim.Name
	if name == nil {
		return nil, fmt.Errorf("resourceClaim.Name must be provided to Apply")
	}
	result = &v1alpha1.ResourceClaim{}
	err = c.client.Patch(types.ApplyPatchType).
		Namespace(c.ns).
		Resource("resourceclaims").
		Name(*name).
		VersionedParams(&patchOpts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}

// ApplyStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating ApplyStatus().
func (c *resourceClaims) ApplyStatus(ctx context.Context, resourceClaim *resourcev1alpha1.ResourceClaimApplyConfiguration, opts v1.ApplyOptions) (result *v1alpha1.ResourceClaim, err error) {
	if resourceClaim == nil {
		return nil, fmt.Errorf("resourceClaim provided to Apply must not be nil")
	}
	patchOpts := opts.ToPatchOptions()
	data, err := json.Marshal(resourceClaim)
	if err != nil {
		return nil, err
	}

	name := resourceClaim.Name
	if name == nil {
		return nil, fmt.Errorf("resourceClaim.Name must be provided to Apply")
	}

	result = &v1alpha1.ResourceClaim{}
	err = c.client.Patch(types.ApplyPatchType).
		Namespace(c.ns).
		Resource("resourceclaims").
		Name(*name).
		SubResource("status").
		VersionedParams(&patchOpts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
