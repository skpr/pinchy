package environment

import (
	"context"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skpr/pinchy/proto/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Delete removes a named Environment and its owned Pod (via owner references).
func (s *Server) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	err := s.clientset.PinchyV1beta1().Environments(s.namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "environment %q not found", name)
		}
		return nil, status.Errorf(codes.Internal, "failed to delete environment: %v", err)
	}

	return &pb.DeleteResponse{}, nil
}
