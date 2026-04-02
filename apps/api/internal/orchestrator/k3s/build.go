package k3s

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
)

func (o *Orchestrator) Build(ctx context.Context, app *model.Application, opts orchestrator.BuildOpts) (*orchestrator.BuildResult, error) {
	start := time.Now()
	ns := appNamespace(app)
	appName := appK8sName(app)

	// Ensure registry exists
	if err := o.EnsureRegistry(ctx); err != nil {
		return nil, fmt.Errorf("ensure registry: %w", err)
	}

	// Ensure build namespace exists
	if err := o.ensureNamespace(ctx, ns); err != nil {
		return nil, fmt.Errorf("ensure namespace: %w", err)
	}

	// Build image tag
	tag := opts.CommitSHA
	if tag == "" {
		tag = fmt.Sprintf("build-%d", time.Now().Unix())
	} else if len(tag) > 12 {
		tag = tag[:12]
	}
	// Push tag: used by Kaniko inside the pod (cluster DNS)
	pushTag := fmt.Sprintf("%s/%s:%s", RegistryPushHost, appName, tag)
	// Pull tag: used by containerd on nodes (localhost NodePort)
	pullTag := fmt.Sprintf("%s/%s:%s", RegistryPullHost, appName, tag)

	// Determine dockerfile path and build context
	dockerfile := opts.Dockerfile
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}
	buildContext := opts.BuildContext
	if buildContext == "" || buildContext == "." {
		buildContext = ""
	}

	// Build context path inside /workspace
	contextPath := "/workspace"
	if buildContext != "" {
		contextPath = "/workspace/" + buildContext
	}

	// Build git clone command
	cloneArgs := []string{"clone", "--branch", opts.GitBranch, "--depth", "1"}
	cloneArgs = append(cloneArgs, opts.GitRepo, "/workspace")

	// Build kaniko args
	kanikoArgs := []string{
		fmt.Sprintf("--context=%s", contextPath),
		fmt.Sprintf("--dockerfile=%s/%s", contextPath, dockerfile),
		fmt.Sprintf("--destination=%s", pushTag),
		"--insecure",
		"--insecure-pull",
		"--skip-tls-verify",
		"--snapshot-mode=redo",
		"--use-new-run=true",
		"--compressed-caching=false",
	}
	if opts.NoCache {
		kanikoArgs = append(kanikoArgs, "--cache=false")
	} else {
		kanikoArgs = append(kanikoArgs, "--cache=true", fmt.Sprintf("--cache-repo=%s/%s-cache", RegistryPushHost, appName))
	}

	// Add build args
	for k, v := range opts.BuildArgs {
		kanikoArgs = append(kanikoArgs, fmt.Sprintf("--build-arg=%s=%s", k, v))
	}

	// Build env vars for kaniko
	var kanikoEnv []corev1.EnvVar
	for k, v := range opts.BuildEnvVars {
		kanikoEnv = append(kanikoEnv, corev1.EnvVar{Name: k, Value: v})
	}

	// Init containers
	gitCloneContainer := corev1.Container{
		Name:  "git-clone",
		Image: "alpine/git:latest",
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "workspace", MountPath: "/workspace"},
		},
	}

	if opts.GitToken != "" {
		// Pass repo/branch as env vars to prevent shell injection
		cloneScript := `REPO=$(echo "$GIT_REPO" | sed "s|https://|https://x-access-token:${GIT_TOKEN}@|"); git clone --branch "$GIT_BRANCH" --depth 1 "$REPO" /workspace`
		gitCloneContainer.Command = []string{"sh", "-c", cloneScript}
		gitCloneContainer.Env = []corev1.EnvVar{
			{Name: "GIT_TOKEN", Value: opts.GitToken},
			{Name: "GIT_REPO", Value: opts.GitRepo},
			{Name: "GIT_BRANCH", Value: opts.GitBranch},
		}
	} else {
		gitCloneContainer.Command = []string{"git"}
		gitCloneContainer.Args = cloneArgs
	}

	initContainers := []corev1.Container{gitCloneContainer}

	// If build type is nixpacks, add an init container to generate Dockerfile
	if opts.BuildType == "nixpacks" {
		initContainers = append(initContainers, corev1.Container{
			Name:    "nixpacks-plan",
			Image:   "ghcr.io/railwayapp/nixpacks:latest",
			Command: []string{"sh", "-c"},
			Args:    []string{"nixpacks build /workspace --out /workspace && cp /workspace/.nixpacks/Dockerfile /workspace/Dockerfile || nixpacks plan /workspace > /workspace/.nixpacks-plan.json"},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "workspace", MountPath: "/workspace"},
			},
		})
		// Override dockerfile to the generated one
		for i, arg := range kanikoArgs {
			if strings.HasPrefix(arg, "--dockerfile=") {
				kanikoArgs[i] = "--dockerfile=/workspace/Dockerfile"
				break
			}
		}
	}

	// Create the build Job
	jobName := fmt.Sprintf("build-%s-%s", appName, tag)
	if len(jobName) > 63 {
		jobName = jobName[:63]
	}
	backoffLimit := int32(0)
	ttl := int32(3600) // clean up after 1 hour

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: ns,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "sailbox",
				"sailbox/app-id":               app.ID.String(),
				"sailbox/build":                "true",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "sailbox",
						"sailbox/app-id":               app.ID.String(),
						"sailbox/build":                "true",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:  corev1.RestartPolicyNever,
					InitContainers: initContainers,
					Containers: []corev1.Container{
						{
							Name:  "kaniko",
							Image: "gcr.io/kaniko-project/executor:latest",
							Args:  kanikoArgs,
							Env:   kanikoEnv,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("2"),
									corev1.ResourceMemory: resource.MustParse("2Gi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "workspace", MountPath: "/workspace"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "workspace",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	// Delete any existing job with the same name
	_ = o.client.BatchV1().Jobs(ns).Delete(ctx, jobName, metav1.DeleteOptions{
		PropagationPolicy: func() *metav1.DeletionPropagation { p := metav1.DeletePropagationBackground; return &p }(),
	})
	time.Sleep(2 * time.Second) // wait for cleanup

	// Create the job
	_, err := o.client.BatchV1().Jobs(ns).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create build job: %w", err)
	}

	o.logger.Info("build job created",
		slog.String("job", jobName),
		slog.String("image", pushTag),
		slog.String("buildType", opts.BuildType),
	)

	// Wait for job to complete (streams logs in real-time via callback)
	logs, err := o.waitForBuildJob(ctx, ns, jobName, opts.OnLog)
	duration := time.Since(start)

	if err != nil {
		// Return a result with logs even on failure so the caller can save them
		return &orchestrator.BuildResult{
			Logs:     logs,
			Duration: duration,
		}, fmt.Errorf("build failed: %w", err)
	}

	o.logger.Info("build completed",
		slog.String("image", pushTag),
		slog.Duration("duration", duration),
	)

	return &orchestrator.BuildResult{
		Image:    pullTag,
		Duration: duration,
		Logs:     logs,
	}, nil
}

func (o *Orchestrator) waitForBuildJob(ctx context.Context, namespace, jobName string, onLog orchestrator.LogCallback) (string, error) {
	// Watch the job until it completes or fails
	watcher, err := o.client.BatchV1().Jobs(namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", jobName),
	})
	if err != nil {
		return "", fmt.Errorf("watch job: %w", err)
	}
	defer watcher.Stop()

	// Stream logs in real-time while waiting for job to finish
	var logCancel context.CancelFunc
	if onLog != nil {
		var logCtx context.Context
		logCtx, logCancel = context.WithCancel(ctx)
		go o.streamBuildLogs(logCtx, namespace, jobName, onLog)
	}

	timeout := time.After(30 * time.Minute)

	var result string
	var resultErr error

	for {
		select {
		case <-timeout:
			result = o.collectBuildLogs(ctx, namespace, jobName)
			resultErr = fmt.Errorf("build timed out after 30 minutes")
		case <-ctx.Done():
			result = o.collectBuildLogs(ctx, namespace, jobName)
			resultErr = ctx.Err()
		case event, ok := <-watcher.ResultChan():
			if !ok {
				result = o.collectBuildLogs(ctx, namespace, jobName)
				resultErr = fmt.Errorf("watch channel closed")
			} else if event.Type == watch.Modified || event.Type == watch.Added {
				job, ok := event.Object.(*batchv1.Job)
				if !ok {
					continue
				}
				for _, cond := range job.Status.Conditions {
					if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
						result = o.collectBuildLogs(ctx, namespace, jobName)
					}
					if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
						result = o.collectBuildLogs(ctx, namespace, jobName)
						resultErr = fmt.Errorf("build job failed: %s", cond.Message)
					}
				}
				if result != "" || resultErr != nil {
					// Job finished
				} else {
					continue
				}
			} else {
				continue
			}
		}

		// Job finished — stop log streaming and return
		if logCancel != nil {
			logCancel()
		}
		return result, resultErr
	}
}

