# rexec

[![GoDoc](https://pkg.go.dev/badge/github.com/cdfmlr/rexec/v2)](https://pkg.go.dev/github.com/cdfmlr/rexec/v2)

Run external commands locally or over SSH through a small, config-friendly API. `rexec` wraps `os/exec` and `golang.org/x/crypto/ssh` executors, plus a factory to choose between them.

## Features

- Local execution with `os/exec` (no shell required)
- Shell-based execution (`sh -c`) when you need shell semantics
- SSH execution: immediate connect-per-command or keep-alive reusable sessions
- Pluggable factory (`ExecutorFactory`) for config-driven executor selection
- Safe defaults: command validation, opt-in logging, JSON-friendly structs

## Install

```bash
go get github.com/cdfmlr/rexec/v2
```

## Quick start

Run a local command:

```go
package main

import (
    "bytes"
    "context"
    "fmt"

    "github.com/cdfmlr/rexec/v2"
)

func main() {
    ctx := context.Background()

    stdout := &bytes.Buffer{}
    cmd := &rexec.Command{Command: "echo hello", Stdout: stdout}

    exec := &rexec.LocalExecutor{}
    if err := exec.Execute(ctx, cmd); err != nil {
        panic(err)
    }

    fmt.Println("exit status:", cmd.Status)
    fmt.Println("stdout:", stdout.String())
}
```

Use a shell (e.g., `/bin/sh -c`) when you need shell features:

```go
exec := &rexec.ShellExecutor{ShellPath: "/bin/sh", ShellArgs: []string{"-c"}}
cmd := &rexec.Command{Command: "echo $EDITOR", Stdout: os.Stdout}
_ = exec.Execute(context.Background(), cmd)
```

### SSH execution

Immediate (connect per command):

```go
cfg := &rexec.SshClientConfig{
    Addr: "example.com:22",
    User: "root",
    Auth: []rexec.SshAuth{{Password: "secret"}},
}
stdout := &bytes.Buffer{}
cmd := &rexec.Command{Command: "hostname", Stdout: stdout}
exec := &rexec.ImmediateSshExecutor{Config: cfg}
_ = exec.Execute(context.Background(), cmd)
```

Since v2.3.0, rexec enforces SSH host key checking by default (for security).
The target host key must be in `/etc/ssh/ssh_known_hosts` or `~/ssh/known_hosts`,
or specified it via custom `HostKeyCheck` in `SshClientConfig`.
To disable it (not recommended), set `cfg.HostKeyCheck = &rexec.SshHostKeyCheckConfig{ InsecureIgnore: true }`.

Keep-alive (connection reused across commands):

```go
ka := &rexec.KeepAliveSshExecutor{Config: cfg}
defer ka.Close()
cmd := &rexec.Command{Command: "uptime", Stdout: stdout}
_ = ka.Execute(context.Background(), cmd)
```

### Factory usage

Pick exactly one configured executor; the factory returns it or errors if misconfigured:

```go
factory := rexec.ExecutorFactory{
    Shell: &rexec.ShellExecutor{ShellPath: "/bin/sh", ShellArgs: []string{"-c"}},
}
exec, err := factory.Executor()
if err != nil { /* handle */ }
_ = exec.Execute(context.Background(), &rexec.Command{Command: "date"})
```

### Configurations from JSON/YAML

All the structs in `rexec` is designed to be JSON/YAML serializable.
So you can load configurations from config files directly into `rexec` 
structs easily, no extra wrappers needed.

Example JSON config for `ExecutorFactory`:

```go
	// Create an ExecutorFactory with ShellExecutor
executorJson := []byte(`{
		"Shell": {
			"ShellPath": "/usr/bin/bc",
			"ShellArgs": ["--expression"]
        }
	}`)
// And a Command
commandJson := []byte(`{
		"Command": "1+1"
	}`)

var f rexec.ExecutorFactory
_ = json.Unmarshal(executorJson, &f)

// Get the Executor
executor, _ := f.Executor()
defer executor.Close()

// Unmarshal Command
command := new(rexec.Command)
_ = json.Unmarshal(commandJson, &command)
command.Stdout = os.Stdout

// Use the executor to run the command
_ = executor.Execute(context.Background(), command)
```

### Validation & safety

`Command.Validate()` rejects empty commands and common dangerous substrings in command, workdir, and env. Always set `Command` fields via struct literals; avoid interpolating untrusted input without validation.

### Logging

Logging is disabled by default. To enable slog-based logging:

```go
rexec.Logger = slog.Default().With("pkg", "rexec")
```

Set `useDebugLogger` in `logger.go` for built-in debug output during development.

### Testing SSH (dev only)

To run rexec tests involving SSH, we need to spin up a test SSH server (sshd):

- Listening on `localhost:24622`
- Accepting username `root` with password `root` or private key `./testsshd/testsshd.id_rsa`

As v2.1.0 onwards, `rexec` includes two setups for the testsshd server:

1. `"internal"`: an in-process SSH server implemented in Go, located at `internal/testsshd`. 
   This server is lightweight and does not require Docker. 
   See `internal/testsshd/README.md` for usage details.
2. `"docker"`: A Docker-based SSH server located at `testsshd/`. 
   This is a more realistic SSH server setup for testing purposes.

It is set to try the `"docker"` testsshd server by default in tests. 
(We actually just try to connect to `localhost:24622`, if it works, we use it. 
We don't really care whether it's a container or not, so you can forward the 
port to any sshd server you like, as long as it accepts the test credentials.)
If the `localhost:24622` service is not available,
it falls back to the `"internal"` testsshd service which should always work
(but may not as realistic as a real OpenSSH server in the container).

To start the Docker-based testsshd server,
Make sure you have Docker installed and running, then:

```bash
cd testsshd
docker compose -f testsshd-docker-compose.yml up
```

To test the testsshd service itself, run the following test:

```bash
go test -test.run=Test_testsshd -v .
```

See `testsshd/README.md` for details.

## Documentation

See the GoDoc page: https://pkg.go.dev/github.com/cdfmlr/rexec/v2

## Version

Current major version: v2. 
Always import `github.com/cdfmlr/rexec/v2`.

## License

MIT OR Apache-2.0 (choose at your option)
