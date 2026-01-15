package rexec

import (
	"context"
	"errors"
	"fmt"
	osexec "os/exec"

	"golang.org/x/crypto/ssh"
)

// Executor executes given command.
type Executor interface {
	// Execute implements the execution of a command.
	//
	// A typical implementation of Execute should:
	//  0. Fast fail if the context is done.
	//  1. Fast fail if the command is nil.
	//  2. Fast fail if the command has already been executed.
	//  3. Set the command status to -1.
	//  4. Validate the command.
	//  5. Prepare the command, make the proc/client/session/... to execute the command.
	//  6. Start the command in another goroutine.
	//  7. Wait for the command to finish in the main goroutine.
	//  8. set status (exit code) of the command. (prefer to do this in a defer statement placing at 3~5 as early as possible)
	//  9. return the error.
	Execute(ctx context.Context, cmd *Command) error
}

// LocalExecutor runs command with os/exec on the local machine.
type LocalExecutor struct{}

var _ Executor = (*LocalExecutor)(nil)

func (e *LocalExecutor) Execute(ctx context.Context, cmd *Command) error {
	logger := Logger.With("field", "rexec.LocalExecutor.Execute", "cmd", cmd)

	if err := ctx.Err(); err != nil {
		logger.Info("skipping execution: context done", "ctxErr", err)
		return err
	}

	if cmd == nil {
		logger.Warn("reject execution: nil command")
		return ErrNilCommand
	}

	if !cmd.started.CompareAndSwap(false, true) {
		// compare-and-swap return true for the first call
		// and false for later calls.
		logger.Warn("reject execution: command already started")
		return ErrStartedCommand
	}

	logger.Debug("executing command")

	cmd.Status = -1

	if err := cmd.Validate(); err != nil {
		logger.Warn("reject execution: invalid command", "err", err)
		return fmt.Errorf("%w: %w", ErrInvalidCommand, err)
	}

	// we don't rely on the ShellString() here,
	// see proc.Dir and proc.Env below.
	cmdStr := cmd.Command

	// Execute the command
	// os/exec needs the command and its arguments to be separate
	// so that the command can be looked up in the PATH correctly.
	cmdParts, err := cmdSlice(cmdStr)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrParseCommand, err)
	}
	proc := osexec.Command(cmdParts[0], cmdParts[1:]...)

	defer func() {
		if proc != nil && proc.ProcessState != nil {
			cmd.Status = proc.ProcessState.ExitCode()
			logger.Debug("command finished. setting status", "status", cmd.Status)
		} else {
			cmd.Status = -1
			logger.Warn("failed to get exit code of the command. setting default -1")
		}
	}()

	// the working directory and environment variables
	// are set on the process directly.
	proc.Dir = cmd.Workdir
	proc.Env = envSlice(cmd.Env)
	if len(proc.Env) == 0 {
		proc.Env = nil // use the parent process's environment
	}

	proc.Stdin = cmd.Stdin
	proc.Stdout = cmd.Stdout
	proc.Stderr = cmd.Stderr

	logger.Debug("os/exec.Cmd is ready to take off", "proc", proc.String())

	err = runProc(ctx, proc)
	if err != nil {
		logger.Warn("command execution failed", "err", err)
	} else {
		logger.Info("command execution succeeded", "status", cmd.Status)
	}

	return err
}

// ShellExecutor is an Executor that runs commands on a
// local `shell -c` or ssh command.
//
//   - sh <sh-args> -c "command args..."
//   - ssh <ssh-args> "command args..."
//
// The shell will be run with os/exec.
type ShellExecutor struct {
	ShellPath string
	ShellArgs []string
}

var _ Executor = (*ShellExecutor)(nil)

