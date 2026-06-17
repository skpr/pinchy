package api

import (
	"fmt"
	"net"
	"os"

	"github.com/skpr/pinchy/proto/pb"
	"github.com/skpr/pinchy/servers/api/environment"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/skpr/pinchy/internal/clientset"
	"github.com/skpr/yolog"
)

type ListenParams struct {
	Port       int
	Reflect    bool
	Kubeconfig string
	Namespace  string
}

// Listen for requests.
func Listen(params ListenParams) error {
	logger := yolog.NewLogger("apiserver").SetAttrs(
		"port", params.Port,
		"reflection", params.Reflect)

	k8sconfig, err := clientcmd.BuildConfigFromFlags("", params.Kubeconfig)
	if err != nil {
		return logger.WrapError(err)
	}

	clientset, err := clientset.NewForConfig(k8sconfig)
	if err != nil {
		return logger.WrapError(err)
	}

	kubeClient, err := kubernetes.NewForConfig(k8sconfig)
	if err != nil {
		return logger.WrapError(err)
	}

	server := grpc.NewServer()

	pb.RegisterEnvironmentServer(server, environment.NewServer(
		clientset,
		kubeClient,
		k8sconfig,
		params.Namespace,
	))

	// Turn on reflection if the environment variable is set.
	if params.Reflect {
		reflection.Register(server)
	}

	// Log the services that are registered.
	services := []string{}
	for serviceName := range server.GetServiceInfo() {
		services = append(services, serviceName)
	}
	logger.SetAttr("services", services)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", params.Port))
	if err != nil {
		return logger.WrapError(err)
	}

	logger.Log(os.Stdout)

	err = server.Serve(listener)
	if err != nil {
		return logger.WrapError(err)
	}

	return nil
}
