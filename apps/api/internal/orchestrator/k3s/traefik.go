package k3s

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
)

var helmChartGVR = schema.GroupVersionResource{
	Group:    "helm.cattle.io",
	Version:  "v1",
	Resource: "helmcharts",
}

var helmChartConfigGVR = schema.GroupVersionResource{
	Group:    "helm.cattle.io",
	Version:  "v1",
	Resource: "helmchartconfigs",
}

func (o *Orchestrator) GetTraefikConfig(ctx context.Context) (string, error) {
	dynClient, err := dynamic.NewForConfig(o.config)
	if err != nil {
		return "", fmt.Errorf("create dynamic client: %w", err)
	}

	// Try HelmChart CRD (K3s default)
	obj, err := dynClient.Resource(helmChartGVR).Namespace("kube-system").Get(ctx, "traefik", metav1.GetOptions{})
	if err == nil {
		return marshalSpec(obj.Object)
	}
	if !errors.IsNotFound(err) {
		return "", fmt.Errorf("get HelmChart: %w", err)
	}

	// Fallback: try HelmChartConfig CRD
	obj, err = dynClient.Resource(helmChartConfigGVR).Namespace("kube-system").Get(ctx, "traefik", metav1.GetOptions{})
	if err == nil {
		return marshalSpec(obj.Object)
	}

	return "", fmt.Errorf("traefik config not found: no HelmChart or HelmChartConfig resource")
}

func (o *Orchestrator) UpdateTraefikConfig(ctx context.Context, yamlContent string) error {
	dynClient, err := dynamic.NewForConfig(o.config)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}

	// Parse the YAML content — this may be spec-only (from GetTraefikConfig)
	// or a full object with a "spec:" key
	var parsed map[string]interface{}
	if parseErr := sigsyaml.Unmarshal([]byte(yamlContent), &parsed); parseErr != nil {
		return fmt.Errorf("parse yaml: %w", parseErr)
	}

	// If the YAML contains a "spec" key, use its value; otherwise treat the
	// entire parsed content as the spec (since GetTraefikConfig returns spec-only)
	spec := parsed
	if s, ok := parsed["spec"]; ok {
		if specMap, ok := s.(map[string]interface{}); ok {
			spec = specMap
		}
	}

	// Try to update HelmChart CRD first
	existing, err := dynClient.Resource(helmChartGVR).Namespace("kube-system").Get(ctx, "traefik", metav1.GetOptions{})
	if err == nil {
		if existing.Object == nil {
			return fmt.Errorf("existing HelmChart has nil object")
		}
		existing.Object["spec"] = spec
		_, updateErr := dynClient.Resource(helmChartGVR).Namespace("kube-system").Update(ctx, existing, metav1.UpdateOptions{})
		if updateErr != nil {
			return fmt.Errorf("update HelmChart: %w", updateErr)
		}
		return nil
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("get HelmChart: %w", err)
	}

	// Fallback: update HelmChartConfig CRD
	existing, err = dynClient.Resource(helmChartConfigGVR).Namespace("kube-system").Get(ctx, "traefik", metav1.GetOptions{})
	if err == nil {
		if existing.Object == nil {
			return fmt.Errorf("existing HelmChartConfig has nil object")
		}
		existing.Object["spec"] = spec
		_, updateErr := dynClient.Resource(helmChartConfigGVR).Namespace("kube-system").Update(ctx, existing, metav1.UpdateOptions{})
		if updateErr != nil {
			return fmt.Errorf("update HelmChartConfig: %w", updateErr)
		}
		return nil
	}

	return fmt.Errorf("traefik config not found: cannot update")
}

func (o *Orchestrator) RestartTraefik(ctx context.Context) error {
	deploy, err := o.client.AppsV1().Deployments("kube-system").Get(ctx, "traefik", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("traefik deployment not found: %w", err)
	}
	if deploy.Spec.Template.Annotations == nil {
		deploy.Spec.Template.Annotations = make(map[string]string)
	}
	deploy.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)
	_, err = o.client.AppsV1().Deployments("kube-system").Update(ctx, deploy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("restart traefik: %w", err)
	}
	o.logger.Info("traefik restarted")
	return nil
}

func (o *Orchestrator) GetTraefikStatus(ctx context.Context) (*orchestrator.TraefikStatus, error) {
	pods, err := o.client.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=traefik",
	})
	if err != nil || len(pods.Items) == 0 {
		return &orchestrator.TraefikStatus{Ready: false}, nil
	}
	pod := pods.Items[0]
	// Check both pod phase and container readiness
	ready := pod.Status.Phase == "Running"
	var restarts int32
	if len(pod.Status.ContainerStatuses) > 0 {
		cs := pod.Status.ContainerStatuses[0]
		restarts = cs.RestartCount
		if !cs.Ready {
			ready = false
		}
	}
	age := time.Since(pod.CreationTimestamp.Time).Truncate(time.Second).String()
	return &orchestrator.TraefikStatus{
		Ready:    ready,
		PodName:  pod.Name,
		Restarts: restarts,
		Age:      age,
	}, nil
}

var traefikMiddlewareGVR = schema.GroupVersionResource{
	Group:    "traefik.io",
	Version:  "v1alpha1",
	Resource: "middlewares",
}

const redirectMiddlewareName = "redirect-https"

// RedirectHTTPSMiddlewareRef returns the Traefik annotation value for referencing
// the redirect-https middleware in the given namespace.
func RedirectHTTPSMiddlewareRef(namespace string) string {
	return fmt.Sprintf("%s-%s@kubernetescrd", namespace, redirectMiddlewareName)
}

// EnsureRedirectHTTPSMiddleware creates a RedirectScheme middleware in the given
// namespace if it does not already exist. The middleware redirects HTTP → HTTPS
// with port "443" to avoid leaking Traefik's internal listener port.
func (o *Orchestrator) EnsureRedirectHTTPSMiddleware(ctx context.Context, namespace string) error {
	dynClient, err := dynamic.NewForConfig(o.config)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}

	_, err = dynClient.Resource(traefikMiddlewareGVR).Namespace(namespace).Get(ctx, redirectMiddlewareName, metav1.GetOptions{})
	if err == nil {
		return nil // already exists
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("get middleware %s/%s: %w", namespace, redirectMiddlewareName, err)
	}

	mw := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "traefik.io/v1alpha1",
			"kind":       "Middleware",
			"metadata": map[string]interface{}{
				"name":      redirectMiddlewareName,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/managed-by": "sailbox",
				},
			},
			"spec": map[string]interface{}{
				"redirectScheme": map[string]interface{}{
					"scheme":    "https",
					"permanent": true,
					"port":      "443",
				},
			},
		},
	}

	_, err = dynClient.Resource(traefikMiddlewareGVR).Namespace(namespace).Create(ctx, mw, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create middleware %s/%s: %w", namespace, redirectMiddlewareName, err)
	}
	o.logger.Info("redirect-https middleware created", slog.String("namespace", namespace))
	return nil
}

// marshalSpec extracts and serializes only the spec section of a K8s object.
func marshalSpec(obj map[string]interface{}) (string, error) {
	spec, ok := obj["spec"]
	if !ok {
		data, err := sigsyaml.Marshal(obj)
		if err != nil {
			return "", fmt.Errorf("marshal object: %w", err)
		}
		return string(data), nil
	}
	data, err := sigsyaml.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("marshal spec: %w", err)
	}
	return string(data), nil
}