func (e *ShellExecutor) Execute(ctx context.Context, cmd *Command) error {
	logger := Logger.With("field", "rexec.ShellExecutor.Execute", "cmd", cmd)

	if err := ctx.Err(); err != nil {
		logger.Info("skipping execution: context done", "ctxErr", err)
		return err
	}

	if cmd == nil {
		logger.Warn("reject execution: nil command")
		return ErrNilCommand
	}

	if !cmd.started.CompareAndSwap(false, true) {
		// compare-and-swap return true for the first call
		// and false for later calls.
		logger.Warn("reject execution: command already started")
		return ErrStartedCommand
	}

	cmd.Status = -1

	if err := cmd.Validate(); err != nil {
		logger.Warn("reject execution: invalid command", "err", err)
		return fmt.Errorf("%w: %w", ErrInvalidCommand, err)
	}

	cmdStr := cmd.ShellString()

	// Execute the command
	proc := osexec.Command(e.ShellPath, append(e.ShellArgs, cmdStr)...)

	defer func() {
		if proc != nil && proc.ProcessState != nil {
			cmd.Status = proc.ProcessState.ExitCode()
			logger.Debug("command finished. setting status", "status", cmd.Status)
		} else {
			cmd.Status = -1
			logger.Warn("failed to get exit code of the command. setting default -1")
		}
	}()

	// It is WRONG to set dir and env here.
	// we exec the shell from cwd & env of the parent process.
	// the Workdir & Env of the command are set in the ShellString().
	//
	// proc.Dir = cmd.Workdir
	// proc.Env = envSlice(cmd.Env)

	proc.Stdin = cmd.Stdin
	proc.Stdout = cmd.Stdout
	proc.Stderr = cmd.Stderr

	logger.Debug("os/exec.Cmd is ready to take off", "proc", proc.String())

	err := runProc(ctx, proc)

	if err != nil {
		logger.Warn("command execution failed", "err", err)
	} else {
		logger.Info("command execution succeeded", "status", cmd.Status)
	}

	return err
}

// runProc starts the os/exec process and waits for it to finish or
// the context to be done.
func runProc(ctx context.Context, proc *osexec.Cmd) error {
	logger := Logger.With("field", "rexec.runProc", "proc", proc.String())

	if proc == nil {
		return fmt.Errorf("%w: nil process", ErrInternalError)
	}

	if err := proc.Start(); err != nil {
		logger.Error("failed to start process", "err", err)
		return err
	}

	done := make(chan error)
	go func() {
		done <- proc.Wait()
		logger.Debug("process finished")
	}()

	select {
	case <-ctx.Done():
		err := ctx.Err()
		logger.Debug("context done, killing process", "ctxErr", err)
		_ = proc.Process.Kill()
		return err
	case err := <-done:
		logger.Debug("process done", "exitErr", err)
		return err
	}
}

// ImmediateSshExecutor is an SSH Executor based on golang.org/x/crypto/ssh
// that dials the remote host immediately each time it is called to Execute(cmd)
// and closes the connection immediately after the command is finished.
//
// It's safe to reuse the same ImmediateSshExecutor for multiple commands
// concurrently.
// But keep in mind that the connections won't be reused between commands.
type ImmediateSshExecutor struct {
	Config *SshClientConfig
}

var _ Executor = (*ImmediateSshExecutor)(nil)

