package listen

import (
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/skpr/pinchy/servers/proxy"
)

// Options for the listen command.
type Options struct {
	Ports      []int
	Kubeconfig string
	Namespace  string
	Interval   time.Duration
}

// NewCommand will return a new Cobra command.
func NewCommand() *cobra.Command {
	o := Options{}

	cmd := &cobra.Command{
		Use:   "listen",
		Short: "Listen for incoming requests and proxy them to environments.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return proxy.Listen(cmd.Context(), proxy.ListenParams{
				Ports:      o.Ports,
				Kubeconfig: o.Kubeconfig,
				Namespace:  o.Namespace,
				Interval:   o.Interval,
			})
		},
	}

	cmd.Flags().IntSliceVar(&o.Ports, "ports", []int{8080, 3000}, "Ports to listen on. Each port is also the destination Pod port")
	cmd.Flags().StringVar(&o.Kubeconfig, "kubeconfig", os.Getenv("KUBECONFIG"), "Path to kubeconfig file (defaults to KUBECONFIG env var)")
	cmd.Flags().StringVar(&o.Namespace, "environment-namespace", "pinchy-environment", "Namespace to use for environments")
	cmd.Flags().DurationVar(&o.Interval, "interval", proxy.DefaultInterval, "How often to poll for environment changes")

	return cmd
}