// streamBuildLogs tails pod logs in real-time and pushes them via the callback.
// It follows each container (init + main) sequentially.
func (o *Orchestrator) streamBuildLogs(ctx context.Context, namespace, jobName string, onLog orchestrator.LogCallback) {
	// Wait for the pod to be created
	var podName string
	for i := 0; i < 30; i++ {
		pods, err := o.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("job-name=%s", jobName),
		})
		if err == nil && len(pods.Items) > 0 {
			podName = pods.Items[0].Name
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
	if podName == "" {
		return
	}

	// Get the full container list: init containers first, then main containers
	pod, err := o.client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return
	}
	containers := append(pod.Spec.InitContainers, pod.Spec.Containers...)

	var buf strings.Builder
	lastFlush := time.Now()

	flush := func() {
		if buf.Len() > 0 {
			onLog(buf.String())
			lastFlush = time.Now()
		}
	}

	for _, c := range containers {
		if ctx.Err() != nil {
			break
		}

		fmt.Fprintf(&buf, "=== %s ===\n", c.Name)
		flush()

		// Wait for this container to start (init containers run sequentially)
		if !o.waitForContainerRunning(ctx, namespace, podName, c.Name) {
			continue
		}

		logStream, err := o.client.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
			Container: c.Name,
			Follow:    true,
		}).Stream(ctx)
		if err != nil {
			fmt.Fprintf(&buf, "[failed to stream: %s]\n", err)
			flush()
			continue
		}

		scanner := bufio.NewScanner(logStream)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			buf.WriteString(scanner.Text() + "\n")
			if time.Since(lastFlush) > 3*time.Second {
				flush()
			}
		}
		_ = logStream.Close()
		flush()
	}
}

