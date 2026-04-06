package util

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
)

// KubeClients holds the Kubernetes clients used by CLI commands.
type KubeClients struct {
	CRClient   client.Client
	Clientset  kubernetes.Interface
	RestConfig *rest.Config
	Namespace  string
}

// NewKubeClients creates K8s clients from kubeconfig/context/namespace flags.
func NewKubeClients(kubeconfigPath, context, namespace string) (*KubeClients, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		rules.ExplicitPath = kubeconfigPath
	}

	overrides := &clientcmd.ConfigOverrides{}
	if context != "" {
		overrides.CurrentContext = context
	}

	config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)

	restConfig, err := config.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to connect to cluster: %w", err)
	}

	// Resolve namespace: flag > kubeconfig > "default"
	ns := namespace
	if ns == "" {
		ns, _, _ = config.Namespace()
		if ns == "" {
			ns = "default"
		}
	}

	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to register CRD scheme: %w", err)
	}

	crClient, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create CRD client: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &KubeClients{
		CRClient:   crClient,
		Clientset:  clientset,
		RestConfig: restConfig,
		Namespace:  ns,
	}, nil
}
