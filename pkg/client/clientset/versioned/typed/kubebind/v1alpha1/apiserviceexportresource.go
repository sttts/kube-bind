/*
Copyright The Kube Bind Authors.

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
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"

	v1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	scheme "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned/scheme"
)

// APIServiceExportResourcesGetter has a method to return a APIServiceExportResourceInterface.
// A group's client should implement this interface.
type APIServiceExportResourcesGetter interface {
	APIServiceExportResources(namespace string) APIServiceExportResourceInterface
}

// APIServiceExportResourceInterface has methods to work with APIServiceExportResource resources.
type APIServiceExportResourceInterface interface {
	Create(ctx context.Context, aPIServiceExportResource *v1alpha1.APIServiceExportResource, opts v1.CreateOptions) (*v1alpha1.APIServiceExportResource, error)
	Update(ctx context.Context, aPIServiceExportResource *v1alpha1.APIServiceExportResource, opts v1.UpdateOptions) (*v1alpha1.APIServiceExportResource, error)
	UpdateStatus(ctx context.Context, aPIServiceExportResource *v1alpha1.APIServiceExportResource, opts v1.UpdateOptions) (*v1alpha1.APIServiceExportResource, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.APIServiceExportResource, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.APIServiceExportResourceList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.APIServiceExportResource, err error)
	APIServiceExportResourceExpansion
}

// aPIServiceExportResources implements APIServiceExportResourceInterface
type aPIServiceExportResources struct {
	client rest.Interface
	ns     string
}

// newAPIServiceExportResources returns a APIServiceExportResources
func newAPIServiceExportResources(c *KubeBindV1alpha1Client, namespace string) *aPIServiceExportResources {
	return &aPIServiceExportResources{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the aPIServiceExportResource, and returns the corresponding aPIServiceExportResource object, and an error if there is any.
func (c *aPIServiceExportResources) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.APIServiceExportResource, err error) {
	result = &v1alpha1.APIServiceExportResource{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("apiserviceexportresources").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of APIServiceExportResources that match those selectors.
func (c *aPIServiceExportResources) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.APIServiceExportResourceList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.APIServiceExportResourceList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("apiserviceexportresources").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested aPIServiceExportResources.
func (c *aPIServiceExportResources) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("apiserviceexportresources").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a aPIServiceExportResource and creates it.  Returns the server's representation of the aPIServiceExportResource, and an error, if there is any.
func (c *aPIServiceExportResources) Create(ctx context.Context, aPIServiceExportResource *v1alpha1.APIServiceExportResource, opts v1.CreateOptions) (result *v1alpha1.APIServiceExportResource, err error) {
	result = &v1alpha1.APIServiceExportResource{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("apiserviceexportresources").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(aPIServiceExportResource).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a aPIServiceExportResource and updates it. Returns the server's representation of the aPIServiceExportResource, and an error, if there is any.
func (c *aPIServiceExportResources) Update(ctx context.Context, aPIServiceExportResource *v1alpha1.APIServiceExportResource, opts v1.UpdateOptions) (result *v1alpha1.APIServiceExportResource, err error) {
	result = &v1alpha1.APIServiceExportResource{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("apiserviceexportresources").
		Name(aPIServiceExportResource.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(aPIServiceExportResource).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *aPIServiceExportResources) UpdateStatus(ctx context.Context, aPIServiceExportResource *v1alpha1.APIServiceExportResource, opts v1.UpdateOptions) (result *v1alpha1.APIServiceExportResource, err error) {
	result = &v1alpha1.APIServiceExportResource{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("apiserviceexportresources").
		Name(aPIServiceExportResource.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(aPIServiceExportResource).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the aPIServiceExportResource and deletes it. Returns an error if one occurs.
func (c *aPIServiceExportResources) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("apiserviceexportresources").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *aPIServiceExportResources) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("apiserviceexportresources").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched aPIServiceExportResource.
func (c *aPIServiceExportResources) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.APIServiceExportResource, err error) {
	result = &v1alpha1.APIServiceExportResource{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("apiserviceexportresources").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
