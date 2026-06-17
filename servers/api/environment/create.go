package environment

import (
	"context"
	"encoding/json"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/skpr/pinchy/apis/pinchy/v1beta1"
	"github.com/skpr/pinchy/internal/envname"
	"github.com/skpr/pinchy/proto/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// sessionLabelKey returns the label key used to record that a session is using
// this environment. Labels must be valid k8s label keys (63-char value limit,
// alphanumeric/-/_/. chars). We store the sanitised session ID in the key so
// all sessions for an environment are queryable without listing annotations.
func sessionLabelKey(sessionID string) string {
	return "pinchy.dev/session-" + RFC1123Subdomain(sessionID)
}

// Create a new environment, or reuse an existing one for the same workspace path.
//
// Environments are keyed by workspace path: all sessions operating in the same
// directory share a single Environment (and its Pod). When path is empty we
// fall back to keying by session ID, preserving backwards-compatible behaviour
// for callers that do not supply a workdir.
//
// Each session that uses the environment is recorded as a label
// "pinchy.dev/session-<sanitised-id>: true" so the board and cleanup logic
// can enumerate them.
func (s *Server) Create(ctx context.Context, req *pb.CreateRequest) (*pb.CreateResponse, error) {
	sessionID := req.GetEnvironment().GetSessionID()
	path := req.GetEnvironment().GetPath()

	// Derive the environment name: path-based when a workspace is given,
	// session-based otherwise (fallback for callers without --workdir).
	name := envname.FromPath(path)
	if name == "" {
		name = RFC1123Subdomain(sessionID)
	}

	desired := &v1beta1.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: s.namespace,
			Labels: map[string]string{
				sessionLabelKey(sessionID): "true",
			},
		},
		Spec: v1beta1.EnvironmentSpec{
			// Path is the host workspace directory mounted into the environment
			// Pod by the operator. It is fixed for the lifetime of the
			// environment — all sessions sharing a path share this value.
			Path: path,
		},
	}

	_, err := s.clientset.PinchyV1beta1().Environments(s.namespace).Create(ctx, desired, metav1.CreateOptions{})
	switch {
	case err == nil:
		// Freshly created — the session label is already on the object.

	case k8serrors.IsAlreadyExists(err):
		// The environment already exists (another session created it, or this
		// session is re-connecting). Record this session as a user of the
		// shared environment by merging the label in via a strategic-merge patch.
		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]string{
					sessionLabelKey(sessionID): "true",
				},
			},
		}
		patchBytes, err := json.Marshal(patch)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to marshal label patch: %v", err)
		}
		if _, err := s.clientset.PinchyV1beta1().Environments(s.namespace).Patch(
			ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{},
		); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to patch environment labels: %v", err)
		}

	default:
		return nil, status.Errorf(codes.Internal, "failed to create environment: %v", err)
	}

	// Check current phase before watching — the environment may already be
	// Running (e.g. on a second exec call from a different session sharing
	// this workspace), in which case no new watch event will fire and the
	// watch loop would block forever.
	current, err := s.clientset.PinchyV1beta1().Environments(s.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get environment: %v", err)
	}

	switch current.Status.Phase {
	case v1beta1.EnvironmentPhaseRunning:
		return &pb.CreateResponse{Phase: pb.CreateResponse_RUNNING}, nil
	case v1beta1.EnvironmentPhaseFailed:
		return &pb.CreateResponse{Phase: pb.CreateResponse_FAILED}, nil
	}

	// Not yet terminal — watch from the current resourceVersion so we don't
	// miss any transition that occurs between the Get above and the Watch below.
	watcher, err := s.clientset.PinchyV1beta1().Environments(s.namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector:   fields.OneTermEqualSelector("metadata.name", name).String(),
		ResourceVersion: current.ResourceVersion,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to watch environment: %v", err)
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		if event.Type == watch.Error {
			return nil, status.Errorf(codes.Internal, "watch error while waiting for environment")
		}

		environment, ok := event.Object.(*v1beta1.Environment)
		if !ok {
			continue
		}

		switch environment.Status.Phase {
		case v1beta1.EnvironmentPhaseRunning:
			return &pb.CreateResponse{Phase: pb.CreateResponse_RUNNING}, nil
		case v1beta1.EnvironmentPhaseFailed:
			return &pb.CreateResponse{Phase: pb.CreateResponse_FAILED}, nil
		}
	}

	// Watch channel closed without a terminal phase (e.g. context cancelled).
	return &pb.CreateResponse{Phase: pb.CreateResponse_UNKNOWN}, nil
}
