package k3s

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	sigsyaml "sigs.k8s.io/yaml"
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

	var parsed map[string]interface{}
	if parseErr := sigsyaml.Unmarshal([]byte(yamlContent), &parsed); parseErr != nil {
		return fmt.Errorf("parse yaml: %w", parseErr)
	}

	// Try to update HelmChart CRD first
	existing, err := dynClient.Resource(helmChartGVR).Namespace("kube-system").Get(ctx, "traefik", metav1.GetOptions{})
	if err == nil {
		if existing.Object == nil {
			return fmt.Errorf("existing HelmChart has nil object")
		}
		// Only update spec fields
		if spec, ok := parsed["spec"]; ok {
			existing.Object["spec"] = spec
		}
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
		if spec, ok := parsed["spec"]; ok {
			existing.Object["spec"] = spec
		}
		_, updateErr := dynClient.Resource(helmChartConfigGVR).Namespace("kube-system").Update(ctx, existing, metav1.UpdateOptions{})
		if updateErr != nil {
			return fmt.Errorf("update HelmChartConfig: %w", updateErr)
		}
		return nil
	}

	return fmt.Errorf("traefik config not found: cannot update")
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
