package listen

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/skpr/pinchy/servers/board"
)

// Options for the listen command.
type Options struct {
	Port             int
	OpencodeURL      string
	OpencodeWebURL   string
	OpencodePassword string
	EnvPorts         []int
	EnvDomain        string
	EnvScheme        string
	APIServer        string
}

// envOrDefault returns the value of the named environment variable, or fallback
// when it is unset or empty.
func envOrDefault(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// NewCommand will return a new Cobra command.
func NewCommand() *cobra.Command {
	o := Options{}

	cmd := &cobra.Command{
		Use:   "listen",
		Short: "Serve the kanban board and poll opencode for sessions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return board.Listen(cmd.Context(), board.ListenParams{
				Port:             o.Port,
				OpencodeURL:      o.OpencodeURL,
				OpencodeWebURL:   o.OpencodeWebURL,
				OpencodePassword: o.OpencodePassword,
				EnvPorts:         o.EnvPorts,
				EnvDomain:        o.EnvDomain,
				EnvScheme:        o.EnvScheme,
				APIServer:        o.APIServer,
			})
		},
	}

	cmd.Flags().IntVar(&o.Port, "port", 8090, "Port to serve the board on")
	cmd.Flags().StringVar(&o.OpencodeURL, "opencode-url", envOrDefault("OPENCODE_URL", "http://localhost:4096"), "Base URL of the opencode server (defaults to OPENCODE_URL env var)")
	cmd.Flags().StringVar(&o.OpencodeWebURL, "opencode-web-url", os.Getenv("OPENCODE_WEB_URL"), "Base URL the board links to for the opencode web UI (defaults to opencode-url)")
	cmd.Flags().StringVar(&o.OpencodePassword, "opencode-password", os.Getenv("OPENCODE_SERVER_PASSWORD"), "Optional basic-auth password for the opencode server")
	cmd.Flags().IntSliceVar(&o.EnvPorts, "env-ports", []int{8080, 3000}, "Ports each session's environment is linked on (served by pinchy-proxy)")
	cmd.Flags().StringVar(&o.EnvDomain, "env-domain", "pinchy.localhost", "Host suffix pinchy-proxy serves environments under")
	cmd.Flags().StringVar(&o.EnvScheme, "env-scheme", "http", "URL scheme for environment links")
	cmd.Flags().StringVar(&o.APIServer, "api-server", envOrDefault("PINCHY_API", "localhost:50051"), "Address of the pinchy-api gRPC server (defaults to PINCHY_API env var)")

	return cmd
}
