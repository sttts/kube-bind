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

package resources

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

func CreateServiceAccount(ctx context.Context, client kubeclient.Interface, ns string) (*corev1.ServiceAccount, error) {
	logger := klog.FromContext(ctx)

	sa, err := client.CoreV1().ServiceAccounts(ns).Get(ctx, ClusterAdminName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			sa = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ClusterAdminName,
					Namespace: ns,
				},
			}

			logger.Info("Creating service account", "name", sa.Name)
			return client.CoreV1().ServiceAccounts(ns).Create(ctx, sa, metav1.CreateOptions{})
		}
	}

	return sa, err
}

func CreateAdminClusterRoleBinding(ctx context.Context, client kubeclient.Interface, ns string) error {
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-bind-" + ns,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      ClusterAdminName,
				Namespace: ns,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
	}

	if _, err := client.RbacV1().ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		return err
	} else if err == nil {
		return nil
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		existing, err := client.RbacV1().ClusterRoleBindings().Get(ctx, crb.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		existing.Subjects = crb.Subjects
		existing.RoleRef = crb.RoleRef
		_, err = client.RbacV1().ClusterRoleBindings().Update(ctx, crb, metav1.UpdateOptions{})
		return err
	})
}
