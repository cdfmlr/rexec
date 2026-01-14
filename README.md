# rexec

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

## SSH execution

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

Keep-alive (connection reused across commands):

```go
ka := &rexec.KeepAliveSshExecutor{Config: cfg}
defer ka.Close()
cmd := &rexec.Command{Command: "uptime", Stdout: stdout}
_ = ka.Execute(context.Background(), cmd)
```

## Factory usage

Pick exactly one configured executor; the factory returns it or errors if misconfigured:

```go
factory := rexec.ExecutorFactory{
    Shell: &rexec.ShellExecutor{ShellPath: "/bin/sh", ShellArgs: []string{"-c"}},
}
exec, err := factory.Executor()
if err != nil { /* handle */ }
_ = exec.Execute(context.Background(), &rexec.Command{Command: "date"})
```

## Configurations from JSON/YAML

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

## Validation & safety

`Command.Validate()` rejects empty commands and common dangerous substrings in command, workdir, and env. Always set `Command` fields via struct literals; avoid interpolating untrusted input without validation.

## Logging

Logging is disabled by default. To enable slog-based logging:

```go
rexec.Logger = slog.Default().With("pkg", "rexec")
```

Set `useDebugLogger` in `logger.go` for built-in debug output during development.

## Testing SSH helpers (dev only)

To run rexec tests involving SSH, we need to spin up a test SSH server
that defined in `testsshd/`.

It relies on Docker Compose to start the service on localhost:24622. 
Make sure you have Docker installed and running, then:

```bash
cd testsshd
docker compose -f testsshd-docker-compose.yml up
```

See `testsshd/README.md` for details.

## Version

Current major version: v2. 
Always import `github.com/cdfmlr/rexec/v2`.

## License

MIT OR Apache-2.0 (choose at your option)
