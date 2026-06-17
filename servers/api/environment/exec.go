package environment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/exec"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/skpr/pinchy/proto/pb"
)

// Exec runs a command inside the environment's Pod and returns buffered stdout/stderr.
func (s *Server) Exec(ctx context.Context, req *pb.ExecRequest) (*pb.ExecResponse, error) {
	if req.GetSessionID() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "sessionID is required")
	}

	if req.GetCommand() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "command is required")
	}

	// Run the command through a shell so that the full range of shell syntax
	// (arguments, pipes, redirects, &&, env vars, etc.) works as expected.
	// The Pod name equals the sanitized sessionID (see controllers/environment/controller.go).
	command := req.GetCommand()

	// If a working directory was requested, change into it first. The workspace
	// is mounted at the same path inside the Pod, so this matches the directory
	// the caller (e.g. the opencode session) is operating in.
	if workdir := req.GetWorkdir(); workdir != "" {
		command = fmt.Sprintf("cd %s && %s", shellQuote(workdir), command)
	}

	execURL := s.kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(RFC1123Subdomain(req.GetSessionID())).
		Namespace(s.namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "environment",
			Command:   []string{"/bin/sh", "-c", command},
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec).
		URL()

	executor, err := remotecommand.NewSPDYExecutor(s.restConfig, "POST", execURL)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create executor: %v", err)
	}

	var stdout, stderr bytes.Buffer

	streamErr := executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	// A non-zero exit code from the remote command is surfaced as a CodeExitError.
	// Treat it as a successful RPC — return the exit code in the response.
	var exitCode int32
	if streamErr != nil {
		var codeExitErr exec.CodeExitError
		if errors.As(streamErr, &codeExitErr) {
			exitCode = int32(codeExitErr.ExitStatus())
		} else {
			return nil, status.Errorf(codes.Internal, "exec failed: %v", streamErr)
		}
	}

	return &pb.ExecResponse{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: exitCode,
	}, nil
}

// shellQuote wraps s in single quotes so it is treated as a single literal
// argument by /bin/sh, escaping any embedded single quotes. This prevents a
// workspace path from being interpreted as shell syntax.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
