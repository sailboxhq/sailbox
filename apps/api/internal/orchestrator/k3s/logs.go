package k3s

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
)

func (o *Orchestrator) StreamLogs(ctx context.Context, app *model.Application, opts orchestrator.LogOpts) (io.ReadCloser, error) {
	ns := appNamespace(app)
	name := appK8sName(app)

	// Find the first running pod for this app
	pods, err := o.client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s", name),
	})
	if err != nil {
		return nil, err
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pods found for %s", name)
	}

	// Prefer a Running pod
	podName := pods.Items[0].Name
	for _, p := range pods.Items {
		if p.Status.Phase == corev1.PodRunning {
			podName = p.Name
			break
		}
	}

	logOpts := &corev1.PodLogOptions{
		Follow:     opts.Follow,
		Timestamps: opts.Timestamps,
	}
	if opts.TailLines > 0 {
		logOpts.TailLines = &opts.TailLines
	}
	if opts.Container != "" {
		logOpts.Container = opts.Container
	}
	if !opts.Since.IsZero() {
		since := metav1.NewTime(opts.Since)
		logOpts.SinceTime = &since
	}

	return o.client.CoreV1().Pods(ns).GetLogs(podName, logOpts).Stream(ctx)
}

func (o *Orchestrator) StreamPodLogs(ctx context.Context, app *model.Application, podName string, opts orchestrator.LogOpts) (io.ReadCloser, error) {
	ns := appNamespace(app)
	name := appK8sName(app)

	// Validate pod belongs to this app
	pod, err := o.client.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("pod not found: %w", err)
	}
	if pod.Labels["app.kubernetes.io/name"] != name {
		return nil, fmt.Errorf("pod %s does not belong to app %s", podName, name)
	}

	logOpts := &corev1.PodLogOptions{
		Follow:     opts.Follow,
		Timestamps: opts.Timestamps,
	}
	if opts.TailLines > 0 {
		logOpts.TailLines = &opts.TailLines
	}
	if opts.Container != "" {
		logOpts.Container = opts.Container
	}
	if !opts.Since.IsZero() {
		since := metav1.NewTime(opts.Since)
		logOpts.SinceTime = &since
	}

	return o.client.CoreV1().Pods(ns).GetLogs(podName, logOpts).Stream(ctx)
}
