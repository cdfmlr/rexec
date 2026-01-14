// Package rexec runs external commands locally or remotely.
// It wraps os/exec and golang.org/x/crypto/ssh with a simple interface.
//
// The key types are:
//   - Command: a struct that represents a command to run.
//   - Executor: an interface that runs a Command.
//     available executors are LocalExecutor, ShellExecutor,
//     ImmediateSshExecutor, and KeepAliveSshExecutor.
//   - ExecutorFactory: a struct that creates an Executor.
//     This is not necessary, you can create a literal Executor directly.
//
// Everything is designed to be friendly to marshal and unmarshal to/from JSON
// or other formats. Thus, basically all types are created with struct literals.
package rexec
