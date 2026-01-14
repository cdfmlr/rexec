package rexec

import (
	"bytes"
	"fmt"
	"github.com/google/shlex"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
)

// Dangerous substrings that should not be present in the command, workdir, or env.
// These are used to prevent injection attacks.
var (
	WorkdirDangerous = []string{"\n", "\t", "\r", "\b", " ", ";", "&", "|", "<", ">", "`", "(", ")", "{", "}", "[", "]", "$", "~"}
	EnvDangerous     = WorkdirDangerous
	CommandDangerous = []string{":(){ :|:& };:"}
)

// Command is a command to run.
//
// It includes a command (with arguments joined by space),
// and optional workdir and env variables to set before running the command.
//
// Stdin, Stdout, and Stderr are the standard input, output, and error of the
// command.
// It is recommended to set all of them before running the command;
// otherwise, the behavior depends on the executor.
// The Validate() method will set the default values if they are nil.
//
// It is designed to do the same thing as following shell command:
//
//	cd Workdir && \
//	export Env.key=Env.value && \
//	Command < Stdin > Stdout 2> Stderr
//
// However, the actual behavior may vary depending on the executor.
type Command struct {
	// command to run on the remote host. with arguments joined by space.
	Command string
	// workdir is the working directory to run the command in.
	Workdir string
	// env is the environment variables to set for the command.
	Env map[string]string

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	Status int

	// executed is set to true after the command has been started.
	// This is used to prevent running the same command multiple times.
	started atomic.Bool
}

// Validate checks if the shellCmd is safe to run.
// It also sets the default Stdin, Stdout, and Stderr if they are nil.
//
// It returns an error if the command, workdir, or env contains dangerous
// substrings defined by WorkdirDangerous, EnvDangerous, or CommandDangerous.
func (e *Command) Validate() error {
	if e == nil {
		return ErrNilCommand
	}

	e.setDefaultStdio()

	if e.Command == "" {
		return ErrEmptyCommand
	}
	if e.Workdir != "" {
		if d, c := containsDangerous(e.Workdir, WorkdirDangerous); d {
			return fmt.Errorf("workdir (%q) %w: %q",
				e.Workdir, ErrContainsDangerous, c)
		}
	}
	for k, v := range e.Env {
		if d, c := containsDangerous(k, EnvDangerous); d {
			return fmt.Errorf("env key (%q=%q) %w: %q",
				k, v, ErrContainsDangerous, c)
		}
		if d, c := containsDangerous(v, EnvDangerous); d {
			return fmt.Errorf("env value (%q=%q) %w: %q",
				k, v, ErrContainsDangerous, c)
		}
	}
	if d, c := containsDangerous(e.Command, CommandDangerous); d {
		return fmt.Errorf("command (%q) %w: %q",
			e.Command, ErrContainsDangerous, c)
	}
	return nil
}

func (e *Command) setDefaultStdio() {
	if e.Stdin == nil {
		e.Stdin = bytes.NewReader([]byte{})
	}
	if e.Stdout == nil {
		e.Stdout = &bytes.Buffer{}
	}
	if e.Stderr == nil {
		e.Stderr = &bytes.Buffer{}
	}
}

// ShellString returns a combined command line to run on a shell,
// which cd to the workdir, sets the env variables, and runs the command:
//
//	"cd <workdir> && export <env_key>=<env_val> && export ... && <command>"
//
// It is recommended to call Validate() before calling this function
// to ensure the command is not injected.
func (e *Command) ShellString() string {
	if err := e.Validate(); err != nil {
		Logger.Error("calling String() on Validate failed shellCmd, this could be dangerous!!!",
			"err", err)
		// complain loudly, but still allow proceeding.
	}
	return e.cdWorkdirParts() + e.envVarsParts() + e.Command
}

// cdWorkdirParts returns the "cd <workdir> && " part of the ShellString.
func (e *Command) cdWorkdirParts() string {
	if e.Workdir == "" {
		return ""
	}
	return "cd " + e.Workdir + " && "
}

// envVarsParts returns the "export <env_key>=<env_val> && export <env_key>=<env_val> && ... &&" part
// of the ShellString.
func (e *Command) envVarsParts() string {
	if len(e.Env) == 0 {
		return ""
	}
	envs := make([]string, 0, len(e.Env))
	for k, v := range e.Env {
		export := fmt.Sprintf("export %s=%s &&", k, v)
		envs = append(envs, export)
	}
	return strings.Join(envs, " ") + " "
}

func (e *Command) LogValue() slog.Value {
	if e == nil {
		return slog.StringValue("<nil>")
	}
	return slog.GroupValue(
		slog.String("command", e.Command),
		slog.String("workdir", e.Workdir),
		slog.Any("env", e.Env),
		// slog.Int("status", e.Status),
	)
}

// containsDangerous returns true if s contains any of the dangerous characters.
// It also returns the first dangerous character found.
func containsDangerous(s string, dangerous []string) (bool, string) {
	for _, d := range dangerous {
		if strings.Contains(s, d) {
			return true, d
		}
	}
	return false, ""
}

// helper functions to convert fields of the shellCmd to slices for os/exec.

// cmdSlice converts a command string with arguments ("ls -a /usr") into
// a slice of fields (["ls", "-a", "/usr"]), preserving quoted substrings.
// For example,
//
//	"a b 'c d'" -> ["a", "b", "c d"]
func cmdSlice(s string) ([]string, error) {
	return shlex.Split(s)
}

// envSlice converts a map of environment variables ({"key": "value"}) to a
// slice of strings (["key=value"]).
func envSlice(env map[string]string) []string {
	envs := make([]string, 0, len(env))
	for k, v := range env {
		envs = append(envs, k+"="+v)
	}
	return envs
}

// errors

// shellCmd Validate() errors.
var (
	ErrEmptyCommand      = fmt.Errorf("command is empty")
	ErrContainsDangerous = fmt.Errorf("contains dangerous string")
)
