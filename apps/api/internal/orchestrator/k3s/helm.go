package k3s

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
)

func (o *Orchestrator) GetHelmReleases(ctx context.Context) ([]orchestrator.HelmRelease, error) {
	// Helm stores release data in Secrets with the label owner=helm
	secrets, err := o.client.CoreV1().Secrets("").List(ctx, metav1.ListOptions{
		LabelSelector: "owner=helm",
	})
	if err != nil {
		return nil, fmt.Errorf("list helm secrets: %w", err)
	}

	// Track the latest version of each release
	latest := make(map[string]orchestrator.HelmRelease)

	for _, sec := range secrets.Items {
		// Secret name format: sh.helm.release.v1.<name>.v<version>
		name := sec.Name
		if !strings.HasPrefix(name, "sh.helm.release.v1.") {
			continue
		}

		parts := strings.Split(name, ".")
		if len(parts) < 6 {
			continue
		}

		releaseName := parts[4]
		version := strings.TrimPrefix(parts[5], "v")

		status := sec.Labels["status"]
		chart := sec.Labels["name"]
		appVersion := sec.Labels["version"]

		rel := orchestrator.HelmRelease{
			Name:       releaseName,
			Namespace:  sec.Namespace,
			Chart:      chart,
			Version:    version,
			AppVersion: appVersion,
			Status:     status,
			Updated:    sec.CreationTimestamp.Format("2006-01-02 15:04:05"),
		}

		key := sec.Namespace + "/" + releaseName
		if existing, ok := latest[key]; !ok || versionNum(version) > versionNum(existing.Version) {
			latest[key] = rel
		}
	}

	result := make([]orchestrator.HelmRelease, 0, len(latest))
	for _, rel := range latest {
		result = append(result, rel)
	}
	return result, nil
}

func versionNum(v string) int {
	n, _ := strconv.Atoi(v)
	return n
}
