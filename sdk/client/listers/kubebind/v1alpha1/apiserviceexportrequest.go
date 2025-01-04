/*
Copyright 2025 The Kube Bind Authors.

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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/listers"
	"k8s.io/client-go/tools/cache"

	v1alpha1 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha1"
)

// APIServiceExportRequestLister helps list APIServiceExportRequests.
// All objects returned here must be treated as read-only.
type APIServiceExportRequestLister interface {
	// List lists all APIServiceExportRequests in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.APIServiceExportRequest, err error)
	// APIServiceExportRequests returns an object that can list and get APIServiceExportRequests.
	APIServiceExportRequests(namespace string) APIServiceExportRequestNamespaceLister
	APIServiceExportRequestListerExpansion
}

// aPIServiceExportRequestLister implements the APIServiceExportRequestLister interface.
type aPIServiceExportRequestLister struct {
	listers.ResourceIndexer[*v1alpha1.APIServiceExportRequest]
}

// NewAPIServiceExportRequestLister returns a new APIServiceExportRequestLister.
func NewAPIServiceExportRequestLister(indexer cache.Indexer) APIServiceExportRequestLister {
	return &aPIServiceExportRequestLister{listers.New[*v1alpha1.APIServiceExportRequest](indexer, v1alpha1.Resource("apiserviceexportrequest"))}
}

// APIServiceExportRequests returns an object that can list and get APIServiceExportRequests.
func (s *aPIServiceExportRequestLister) APIServiceExportRequests(namespace string) APIServiceExportRequestNamespaceLister {
	return aPIServiceExportRequestNamespaceLister{listers.NewNamespaced[*v1alpha1.APIServiceExportRequest](s.ResourceIndexer, namespace)}
}

// APIServiceExportRequestNamespaceLister helps list and get APIServiceExportRequests.
// All objects returned here must be treated as read-only.
type APIServiceExportRequestNamespaceLister interface {
	// List lists all APIServiceExportRequests in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.APIServiceExportRequest, err error)
	// Get retrieves the APIServiceExportRequest from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.APIServiceExportRequest, error)
	APIServiceExportRequestNamespaceListerExpansion
}

// aPIServiceExportRequestNamespaceLister implements the APIServiceExportRequestNamespaceLister
// interface.
type aPIServiceExportRequestNamespaceLister struct {
	listers.ResourceIndexer[*v1alpha1.APIServiceExportRequest]
}
