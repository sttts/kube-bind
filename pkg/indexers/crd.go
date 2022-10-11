package indexers

import (
	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"strings"
)

const (
	CRDByServiceBinding = "CRDByServiceBinding"
)

func IndexCRDByServiceBinding(obj interface{}) ([]string, error) {
	crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		return nil, nil
	}

	bindings := []string{}
	for _, ref := range crd.OwnerReferences {
		parts := strings.SplitN(ref.APIVersion, "/", 2)
		if parts[0] != kubebindv1alpha1.SchemeGroupVersion.Group || ref.Kind != "ServiceBinding" {
			continue
		}
		bindings = append(bindings, ref.Name)
	}
	return bindings, nil
}
