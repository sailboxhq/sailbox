package k3s

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
)

func (o *Orchestrator) GetDaemonSets(ctx context.Context) ([]orchestrator.DaemonSetInfo, error) {
	dsList, err := o.client.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []orchestrator.DaemonSetInfo
	for _, ds := range dsList.Items {
		// Collect node selector as string
		var selParts []string
		for k, v := range ds.Spec.Template.Spec.NodeSelector {
			selParts = append(selParts, fmt.Sprintf("%s=%s", k, v))
		}

		// Collect container images
		var images []string
		for _, c := range ds.Spec.Template.Spec.Containers {
			images = append(images, c.Image)
		}

		result = append(result, orchestrator.DaemonSetInfo{
			Name:             ds.Name,
			Namespace:        ds.Namespace,
			DesiredScheduled: ds.Status.DesiredNumberScheduled,
			CurrentScheduled: ds.Status.CurrentNumberScheduled,
			Ready:            ds.Status.NumberReady,
			NodeSelector:     strings.Join(selParts, ", "),
			Images:           strings.Join(images, ", "),
			CreatedAt:        ds.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
		})
	}
	return result, nil
}
