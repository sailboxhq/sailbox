package k3s

import (
	"context"
	"log/slog"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	registryNamespace = "sailbox-system"
	registryName      = "registry"
	registryImage     = "registry:2"
	registryPort      = 5000
	registryNodePort  = 30500

	// RegistryPushHost is used by Kaniko (inside pods) to push images.
	// Uses K8s service DNS — reliable within the cluster network.
	RegistryPushHost = "registry.sailbox-system.svc.cluster.local:5000"

	// RegistryPullHost is used in Deployment image references.
	// Uses NodePort on localhost — containerd trusts localhost HTTP by default.
	// No per-node config needed, works on any node automatically.
	RegistryPullHost = "localhost:30500"

	// RegistryHost is the push host (used by build.go for Kaniko)
	RegistryHost = RegistryPushHost
)

func (o *Orchestrator) EnsureRegistry(ctx context.Context) error {
	// Ensure namespace
	if err := o.ensureNamespace(ctx, registryNamespace); err != nil {
		return err
	}

	labels := map[string]string{
		"app":                          registryName,
		"app.kubernetes.io/managed-by": "sailbox",
	}

	// Check if deployment already exists
	_, err := o.client.AppsV1().Deployments(registryNamespace).Get(ctx, registryName, metav1.GetOptions{})
	if err == nil {
		return nil // already exists
	}
	if !errors.IsNotFound(err) {
		return err
	}

	// Create PVC for registry storage
	pvcName := "registry-data"
	_, err = o.client.CoreV1().PersistentVolumeClaims(registryNamespace).Get(ctx, pvcName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: registryNamespace,
				Labels:    labels,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("10Gi"),
					},
				},
			},
		}
		_, err = o.client.CoreV1().PersistentVolumeClaims(registryNamespace).Create(ctx, pvc, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}

	// Create Deployment
	replicas := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      registryName,
			Namespace: registryNamespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  registryName,
							Image: registryImage,
							Ports: []corev1.ContainerPort{{ContainerPort: int32(registryPort)}},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "data", MountPath: "/var/lib/registry"},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvcName,
								},
							},
						},
					},
				},
			},
		},
	}
	_, err = o.client.AppsV1().Deployments(registryNamespace).Create(ctx, dep, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	// Create NodePort Service — accessible on every node at localhost:30500
	// No per-node registries.yaml needed, new nodes work automatically
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      registryName,
			Namespace: registryNamespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Port:       int32(registryPort),
					TargetPort: intstr.FromInt32(int32(registryPort)),
					NodePort:   int32(registryNodePort),
				},
			},
		},
	}
	existing, err := o.client.CoreV1().Services(registryNamespace).Get(ctx, registryName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = o.client.CoreV1().Services(registryNamespace).Create(ctx, svc, metav1.CreateOptions{})
		}
	} else if existing.Spec.Type != corev1.ServiceTypeNodePort {
		// Upgrade existing ClusterIP to NodePort
		existing.Spec.Type = corev1.ServiceTypeNodePort
		existing.Spec.Ports = svc.Spec.Ports
		_, err = o.client.CoreV1().Services(registryNamespace).Update(ctx, existing, metav1.UpdateOptions{})
	}
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	o.logger.Info("deployed cluster registry", slog.String("host", RegistryHost))
	return nil
}
