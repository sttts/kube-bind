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

package servicenamespace

import (
	"context"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	rbacinformers "k8s.io/client-go/informers/rbac/v1"
	kubernetesclient "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	rbaclisters "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
	bindinformers "github.com/kube-bind/kube-bind/pkg/client/informers/externalversions/kubebind/v1alpha1"
	bindlisters "github.com/kube-bind/kube-bind/pkg/client/listers/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/committer"
)

const (
	controllerName = "kube-bind-example-backend-servicenamespace"
)

// NewController returns a new controller for ServiceNamespaces.
func NewController(
	config *rest.Config,
	serviceNamespaceInformer bindinformers.ServiceNamespaceInformer,
	clusterBindingInformer bindinformers.ClusterBindingInformer,
	serviceExportInformer bindinformers.ServiceExportInformer,
	namespaceInformer coreinformers.NamespaceInformer,
	roleInformer rbacinformers.RoleInformer,
	roleBindingInformer rbacinformers.RoleBindingInformer,
) (*controller, error) {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName)

	logger := klog.Background().WithValues("controller", controllerName)

	config = rest.CopyConfig(config)
	config = rest.AddUserAgent(config, controllerName)

	bindClient, err := bindclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	kubeClient, err := kubernetesclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	c := &controller{
		queue: queue,

		bindClient: bindClient,
		kubeClient: kubeClient,

		serviceNamespaceLister:  serviceNamespaceInformer.Lister(),
		serviceNamespaceIndexer: serviceNamespaceInformer.Informer().GetIndexer(),

		clusterBindingLister:  clusterBindingInformer.Lister(),
		clusterBindingIndexer: clusterBindingInformer.Informer().GetIndexer(),

		serviceExportLister:  serviceExportInformer.Lister(),
		serviceExportIndexer: serviceExportInformer.Informer().GetIndexer(),

		namespaceLister:  namespaceInformer.Lister(),
		namespaceIndexer: namespaceInformer.Informer().GetIndexer(),

		roleLister:  roleInformer.Lister(),
		roleIndexer: roleInformer.Informer().GetIndexer(),

		roleBindingLister:  roleBindingInformer.Lister(),
		roleBindingIndexer: roleBindingInformer.Informer().GetIndexer(),

		reconciler: reconciler{
			getNamespace: namespaceInformer.Lister().Get,
			createNamespace: func(ctx context.Context, ns *corev1.Namespace) (*corev1.Namespace, error) {
				return kubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
			},
			deleteNamespace: func(ctx context.Context, name string) error {
				return kubeClient.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
			},

			getServiceNamespace: func(ns, name string) (*kubebindv1alpha1.ServiceNamespace, error) {
				return serviceNamespaceInformer.Lister().ServiceNamespaces(ns).Get(name)
			},

			getClusterBinding: func(ns string) (*kubebindv1alpha1.ClusterBinding, error) {
				return clusterBindingInformer.Lister().ClusterBindings(ns).Get("cluster")
			},

			getRole: func(ns, name string) (*rbacv1.Role, error) {
				return roleInformer.Lister().Roles(ns).Get(name)
			},
			createRole: func(ctx context.Context, cr *rbacv1.Role) (*rbacv1.Role, error) {
				return kubeClient.RbacV1().Roles(cr.Namespace).Create(ctx, cr, metav1.CreateOptions{})
			},
			updateRole: func(ctx context.Context, cr *rbacv1.Role) (*rbacv1.Role, error) {
				return kubeClient.RbacV1().Roles(cr.Namespace).Update(ctx, cr, metav1.UpdateOptions{})
			},

			getRoleBinding: func(ns, name string) (*rbacv1.RoleBinding, error) {
				return roleBindingInformer.Lister().RoleBindings(ns).Get(name)
			},
			createRoleBinding: func(ctx context.Context, crb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
				return kubeClient.RbacV1().RoleBindings(crb.Namespace).Create(ctx, crb, metav1.CreateOptions{})
			},
			updateRoleBinding: func(ctx context.Context, crb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
				return kubeClient.RbacV1().RoleBindings(crb.Namespace).Update(ctx, crb, metav1.UpdateOptions{})
			},

			listServiceExports: func(ns string) ([]*kubebindv1alpha1.ServiceExport, error) {
				return serviceExportInformer.Lister().ServiceExports(ns).List(labels.Everything())
			},
		},

		commit: committer.NewCommitter[*kubebindv1alpha1.ServiceNamespace, *kubebindv1alpha1.ServiceNamespaceSpec, *kubebindv1alpha1.ServiceNamespaceStatus](
			func(ns string) committer.Patcher[*kubebindv1alpha1.ServiceNamespace] {
				return bindClient.KubeBindV1alpha1().ServiceNamespaces(ns)
			},
		),
	}

	// nolint:errcheck
	namespaceInformer.Informer().GetIndexer().AddIndexers(cache.Indexers{
		ByServiceNamespace: IndexByServiceNamespace,
	})

	namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueNamespace(logger, obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueNamespace(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueNamespace(logger, obj)
		},
	})

	serviceNamespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueServiceNamespace(logger, obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueServiceNamespace(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueServiceNamespace(logger, obj)
		},
	})

	clusterBindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueClusterBinding(logger, obj)
		},
	})

	serviceExportInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueServiceExport(logger, obj)
		},
		UpdateFunc: func(old, newObj interface{}) {
			oldExport, ok := old.(*kubebindv1alpha1.ServiceExport)
			if !ok {
				return
			}
			newExport, ok := old.(*kubebindv1alpha1.ServiceExport)
			if !ok {
				return
			}
			if reflect.DeepEqual(oldExport.Spec, newExport.Spec) {
				return
			}
			c.enqueueServiceExport(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueServiceExport(logger, obj)
		},
	})

	return c, nil
}

