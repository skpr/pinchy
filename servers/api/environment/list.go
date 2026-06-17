package environment

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skpr/pinchy/proto/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// List returns a summary of every Environment in the configured namespace.
func (s *Server) List(ctx context.Context, _ *pb.ListRequest) (*pb.ListResponse, error) {
	list, err := s.clientset.PinchyV1beta1().Environments(s.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list environments: %v", err)
	}

	infos := make([]*pb.EnvironmentInfo, 0, len(list.Items))
	for i := range list.Items {
		env := &list.Items[i]
		infos = append(infos, &pb.EnvironmentInfo{
			Name:      env.Name,
			SessionID: env.Annotations["pinchy.dev/session-id"],
			Path:      env.Spec.Path,
			Phase:     string(env.Status.Phase),
			PodIP:     env.Status.PodIP,
			CreatedAt: env.CreationTimestamp.Unix(),
		})
	}

	return &pb.ListResponse{Environments: infos}, nil
}
