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

package plugin

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	backendresources "github.com/kube-bind/kube-bind/contrib/example-backend/kubernetes/resources"
	"github.com/kube-bind/kube-bind/deploy/konnector"
	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
	"github.com/kube-bind/kube-bind/pkg/authenticator"
	bindclient "github.com/kube-bind/kube-bind/pkg/client/clientset/versioned"
	"github.com/kube-bind/kube-bind/pkg/kubectl/base"
	"github.com/kube-bind/kube-bind/pkg/kubectl/bind/plugin/resources"
)

// BindOptions contains the options for creating an APIBinding.
type BindOptions struct {
	*base.Options

	// url is the argument accepted by the command. It contains the
	// reference to where an APIService exists.
	URL string

	// skipKonnector skips the deployment of the konnector.
	SkipKonnector bool
}

// NewBindOptions returns new BindOptions.
func NewBindOptions(streams genericclioptions.IOStreams) *BindOptions {
	return &BindOptions{
		Options: base.NewOptions(streams),
	}
}

// AddCmdFlags binds fields to cmd's flagset.
func (b *BindOptions) AddCmdFlags(cmd *cobra.Command) {
	b.Options.BindFlags(cmd)

	cmd.Flags().BoolVar(&b.SkipKonnector, "skip-konnector", false, "Skip the deployment of the konnector")
}

// Complete ensures all fields are initialized.
func (b *BindOptions) Complete(args []string) error {
	if err := b.Options.Complete(); err != nil {
		return err
	}

	if len(args) > 0 {
		b.URL = args[0]
	}
	return nil
}

// Validate validates the BindOptions are complete and usable.
func (b *BindOptions) Validate() error {
	if b.URL == "" {
		return errors.New("url is required as an argument") // should not happen because we validate that before
	}

	if _, err := url.Parse(b.URL); err != nil {
		return fmt.Errorf("invalid url %q: %w", b.URL, err)
	}

	return b.Options.Validate()
}

