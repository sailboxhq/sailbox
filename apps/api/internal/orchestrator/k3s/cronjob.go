package k3s

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
)

func cronJobNamespace(cj *model.CronJob) string {
	if cj.Namespace != "" {
		return cj.Namespace
	}
	return "default"
}

func cronJobK8sName(cj *model.CronJob) string {
	if cj.K8sName != "" {
		return cj.K8sName
	}
	return sanitize(cj.Name)
}

func (o *Orchestrator) buildCronJobSpec(cj *model.CronJob) *batchv1.CronJob {
	ns := cronJobNamespace(cj)
	name := cronJobK8sName(cj)

	labels := map[string]string{
		"app.kubernetes.io/name":       name,
		"app.kubernetes.io/managed-by": "sailbox",
		"sailbox/cronjob-id":           cj.ID.String(),
	}

	// Build env vars
	var envVars []corev1.EnvVar
	for k, v := range cj.EnvVars {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}

	// Resource limits
	resources := corev1.ResourceRequirements{}
	if cj.CPULimit != "" || cj.MemLimit != "" {
		resources.Limits = corev1.ResourceList{}
		resources.Requests = corev1.ResourceList{}
		if cj.CPULimit != "" {
			resources.Limits[corev1.ResourceCPU] = parseResourceOrDefault(cj.CPULimit, "500m")
			resources.Requests[corev1.ResourceCPU] = parseResourceOrDefault("50m", "50m")
		}
		if cj.MemLimit != "" {
			resources.Limits[corev1.ResourceMemory] = parseResourceOrDefault(cj.MemLimit, "512Mi")
			resources.Requests[corev1.ResourceMemory] = parseResourceOrDefault("64Mi", "64Mi")
		}
	}

	// Parse command
	command := strings.Fields(cj.Command)
	if len(command) == 0 {
		command = []string{"/bin/sh", "-c", cj.Command}
	}

	image := cj.Image
	if image == "" {
		image = "busybox:latest"
	}

	suspend := !cj.Enabled
	backoffLimit := int32(cj.BackoffLimit)
	concurrencyPolicy := batchv1.ForbidConcurrent
	switch cj.ConcurrencyPolicy {
	case "Allow":
		concurrencyPolicy = batchv1.AllowConcurrent
	case "Replace":
		concurrencyPolicy = batchv1.ReplaceConcurrent
	}

	restartPolicy := corev1.RestartPolicyOnFailure
	if cj.RestartPolicy == "Never" {
		restartPolicy = corev1.RestartPolicyNever
	}

	var activeDeadline *int64
	if cj.ActiveDeadlineSeconds > 0 {
		d := int64(cj.ActiveDeadlineSeconds)
		activeDeadline = &d
	}

	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   cj.CronExpression,
			TimeZone:                   &cj.Timezone,
			Suspend:                    &suspend,
			ConcurrencyPolicy:          concurrencyPolicy,
			SuccessfulJobsHistoryLimit: int32Ptr(1),
			FailedJobsHistoryLimit:     int32Ptr(1),
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					BackoffLimit:          &backoffLimit,
					ActiveDeadlineSeconds: activeDeadline,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: labels},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Name:      name,
								Image:     image,
								Command:   command,
								Env:       envVars,
								Resources: resources,
							}},
							RestartPolicy: restartPolicy,
						},
					},
				},
			},
		},
	}
}

func (o *Orchestrator) CreateCronJob(ctx context.Context, cj *model.CronJob) error {
	ns := cronJobNamespace(cj)
	if err := o.ensureNamespace(ctx, ns); err != nil {
		return fmt.Errorf("ensure namespace: %w", err)
	}

	spec := o.buildCronJobSpec(cj)
	_, err := o.client.BatchV1().CronJobs(ns).Create(ctx, spec, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create cronjob: %w", err)
	}

	o.logger.Info("created K8s CronJob", slog.String("name", spec.Name), slog.String("ns", ns))
	return nil
}

func (o *Orchestrator) UpdateCronJob(ctx context.Context, cj *model.CronJob) error {
	ns := cronJobNamespace(cj)
	name := cronJobK8sName(cj)

	existing, err := o.client.BatchV1().CronJobs(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return o.CreateCronJob(ctx, cj)
		}
		return err
	}

	updated := o.buildCronJobSpec(cj)
	existing.Spec = updated.Spec
	_, err = o.client.BatchV1().CronJobs(ns).Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

func (o *Orchestrator) DeleteCronJob(ctx context.Context, cj *model.CronJob) error {
	ns := cronJobNamespace(cj)
	name := cronJobK8sName(cj)
	propagation := metav1.DeletePropagationForeground

	err := o.client.BatchV1().CronJobs(ns).Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &propagation})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	o.logger.Info("deleted K8s CronJob", slog.String("name", name))
	return nil
}

func (o *Orchestrator) SuspendCronJob(ctx context.Context, cj *model.CronJob, suspend bool) error {
	ns := cronJobNamespace(cj)
	name := cronJobK8sName(cj)

	existing, err := o.client.BatchV1().CronJobs(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	existing.Spec.Suspend = &suspend
	_, err = o.client.BatchV1().CronJobs(ns).Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

func (o *Orchestrator) TriggerCronJob(ctx context.Context, cj *model.CronJob) (string, error) {
	ns := cronJobNamespace(cj)
	name := cronJobK8sName(cj)

	// Get the CronJob to use its spec as template
	existing, err := o.client.BatchV1().CronJobs(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	jobName := fmt.Sprintf("%s-manual-%d", name, time.Now().Unix())
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: ns,
			Labels: map[string]string{
				"sailbox/cronjob-id": cj.ID.String(),
				"sailbox/trigger":    "manual",
			},
		},
		Spec: existing.Spec.JobTemplate.Spec,
	}

	_, err = o.client.BatchV1().Jobs(ns).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("trigger cronjob: %w", err)
	}

	o.logger.Info("manually triggered CronJob", slog.String("job", jobName))
	return jobName, nil
}

func (o *Orchestrator) GetJobStatus(ctx context.Context, cj *model.CronJob, jobName string) (string, error) {
	ns := cronJobNamespace(cj)
	job, err := o.client.BatchV1().Jobs(ns).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
			return "succeeded", nil
		}
		if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
			return "failed", nil
		}
	}
	return "running", nil
}

// ── Unused import guard ─────────────────────────────────────────
var _ = resource.MustParse