func (e *ImmediateSshExecutor) Execute(ctx context.Context, cmd *Command) error {
	logger := Logger.With("field", "rexec.ImmediateSshExecutor.Execute", "cmd", cmd)

	var err error // Avoid shadowing, use this as the return value

	if err = ctx.Err(); err != nil {
		logger.Info("skipping execution: context done", "ctxErr", err)
		return err
	}

	if err = validateSshClientConfig(e.Config); err != nil {
		logger.Warn("reject execution: bad SSH client config", "err", err)
		return fmt.Errorf("%w: %w", ErrBadSshConfig, err)
	}

	if cmd == nil {
		logger.Warn("reject execution: nil command")
		return ErrNilCommand
	}

	if !cmd.started.CompareAndSwap(false, true) {
		// compare-and-swap return true for the first call
		// and false for later calls.
		logger.Warn("reject execution: command already started")
		return ErrStartedCommand
	}

	cmd.Status = -1

	// after this deferring, ANY return path should set error to the `err`
	// variable. do not `return someFunc()` directly!!
	defer func() {
		var sshExitError *ssh.ExitError

		switch {
		case err == nil: // no error, command exited successfully
			cmd.Status = 0
		case errors.As(err, &sshExitError):
			cmd.Status = sshExitError.ExitStatus()
		default:
			cmd.Status = -1
		}

		logger.Debug("command finished. setting status based on err", "status", cmd.Status, "err", err)
	}()

	if err = cmd.Validate(); err != nil {
		logger.Warn("reject execution: invalid command", "err", err)
		return err
	}

	client, err := dialSsh(e.Config)
	if err != nil {
		logger.Warn("failed to dial SSH client", "err", err)
		return err
	}
	defer func(client *ssh.Client) {
		_ = client.Close()
	}(client)

	err = execWithSshClient(ctx, cmd, client)

	if err != nil {
		logger.Warn("command execution failed", "err", err)
	} else {
		logger.Info("command execution succeeded", "err", err)
	}

	return err
}

// KeepAliveSshExecutor is an SSH Executor based on golang.org/x/crypto/ssh
// that dials the remote host once and keeps the connection alive until the
// executor is Closed.
//
// It creates a new session for each command to execute.
// It's safe to reuse the same KeepAliveSshExecutor for multiple commands
// concurrently.
type KeepAliveSshExecutor struct {
	Config *SshClientConfig

	ka *keepAliveSshClient
}

var _ Executor = (*KeepAliveSshExecutor)(nil)

// init initializes the keep-alive SSH client based on the configuration.
func (e *KeepAliveSshExecutor) init() {
	e.ka = &keepAliveSshClient{
		SshClientConfig: e.Config,
	}
}

// Execute the command on the SSH client.
//
// It will dial the remote host if the connection is not established yet.
// Or it will reuse the existing keeping-alive connection.
// New session will be created (within the same connection) for each command.
//
// The connection will be kept alive until Close() is called.
func (e *KeepAliveSshExecutor) Execute(ctx context.Context, cmd *Command) error {
	logger := Logger.With("field", "rexec.KeepAliveSshExecutor.Execute", "cmd", cmd)

	if err := validateSshClientConfig(e.Config); err != nil {
		logger.Warn("reject execution: bad SSH client config", "err", err)
		return fmt.Errorf("%w: %w", ErrBadSshConfig, err)
	}

	if e.ka == nil {
		logger.Info("initializing keep-alive SSH client")
		e.init()
	}
	if e.ka == nil { // should not happen
		// panic("failed to initialize keep-alive SSH client")
		logger.Error("got nil keep-alive SSH client after init it. this should not happen.")
		return fmt.Errorf("%w: %w", ErrInternalError, errors.New("failed to initialize keep-alive SSH client"))
	}

	var err error // Avoid shadowing, use this as the return value

	if err = ctx.Err(); err != nil {
		logger.Info("skipping execution: context done", "ctxErr", err)
		return err
	}

	if cmd == nil {
		logger.Warn("reject execution: nil command")
		return ErrNilCommand
	}

	if !cmd.started.CompareAndSwap(false, true) {
		// compare-and-swap return true for the first call
		// and false for later calls.
		logger.Warn("reject execution: command already started")
		return ErrStartedCommand
	}

	cmd.Status = -1

	// after this deferring, ANY return path should set error to the `err`
	// variable. do not `return someFunc()` directly!!
	defer func() {
		var sshExitError *ssh.ExitError

		switch {
		case err == nil: // no error, command exited successfully
			cmd.Status = 0
		case errors.As(err, &sshExitError):
			cmd.Status = sshExitError.ExitStatus()
		default:
			cmd.Status = -1
		}
		logger.Debug("command finished. setting status based on err", "status", cmd.Status, "err", err)
	}()

	if err = cmd.Validate(); err != nil {
		logger.Warn("reject execution: invalid command", "err", err)
		return err
	}

	var client *ssh.Client
	client, err = e.ka.Client()
	if err != nil {
		logger.Warn("failed to get SSH client", "err", err)
		return err
	}

	err = execWithSshClient(ctx, cmd, client)

	if err != nil {
		logger.Warn("command execution failed", "err", err)
	} else {
		logger.Info("command execution succeeded", "err", err)
	}

	return err
}

