package environment

import (
	"context"
	"regexp"
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/skpr/pinchy/apis/pinchy/v1beta1"
	"github.com/skpr/pinchy/proto/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Create a new environment.
func (s *Server) Create(ctx context.Context, req *pb.CreateRequest) (*pb.CreateResponse, error) {
	sessionID := req.GetEnvironment().GetSessionID()

	// Sanitize the session ID once; use it everywhere to ensure the field
	// selector, Create, Get, and Watch all refer to the same object name.
	name := RFC1123Subdomain(sessionID)

	desired := &v1beta1.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: s.namespace,
			Annotations: map[string]string{
				"pinchy.dev/session-id": sessionID,
			},
		},
		Spec: v1beta1.EnvironmentSpec{
			// Path is the host workspace directory the session is working in.
			// The operator mounts it into the environment Pod. Environments are
			// create-only, so this is fixed for the lifetime of the session.
			Path: req.GetEnvironment().GetPath(),
		},
	}

	_, err := s.clientset.PinchyV1beta1().Environments(s.namespace).Create(ctx, desired, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return nil, status.Errorf(codes.Internal, "failed to create environment: %v", err)
	}

	// Check current phase before watching — the environment may already be
	// Running (e.g. on a second exec call), in which case no new watch event
	// will fire and the watch loop would block forever.
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

// matches any run of characters that are NOT lowercase alphanumeric, '-' or '.'
var invalidChars = regexp.MustCompile(`[^a-z0-9.-]+`)

// matches leading/trailing characters that aren't alphanumeric
var trimEdges = regexp.MustCompile(`^[^a-z0-9]+|[^a-z0-9]+$`)

// RFC1123Subdomain converts s into a string that satisfies the
// RFC 1123 subdomain rules used by Kubernetes metadata.name:
//   - lower case alphanumeric, '-' or '.'
//   - must start and end with an alphanumeric character
//   - at most 253 characters
func RFC1123Subdomain(s string) string {
	s = strings.ToLower(s)
	// replace any invalid character (incl. '_') with '-'
	s = invalidChars.ReplaceAllString(s, "-")
	// strip any leading/trailing non-alphanumeric chars
	s = trimEdges.ReplaceAllString(s, "")

	if len(s) > 253 {
		s = s[:253]
		// trimming may have left a trailing separator; clean it up
		s = trimEdges.ReplaceAllString(s, "")
	}
	return s
}
