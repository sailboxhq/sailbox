package k3s

import (
	"fmt"
	"log/slog"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/sailboxhq/sailbox/apps/api/internal/config"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
)

// Orchestrator implements orchestrator.Orchestrator with real K3s/K8s API calls.
type Orchestrator struct {
	client kubernetes.Interface
	config *rest.Config
	logger *slog.Logger
}

// Compile-time check that Orchestrator implements the interface.
var _ orchestrator.Orchestrator = (*Orchestrator)(nil)

// New creates a K3s orchestrator connected to a real cluster.
func New(cfg config.K8sConfig, logger *slog.Logger) (*Orchestrator, error) {
	var restCfg *rest.Config
	var err error

	if cfg.InCluster {
		restCfg, err = rest.InClusterConfig()
	} else if cfg.Kubeconfig != "" {
		restCfg, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
	} else {
		// Try default kubeconfig locations
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		restCfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides).ClientConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to build k8s config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	// Verify connection
	_, err = clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to k8s cluster: %w", err)
	}

	logger.Info("connected to K3s cluster")

	return &Orchestrator{
		client: clientset,
		config: restCfg,
		logger: logger,
	}, nil
}
