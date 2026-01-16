//go:build exclude

package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cdfmlr/rexec/v2"
	"github.com/cdfmlr/rexec/v2/internal/testsshd"
)

func main() {
	// Set up logger for rexec package
	rexec.Logger = slog.Default().With("test", "rexec/example/ssh")

	// A fake SSH server for testing
	sshd, err := testsshd.New(&testsshd.Config{
		Users: []testsshd.User{{Username: "foo", Password: "bar"}},
	})
	if err != nil {
		panic(err)
	}

	cfg := &rexec.SshClientConfig{
		Addr: sshd.Addr(),
		User: "foo",
		Auth: []rexec.SshAuth{{Password: "bar"}},
		HostKeyCheck: &rexec.SshHostKeyCheckConfig{
			InsecureIgnore: true,
		},
	}

	io := rexec.NewManagedIO()

	cmd := &rexec.Command{Command: "echo hello"}
	io.Hijack(cmd)

	err = cmd.Validate()
	if err != nil {
		panic(err)
	}

	ssh := &rexec.ImmediateSshExecutor{Config: cfg}

	ctx := context.Background()
	err = ssh.Execute(ctx, cmd)
	if err != nil {
		panic(err)
	}

	fmt.Println("exit status:", cmd.Status)
	fmt.Println("stdout:", io.Stdout.String())

	// Output:
	// exit status: 0
	// stdout: hello
}