// waitForContainerRunning polls until a specific container is running or terminated.
func (o *Orchestrator) waitForContainerRunning(ctx context.Context, namespace, podName, containerName string) bool {
	for i := 0; i < 120; i++ {
		pod, err := o.client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false
		}

		allStatuses := append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...)
		for _, cs := range allStatuses {
			if cs.Name != containerName {
				continue
			}
			if cs.State.Running != nil || cs.State.Terminated != nil {
				return true
			}
		}

		// Check if pod itself has failed
		if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
			return false
		}

		select {
		case <-ctx.Done():
			return false
		case <-time.After(2 * time.Second):
		}
	}
	return false
}

func (o *Orchestrator) collectBuildLogs(ctx context.Context, namespace, jobName string) string {
	// Find the pod created by this job
	pods, err := o.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil || len(pods.Items) == 0 {
		return ""
	}

	podName := pods.Items[0].Name
	var allLogs strings.Builder

	// Collect logs from all containers (init + main)
	for _, c := range append(pods.Items[0].Spec.InitContainers, pods.Items[0].Spec.Containers...) {
		logStream, err := o.client.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
			Container: c.Name,
		}).Stream(ctx)
		if err != nil {
			continue
		}
		fmt.Fprintf(&allLogs, "=== %s ===\n", c.Name)
		scanner := bufio.NewScanner(logStream)
		for scanner.Scan() {
			allLogs.WriteString(scanner.Text() + "\n")
		}
		_ = logStream.Close()
	}

	return allLogs.String()
}

func (o *Orchestrator) GetBuildLogs(ctx context.Context, jobName, namespace string) (io.ReadCloser, error) {
	pods, err := o.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil || len(pods.Items) == 0 {
		return nil, fmt.Errorf("no build pod found")
	}

	return o.client.CoreV1().Pods(namespace).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{
		Follow: true,
	}).Stream(ctx)
}

func (o *Orchestrator) CancelBuild(ctx context.Context, app *model.Application) error {
	ns := appNamespace(app)
	jobs, err := o.client.BatchV1().Jobs(ns).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("sailbox/app-id=%s,sailbox/build=true", app.ID.String()),
	})
	if err != nil {
		return err
	}
	bg := metav1.DeletePropagationBackground
	for _, job := range jobs.Items {
		_ = o.client.BatchV1().Jobs(ns).Delete(ctx, job.Name, metav1.DeleteOptions{
			PropagationPolicy: &bg,
		})
	}
	return nil
}

func (o *Orchestrator) ClearBuildCache(ctx context.Context, app *model.Application) error {
	appName := appK8sName(app)
	cacheRepo := fmt.Sprintf("%s-cache", appName)

	// Run a short-lived Job that calls the registry API to delete the cache repo
	jobName := fmt.Sprintf("clear-cache-%s", appName)
	if len(jobName) > 63 {
		jobName = jobName[:63]
	}
	backoff := int32(0)
	ttl := int32(60)

	registryURL := fmt.Sprintf("http://registry.%s.svc.cluster.local:%d", registryNamespace, registryPort)
	script := fmt.Sprintf(
		`for tag in $(wget -qO- %s/v2/%s/tags/list 2>/dev/null | grep -o '"[^"]*"' | tr -d '"' | grep -v name | grep -v tags); do `+
			`wget -qO- --method=DELETE "%s/v2/%s/manifests/$(wget -qO- --header='Accept: application/vnd.docker.distribution.manifest.v2+json' -S %s/v2/%s/manifests/$tag 2>&1 | grep Docker-Content-Digest | awk '{print $2}' | tr -d '\r')" 2>/dev/null; `+
			`done; echo "cache cleared"`,
		registryURL, cacheRepo, registryURL, cacheRepo, registryURL, cacheRepo,
	)

	_ = o.client.BatchV1().Jobs(appNamespace(app)).Delete(ctx, jobName, metav1.DeleteOptions{})

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: appNamespace(app)},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:    "clear-cache",
						Image:   "busybox:latest",
						Command: []string{"sh", "-c", script},
					}},
				},
			},
		},
	}

	_, err := o.client.BatchV1().Jobs(appNamespace(app)).Create(ctx, job, metav1.CreateOptions{})
	return err
}
