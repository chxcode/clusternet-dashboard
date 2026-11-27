package kube

import (
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type ParentConnection struct {
	Name      string
	InCluster bool
}

// NewRestConfig returns the identity used to build the client, alongside its configuration.
func NewRestConfig() (*rest.Config, ParentConnection, error) {
	path := os.Getenv("KUBECONFIG")
	if path == "" {
		if cfg, err := rest.InClusterConfig(); err == nil {
			return cfg, ParentConnection{Name: "in-cluster", InCluster: true}, nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, ParentConnection{}, err
		}
		path = filepath.Join(home, ".kube", "config")
	}
	rules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: path}
	overrides := &clientcmd.ConfigOverrides{CurrentContext: os.Getenv("KUBE_CONTEXT")}
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
	raw, err := loader.RawConfig()
	if err != nil {
		return nil, ParentConnection{}, err
	}
	cfg, err := loader.ClientConfig()
	if err != nil {
		return nil, ParentConnection{}, err
	}
	name := overrides.CurrentContext
	if name == "" {
		name = raw.CurrentContext
	}
	return cfg, ParentConnection{Name: name}, nil
}

func NewExplorerFromConfig(cfg *rest.Config) (*Explorer, error) {
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}
	// A release deletion or new subscription requires one operator-confirmed
	// request, including when the API server replies with Retry-After.
	dynamicConfig := dynamic.ConfigFor(cfg)
	dynamicConfig.GroupVersion = nil
	dynamicREST, err := rest.UnversionedRESTClientFor(dynamicConfig)
	if err != nil {
		return nil, err
	}
	return NewExplorer(client, discoveryClient, dynamic.New(operatorRESTClient{dynamicREST})), nil
}

type operatorRESTClient struct{ rest.Interface }

func (client operatorRESTClient) Delete() *rest.Request {
	return client.Interface.Delete().MaxRetries(0)
}

func (client operatorRESTClient) Patch(pt types.PatchType) *rest.Request {
	return client.Interface.Patch(pt).MaxRetries(0)
}

func (client operatorRESTClient) Post() *rest.Request {
	return client.Interface.Post().MaxRetries(0)
}
