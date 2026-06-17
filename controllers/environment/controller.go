package environment

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/skpr/pinchy/apis/pinchy/v1beta1"
)

const (
	// ControllerName is used to identify this controller in logs and events.
	ControllerName = "environment-controller"
)

// Reconciler reconciles a Chown object.
type Reconciler struct {
	client.Client
	Log      logr.Logger
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
}

// Reconcile a Chown object.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("request", req.NamespacedName)

	log.Info("Starting reconcile loop")

	environment := &v1beta1.Environment{}

	if err := r.Get(ctx, req.NamespacedName, environment); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Create the Pod if it does not exist. Pods are effectively immutable once
	// created, so we never issue an update — only create-if-not-found.
	// The Pod is owned by the Environment so it is garbage-collected when the
	// Environment is deleted.
	//
	// Note: the Environment name is derived from the workspace path (not the
	// session ID), so one Pod is shared by all sessions operating in the same
	// directory. See internal/envname for the naming scheme.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      environment.Name,
			Namespace: environment.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "environment",
				"app.kubernetes.io/instance":   environment.Name,
				"app.kubernetes.io/part-of":    "pinchy",
				"app.kubernetes.io/managed-by": ControllerName,
			},
		},
	}

	err := r.Get(ctx, types.NamespacedName{Name: environment.Name, Namespace: environment.Namespace}, pod)
	switch {
	case apierrors.IsNotFound(err):
		container := corev1.Container{
			Name:    "environment",
			Image:   "alpine:3.23",
			Command: []string{"sleep", "infinity"},
		}

		pod.Spec = corev1.PodSpec{}

		// Mount the workspace directory into the environment when a path has
		// been provided. The path is a directory on the node (the host
		// ./workspace bind mount), so we hostPath-mount it at the same path
		// inside the container, keeping the environment's view consistent with
		// every session's working directory. The directory is assumed to exist.
		if environment.Spec.Path != "" {
			hostPathDir := corev1.HostPathDirectory

			pod.Spec.Volumes = []corev1.Volume{
				{
					Name: "workspace",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: environment.Spec.Path,
							Type: &hostPathDir,
						},
					},
				},
			}

			container.VolumeMounts = []corev1.VolumeMount{
				{
					Name:      "workspace",
					MountPath: environment.Spec.Path,
				},
			}

			container.WorkingDir = environment.Spec.Path
		}

		pod.Spec.Containers = []corev1.Container{container}

		if err := controllerutil.SetControllerReference(environment, pod, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.Create(ctx, pod); err != nil {
			return ctrl.Result{}, err
		}

	case err != nil:
		return ctrl.Result{}, err
	}

	// Reflect the Pod's phase and IP back onto the Environment status.
	// Only write a status update when the observed state actually changes to
	// avoid reconcile churn (a status write re-enqueues the object).
	previousPhase := environment.Status.Phase
	previousPodIP := environment.Status.PodIP

	switch pod.Status.Phase {
	case corev1.PodRunning:
		environment.Status.Phase = v1beta1.EnvironmentPhaseRunning
	case corev1.PodFailed:
		environment.Status.Phase = v1beta1.EnvironmentPhaseFailed
	default:
		environment.Status.Phase = v1beta1.EnvironmentPhasePending
	}

	environment.Status.PodIP = pod.Status.PodIP

	if environment.Status.Phase != previousPhase || environment.Status.PodIP != previousPodIP {
		if err := r.Status().Update(ctx, environment); err != nil {
			return ctrl.Result{}, err
		}
	}

	log.Info("Finished reconcile loop")

	return ctrl.Result{}, nil
}

// SetupWithManager determines which events the reconciler is called on.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Environment{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
