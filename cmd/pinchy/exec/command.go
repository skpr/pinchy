package exec

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/skpr/pinchy/proto/pb"
)

// Options for the exec command.
type Options struct {
	Session string
	Server  string
	Workdir string
}

// NewCommand will return a new Cobra command.
func NewCommand() *cobra.Command {
	o := Options{}

	cmd := &cobra.Command{
		Use:   "exec --session <session> -- <command> [args...]",
		Short: "Execute a command in the environment.",
		// Arguments after -- are the command to run.
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if o.Session == "" {
				return fmt.Errorf("--session is required")
			}

			// ArgsLenAtDash returns -1 if no -- was provided; in that case
			// treat all positional args as the command.
			command := args
			if i := cmd.ArgsLenAtDash(); i >= 0 {
				command = args[i:]
			}

			if len(command) == 0 {
				return fmt.Errorf("a command to execute is required (pass it after --)")
			}

			conn, err := grpc.NewClient(o.Server, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return fmt.Errorf("failed to connect to API server: %w", err)
			}
			defer conn.Close()

			client := pb.NewEnvironmentClient(conn)
			ctx := context.Background()

			// Create (or reconcile) the environment and wait for it to be running.
			createResp, err := client.Create(ctx, &pb.CreateRequest{
				Environment: &pb.Environment{
					SessionID: o.Session,
					Path:      o.Workdir,
				},
			})
			if err != nil {
				return fmt.Errorf("failed to create environment: %w", err)
			}

			if createResp.GetPhase() == pb.CreateResponse_FAILED {
				return fmt.Errorf("environment %q failed to start", o.Session)
			}

			// Execute the command inside the environment.
			// Join post-'--' args with spaces to form a shell command line;
			// the server runs it via /bin/sh -c so pipes, redirects, etc. work.
			execResp, err := client.Exec(ctx, &pb.ExecRequest{
				SessionID: o.Session,
				Command:   strings.Join(command, " "),
				Workdir:   o.Workdir,
			})
			if err != nil {
				return fmt.Errorf("exec failed: %w", err)
			}

			if len(execResp.GetStdout()) > 0 {
				os.Stdout.Write(execResp.GetStdout())
			}

			if len(execResp.GetStderr()) > 0 {
				os.Stderr.Write(execResp.GetStderr())
			}

			if execResp.GetExitCode() != 0 {
				os.Exit(int(execResp.GetExitCode()))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&o.Session, "session", "", "Name of the environment session")
	cmd.Flags().StringVar(&o.Server, "server", envOrDefault("PINCHY_API", "localhost:50051"), "Address of the Pinchy API server")
	cmd.Flags().StringVar(&o.Workdir, "workdir", "", "Host workspace path to mount and run the command in")

	return cmd
}

// envOrDefault returns the value of the named environment variable, or the
// provided default if the variable is unset or empty.
func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return def
}
