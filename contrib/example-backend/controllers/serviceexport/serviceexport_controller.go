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

package serviceexport

import (
	"context"
	"fmt"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions/apiextensions/v1"
	apiextensionslisters "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubernetesclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
	bindinformers "github.com/kube-bind/kube-bind/pkg/client/informers/externalversions/kubebind/v1alpha1"
	bindlisters "github.com/kube-bind/kube-bind/pkg/client/listers/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/committer"
	"github.com/kube-bind/kube-bind/pkg/indexers"
)

const (
	controllerName = "kube-bind-example-backend-serviceexport"
)

// NewController returns a new controller to reconcile CRDs.
func NewController(
	config *rest.Config,
	serviceExportInformer bindinformers.ServiceExportInformer,
	serviceExportResourceInformer bindinformers.ServiceExportResourceInformer,
	crdInformer apiextensionsinformers.CustomResourceDefinitionInformer,
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

		serviceExportLister:  serviceExportInformer.Lister(),
		serviceExportIndexer: serviceExportInformer.Informer().GetIndexer(),

		serviceExportResourceLister:  serviceExportResourceInformer.Lister(),
		serviceExportResourceIndexer: serviceExportResourceInformer.Informer().GetIndexer(),

		crdLister:  crdInformer.Lister(),
		crdIndexer: crdInformer.Informer().GetIndexer(),

		reconciler: reconciler{
			getCRD: func(name string) (*apiextensionsv1.CustomResourceDefinition, error) {
				return crdInformer.Lister().Get(name)
			},
			getServiceExportResource: func(ns, name string) (*kubebindv1alpha1.ServiceExportResource, error) {
				return serviceExportResourceInformer.Lister().ServiceExportResources(ns).Get(name)
			},
			createServiceExportResource: func(ctx context.Context, resource *kubebindv1alpha1.ServiceExportResource) (*kubebindv1alpha1.ServiceExportResource, error) {
				return bindClient.KubeBindV1alpha1().ServiceExportResources(resource.Namespace).Create(ctx, resource, metav1.CreateOptions{})
			},
			updateServiceExportResource: func(ctx context.Context, resource *kubebindv1alpha1.ServiceExportResource) (*kubebindv1alpha1.ServiceExportResource, error) {
				return bindClient.KubeBindV1alpha1().ServiceExportResources(resource.Namespace).Update(ctx, resource, metav1.UpdateOptions{})
			},
			deleteServiceExportResource: func(ctx context.Context, ns, name string) error {
				return bindClient.KubeBindV1alpha1().ServiceExportResources(ns).Delete(ctx, name, metav1.DeleteOptions{})
			},
		},

		commit: committer.NewCommitter[*kubebindv1alpha1.ServiceExport, *kubebindv1alpha1.ServiceExportSpec, *kubebindv1alpha1.ServiceExportStatus](
			func(ns string) committer.Patcher[*kubebindv1alpha1.ServiceExport] {
				return bindClient.KubeBindV1alpha1().ServiceExports(ns)
			},
		),
	}

	indexers.AddIfNotPresentOrDie(serviceExportInformer.Informer().GetIndexer(), cache.Indexers{
		indexers.ServiceExportByServiceExportResource: indexers.IndexServiceExportByServiceExportResource,
	})

	indexers.AddIfNotPresentOrDie(serviceExportInformer.Informer().GetIndexer(), cache.Indexers{
		indexers.ServiceExportByCustomResourceDefinition: indexers.IndexServiceExportByCustomResourceDefinition,
	})

	serviceExportInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueServiceExport(logger, obj)
		},
		UpdateFunc: func(old, newObj interface{}) {
			c.enqueueServiceExport(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueServiceExport(logger, obj)
		},
	})

	serviceExportResourceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueServiceExportResource(logger, obj)
		},
		UpdateFunc: func(old, newObj interface{}) {
			c.enqueueServiceExportResource(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueServiceExportResource(logger, obj)
		},
	})

	crdInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueCRD(logger, obj)
		},
		UpdateFunc: func(old, newObj interface{}) {
			c.enqueueCRD(logger, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueCRD(logger, obj)
		},
	})

	return c, nil
}

type Resource = committer.Resource[*kubebindv1alpha1.ServiceExportSpec, *kubebindv1alpha1.ServiceExportStatus]
type CommitFunc = func(context.Context, *Resource, *Resource) error

// controller reconciles ServiceNamespaces by creating a Namespace for each, and deleting it if
// the ServiceNamespace is deleted.
type controller struct {
	queue workqueue.RateLimitingInterface

	bindClient bindclient.Interface
	kubeClient kubernetesclient.Interface

	serviceExportLister  bindlisters.ServiceExportLister
	serviceExportIndexer cache.Indexer

	serviceExportResourceLister  bindlisters.ServiceExportResourceLister
	serviceExportResourceIndexer cache.Indexer

	crdLister  apiextensionslisters.CustomResourceDefinitionLister
	crdIndexer cache.Indexer

	reconciler

	commit CommitFunc
}

func (c *controller) enqueueServiceExport(logger klog.Logger, obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	logger.V(2).Info("queueing ServiceExport", "key", key)
	c.queue.Add(key)
}

func (c *controller) enqueueServiceExportResource(logger klog.Logger, obj interface{}) {
	serKey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	exports, err := c.serviceExportIndexer.ByIndex(indexers.ServiceExportByServiceExportResource, serKey)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	for _, obj := range exports {
		key, err := cache.MetaNamespaceKeyFunc(obj)
		if err != nil {
			runtime.HandleError(err)
			continue
		}
		logger.V(2).Info("queueing ServiceExport", "key", key, "reason", "ServiceExportResource", "ServiceExportResourceKey", serKey)
		c.queue.Add(key)
	}
}

func (c *controller) enqueueCRD(logger klog.Logger, obj interface{}) {
	crdKey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	exports, err := c.serviceExportIndexer.ByIndex(indexers.ServiceExportByCustomResourceDefinition, crdKey)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	for _, obj := range exports {
		export, ok := obj.(*kubebindv1alpha1.ServiceExport)
		if !ok {
			runtime.HandleError(fmt.Errorf("unexpected type %T", obj))
			return
		}
		key, err := cache.MetaNamespaceKeyFunc(export)
		if err != nil {
			runtime.HandleError(err)
			continue
		}
		logger.V(2).Info("queueing ServiceExport", "key", key, "reason", "CustomResourceDefinition", "CustomResourceDefinitionKey", crdKey)
		c.queue.Add(key)
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
	logger := klog.FromContext(ctx)

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(err)
		return nil // we cannot do anything
	}

	obj, err := c.serviceExportLister.ServiceExports(ns).Get(name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		logger.V(2).Info("ServiceExport not found, ignoring")
		return nil // nothing we can do
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