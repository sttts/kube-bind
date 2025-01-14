/*
Copyright 2022 The Kube Bind Authors.

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

package indexers

import (
	"github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

const (
	ServiceExportByServiceExportResource    = "serviceExportByServiceExportResource"
	ServiceExportByCustomResourceDefinition = "serviceExportByCustomResourceDefinition"
)

func IndexServiceExportByServiceExportResource(obj interface{}) ([]string, error) {
	export, ok := obj.(*v1alpha1.APIServiceExport)
	if !ok {
		return nil, nil
	}

	grs := []string{}
	for _, gr := range export.Spec.Resources {
		grs = append(grs, ServiceExportByServiceExportResourceKey(export.Namespace, gr.Resource, gr.Group))
	}
	return grs, nil
}

func ServiceExportByServiceExportResourceKey(ns, resource, group string) string {
	return ns + "/" + resource + "." + group
}

func IndexServiceExportByCustomResourceDefinition(obj interface{}) ([]string, error) {
	export, ok := obj.(*v1alpha1.APIServiceExport)
	if !ok {
		return nil, nil
	}

	grs := []string{}
	for _, gr := range export.Spec.Resources {
		grs = append(grs, gr.Resource+"."+gr.Group)
	}
	return grs, nil
}