// Close the SSH client and stops the keep-alive loop.
func (e *KeepAliveSshExecutor) Close() error {
	if e.ka == nil {
		return nil
	}
	err := e.ka.Close()
	e.ka = nil
	return err
}

// execWithSshClient is a subroutine shared by ImmediateSshExecutor.Execute and
// KeepAliveSshExecutor.Execute.
//
// execWithSshClient creates a new session in the given client and
// executes the validated command on the session.
//
// requirements:
//   - the given cmd must be validated (Command.Validate()).
//   - the given client must be dialed and ready to use.
//
// Blocks until the command is finished or the context is done.
func execWithSshClient(ctx context.Context, cmd *Command, client *ssh.Client) error {
	logger := Logger.With("field", "rexec.execWithSshClient", "cmd", cmd, "client", sshClientString(client))

	if client == nil {
		return fmt.Errorf("%w: nil ssh client", ErrInternalError)
	}
	if cmd == nil {
		return ErrNilCommand
	}

	session, err := client.NewSession()
	if err != nil {
		logger.Warn("failed to create SSH session", "err", err)
		return err
	}
	defer func(session *ssh.Session) {
		closeErr := session.Close()
		logger.Debug("close SSH session", "closeErr", closeErr)
	}(session)

	session.Stdin = cmd.Stdin
	session.Stdout = cmd.Stdout
	session.Stderr = cmd.Stderr

	cmdStr := cmd.ShellString()

	logger.Debug("executing command on SSH session", "cmd", cmdStr, "session", fmt.Sprintf("%p", session))

	err = runSshSession(ctx, session, cmdStr)
	return err
}

// runSshSession run the given command on the SSH session.
// Blocks until the command is finished or the context is done.
func runSshSession(ctx context.Context, session *ssh.Session, cmdStr string) error {
	logger := Logger.With("field", "rexec.runSshSession", "cmd", cmdStr, "session", fmt.Sprintf("%p", session))

	if session == nil {
		return fmt.Errorf("%w: nil session", ErrInternalError)
	}
	if cmdStr == "" {
		return fmt.Errorf("%w: empty command", ErrParseCommand)
	}

	var err error
	if err = session.Start(cmdStr); err != nil {
		logger.Warn("failed to start command on SSH session", "err", err)
		return err
	}

	done := make(chan error)
	go func() {
		done <- session.Wait()
		logger.Debug("command finished on SSH session")
	}()

	select {
	case <-ctx.Done():
		killErr := session.Signal(ssh.SIGKILL)
		err = ctx.Err()
		logger.Debug("context done, killing command on SSH session", "ctxErr", err, "killErr", killErr)
	case err = <-done:
		logger.Debug("command done on SSH session", "exitErr", err)
	}
	return err
}

// errors that Executor.Execute may return.
var (
	ErrNilCommand     = errors.New("nil command")
	ErrParseCommand   = errors.New("failed to parse command")
	ErrInvalidCommand = errors.New("invalid command")
	ErrStartedCommand = errors.New("command has already been executed")
	ErrBadSshConfig   = errors.New("bad SSH client configuration")
	ErrInternalError  = errors.New("internal error") // should not happen, means a bug of code logic
)