// Run starts the binding process.
func (b *BindOptions) Run(ctx context.Context, urlCh chan<- string) error {
	logger := klog.FromContext(ctx).WithValues("command", "bind", "url", b.URL)

	config, err := b.ClientConfig.ClientConfig()
	if err != nil {
		return err
	}
	kubeClient, err := kubeclient.NewForConfig(config)
	if err != nil {
		return err
	}
	apiextensionsClient, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		return err
	}
	bindClient, err := bindclient.NewForConfig(config)
	if err != nil {
		return err
	}

	var response *backendresources.AuthResponse
	auth, err := authenticator.NewDefaultAuthenticator(10*time.Minute, func(ctx context.Context, resp *backendresources.AuthResponse) error {
		response = resp
		return nil
	})
	if err != nil {
		return err
	}

	exportURL, err := url.Parse(b.URL)
	if err != nil {
		return err // should never happen because we test this in Validate()
	}

	provider, err := fetchAuthenticationRoute(exportURL.String())
	if err != nil {
		return fmt.Errorf("failed to fetch authentication url %q: %v", exportURL, err)
	}

	sessionID := rand.String(rand.IntnRange(20, 30))

	if err := authenticate(provider, auth.Endpoint(ctx), sessionID, urlCh); err != nil {
		return err
	}

	if err := auth.Execute(ctx); err != nil && !strings.Contains(err.Error(), "Server closed") {
		return err
	} else if response == nil {
		return fmt.Errorf("authentication timeout")
	}

	fmt.Fprintf(b.IOStreams.Out, "Successfully authenticated to %s\n", exportURL.String()) // nolint: errcheck

	// bootstrap the konnector
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}
	if !b.SkipKonnector {
		logger.V(1).Info("Deploying konnector")
		if err := konnector.Bootstrap(ctx, discoveryClient, dynamicClient, sets.NewString()); err != nil {
			return err
		}
	}
	first := true
	if err := wait.PollImmediateInfiniteWithContext(ctx, 1*time.Second, func(ctx context.Context) (bool, error) {
		_, err := bindClient.KubeBindV1alpha1().APIServiceBindings().List(ctx, metav1.ListOptions{})
		if err != nil {
			logger.V(2).Info("Waiting for APIServiceBindings to be served", "error", err, "host", bindClient.RESTClient())
			if first {
				fmt.Fprint(b.IOStreams.Out, "Waiting for the konnector to be ready") // nolint: errcheck
				first = false
			} else {
				fmt.Fprint(b.IOStreams.Out, ".") // nolint: errcheck
			}
		}
		return err == nil, nil
	}); err != nil {
		return err
	}

	// create the namespace
	if _, err := kubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-bind",
		},
	}, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	} else if err == nil {
		fmt.Fprintf(b.IOStreams.Out, "Created kube-binding namespace.\n") // nolint: errcheck
	}

	// look for secret of the given identity
	secrets, err := kubeClient.CoreV1().Secrets("kube-bind").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	var secretName string
	for _, s := range secrets.Items {
		if s.Annotations[resources.ClusterIDAnnotationKey] == response.ID {
			secretName = s.Name
			break
		}
	}

	// check for existing CRD
	crd, err := apiextensionsClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, response.Resource+"."+response.Group, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if secretName == "" {
		if err == nil {
			return fmt.Errorf("CRD %s.%s already exists and is not from this service provider", response.Resource, response.Group)
		}

		fmt.Fprintf(b.IOStreams.Out, "Creating secret for identity %s\n", response.ID) // nolint: errcheck
		secretName, err = resources.EnsureServiceBindingAuthData(ctx, string(response.Kubeconfig), response.ID, "kube-bind", "", kubeClient)
		if err != nil {
			return err
		}
	} else if err == nil {
		fmt.Fprintf(b.IOStreams.Out, "Found existing CRD %s. Checking owner.\n", crd.Name) // noline: errcheck

		// check if the CRD is owner-refed by the APIServiceBinding
		for _, ref := range crd.OwnerReferences {
			parts := strings.SplitN(ref.APIVersion, "/", 2)
			if parts[0] != kubebindv1alpha1.SchemeGroupVersion.Group || ref.Kind != "APIServiceBinding" {
				continue
			}

			existing, err := bindClient.KubeBindV1alpha1().APIServiceBindings().Get(ctx, ref.Name, metav1.GetOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			} else if apierrors.IsNotFound(err) {
				continue
			}

			if existing.Spec.KubeconfigSecretRef.Namespace == "kube-bind" && existing.Spec.KubeconfigSecretRef.Name == secretName {
				fmt.Fprintf(b.IOStreams.Out, "Updating credentials for existing APIServiceBinding %s\n", existing.Name) // nolint: errcheck
				_, err = resources.EnsureServiceBindingAuthData(ctx, string(response.Kubeconfig), response.ID, "kube-bind", secretName, kubeClient)
				return err
			}
		}
		return fmt.Errorf("found existing CustomResourceDefinition %s not from this service provider", response.ID)
	} else {
		fmt.Fprintf(b.IOStreams.Out, "Updating credentials\n") // noilnt: errcheck
		secretName, err = resources.EnsureServiceBindingAuthData(ctx, string(response.Kubeconfig), response.ID, "kube-bind", secretName, kubeClient)
		if err != nil {
			return err
		}
	}

	// create new APIServiceBinding.
	first = true
	if err := wait.PollInfinite(1*time.Second, func() (bool, error) {
		if !first {
			first = false
			fmt.Fprint(b.IOStreams.Out, ".") // nolint: errcheck
		}
		_, err := bindClient.KubeBindV1alpha1().APIServiceBindings().Create(ctx, &kubebindv1alpha1.APIServiceBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      response.Resource + "." + response.Group,
				Namespace: "kube-bind",
			},
			Spec: kubebindv1alpha1.APIServiceBindingSpec{
				KubeconfigSecretRef: kubebindv1alpha1.ClusterSecretKeyRef{
					LocalSecretKeyRef: kubebindv1alpha1.LocalSecretKeyRef{
						Name: secretName,
						Key:  "kubeconfig",
					},
					Namespace: "kube-bind",
				},
				Export: response.Export,
			},
		}, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return false, err
		} else if apierrors.IsAlreadyExists(err) {
			existing, err := bindClient.KubeBindV1alpha1().APIServiceBindings().Get(ctx, response.Resource+"."+response.Group, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			if existing.Spec.KubeconfigSecretRef.Namespace == "kube-bind" && existing.Spec.KubeconfigSecretRef.Name == secretName {
				return true, nil
			}
			return false, fmt.Errorf("APIServiceBinding %s.%s already exists, but from different provider", response.Resource, response.Group)
		}

		return true, nil
	}); err != nil {
		fmt.Fprintln(b.IOStreams.Out, "") // nolint: errcheck
		return err
	}
	fmt.Fprintf(b.IOStreams.Out, "\nCreated APIServiceBinding %s.%s\n", response.Resource, response.Group) // nolint: errcheck

	return nil
}
