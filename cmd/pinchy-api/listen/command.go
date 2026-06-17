package listen

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/skpr/pinchy/servers/api"
)

// Options for the watch command.
type Options struct {
	Port       int
	Reflection bool
	Kubeconfig string
	Namespace  string
}

// NewCommand will return a new Cobra command.
func NewCommand() *cobra.Command {
	o := Options{}

	cmd := &cobra.Command{
		Use:   "listen",
		Short: "Listen on a port for incoming requests.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return api.Listen(api.ListenParams{
				Port:       o.Port,
				Reflect:    o.Reflection,
				Kubeconfig: o.Kubeconfig,
				Namespace:  o.Namespace,
			})
		},
	}

	cmd.Flags().IntVar(&o.Port, "port", 50051, "Port of the API server")
	cmd.Flags().BoolVar(&o.Reflection, "reflection", true, "Enable gRPC reflection")
	cmd.Flags().StringVar(&o.Kubeconfig, "kubeconfig", os.Getenv("KUBECONFIG"), "Path to kubeconfig file (defaults to KUBECONFIG env var)")
	cmd.Flags().StringVar(&o.Namespace, "environment-namespace", "pinchy-environment", "Namespace to use for environments")

	return cmd
}
