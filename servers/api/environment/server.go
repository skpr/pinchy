package environment

import (
	"github.com/skpr/pinchy/internal/clientset"
	"github.com/skpr/pinchy/proto/pb"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Server implements the GRPC "environment" definition.
type Server struct {
	pb.UnimplementedEnvironmentServer
	clientset  *clientset.Clientset
	kubeClient kubernetes.Interface
	restConfig *rest.Config
	namespace  string
}

// NewServer creates a new environment server.
func NewServer(clientset *clientset.Clientset, kubeClient kubernetes.Interface, restConfig *rest.Config, namespace string) *Server {
	return &Server{
		clientset:  clientset,
		kubeClient: kubeClient,
		restConfig: restConfig,
		namespace:  namespace,
	}
}