type Resource = committer.Resource[*kubebindv1alpha1.ServiceNamespaceSpec, *kubebindv1alpha1.ServiceNamespaceStatus]
type CommitFunc = func(context.Context, *Resource, *Resource) error

// controller reconciles ServiceNamespaces by creating a Namespace for each, and deleting it if
// the ServiceNamespace is deleted.
type controller struct {
	queue workqueue.RateLimitingInterface

	bindClient bindclient.Interface
	kubeClient kubernetesclient.Interface

	namespaceLister  corelisters.NamespaceLister
	namespaceIndexer cache.Indexer

	serviceNamespaceLister  bindlisters.ServiceNamespaceLister
	serviceNamespaceIndexer cache.Indexer

	clusterBindingLister  bindlisters.ClusterBindingLister
	clusterBindingIndexer cache.Indexer

	serviceExportLister  bindlisters.ServiceExportLister
	serviceExportIndexer cache.Indexer

	roleLister  rbaclisters.RoleLister
	roleIndexer cache.Indexer

	roleBindingLister  rbaclisters.RoleBindingLister
	roleBindingIndexer cache.Indexer

	reconciler

	commit CommitFunc
}

func (c *controller) enqueueServiceNamespace(logger klog.Logger, obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	logger.V(2).Info("queueing ServiceNamespace", "key", key)
	c.queue.Add(key)
}

func (c *controller) enqueueClusterBinding(logger klog.Logger, obj interface{}) {
	cbKey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	ns, _, err := cache.SplitMetaNamespaceKey(cbKey)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	snss, err := c.serviceNamespaceIndexer.ByIndex(cache.NamespaceIndex, ns)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	logger.V(2).Info("queueing ServiceNamespaces", "namespace", ns, "number", len(snss), "reason", "ClusterBinding", "ClusterBindingKey", cbKey)
	for _, sns := range snss {
		key, err := cache.MetaNamespaceKeyFunc(sns)
		if err != nil {
			runtime.HandleError(err)
			continue
		}
		c.queue.Add(key)
	}
}

func (c *controller) enqueueServiceExport(logger klog.Logger, obj interface{}) {
	seKey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	ns, _, err := cache.SplitMetaNamespaceKey(seKey)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	snss, err := c.serviceNamespaceIndexer.ByIndex(cache.NamespaceIndex, ns)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	logger.V(2).Info("queueing ServiceNamespaces", "namespace", ns, "number", len(snss), "reason", "ServiceExport", "ServiceExportKey", seKey)
	for _, sns := range snss {
		key, err := cache.MetaNamespaceKeyFunc(sns)
		if err != nil {
			runtime.HandleError(err)
			continue
		}
		c.queue.Add(key)
	}
}

func (c *controller) enqueueNamespace(logger klog.Logger, obj interface{}) {
	if ns, ok := obj.(*corev1.Namespace); ok {
		if value, found := ns.Annotations[serviceNamespaceAnnotationKey]; found {
			ns, name, err := ServiceNamespaceFromAnnotation(value)
			if err != nil {
				runtime.HandleError(err)
				return
			}
			key := ns + "/" + name

			logger.V(2).Info("queueing ServiceNamespace", "key", key, "reason", "Namespace", "NamespaceKey", key)
			c.queue.Add(key)
		}
	}
}

// Start starts the controller, which stops when ctx.Done() is closed.
func (c *controller) Start(ctx context.Context, numThreads int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	logger := klog.FromContext(ctx).WithValues("controller", controllerName)

	logger.Info("Starting controller")
	defer logger.Info("Shutting down controller")

	for i := 0; i < numThreads; i++ {
		go wait.UntilWithContext(ctx, c.startWorker, time.Second)
	}

	<-ctx.Done()
}

func (c *controller) startWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *controller) processNextWorkItem(ctx context.Context) bool {
	// Wait until there is a new item in the working queue
	k, quit := c.queue.Get()
	if quit {
		return false
	}
	key := k.(string)

	logger := klog.FromContext(ctx).WithValues("key", key)
	ctx = klog.NewContext(ctx, logger)
	logger.V(2).Info("processing key")

	// No matter what, tell the queue we're done with this key, to unblock
	// other workers.
	defer c.queue.Done(key)

	if err := c.process(ctx, key); err != nil {
		runtime.HandleError(fmt.Errorf("%q controller failed to sync %q, err: %w", controllerName, key, err))
		c.queue.AddRateLimited(key)
		return true
	}
	c.queue.Forget(key)
	return true
}

func (c *controller) process(ctx context.Context, key string) error {
	snsNamespace, snsName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(err)
		return nil // we cannot do anything
	}
	nsName := snsNamespace + "-" + snsName

	obj, err := c.getServiceNamespace(snsNamespace, snsName)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		if err := c.deleteNamespace(ctx, nsName); err != nil && !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	old := obj
	obj = obj.DeepCopy()

	var errs []error
	if err := c.reconcile(ctx, obj); err != nil {
		errs = append(errs, err)
	}

	// Regardless of whether reconcile returned an error or not, always try to patch status if needed. Return the
	// reconciliation error at the end.

	// If the object being reconciled changed as a result, update it.
	oldResource := &Resource{ObjectMeta: old.ObjectMeta, Spec: &old.Spec, Status: &old.Status}
	newResource := &Resource{ObjectMeta: obj.ObjectMeta, Spec: &obj.Spec, Status: &obj.Status}
	if err := c.commit(ctx, oldResource, newResource); err != nil {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}
