package rexec

import (
	"bytes"
)

// ManagedIO is a bundle of bytes.Buffer that can be used as the standard input,
// output, and error of a Command.
//
// The zero value for ManagedIO is NOT ready to use.
// Use NewManagedIO or NewCombinedOutputManagedIO to create a correct instance,
// or assign the buffers manually (never nil) before using it.
type ManagedIO struct {
	Stdin  *bytes.Buffer
	Stdout *bytes.Buffer
	Stderr *bytes.Buffer
}

// NewManagedIO creates a new ManagedIO with empty buffers
// for Stdin, Stdout, and Stderr respectively.
func NewManagedIO() *ManagedIO {
	return &ManagedIO{
		Stdin:  &bytes.Buffer{},
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
}

// Deprecated: this is buggy. The output maybe lost. Do not use it.
//
// NewCombinedOutputManagedIO creates a new ManagedIO with a single buffer
// for both Stdout and Stderr.
func NewCombinedOutputManagedIO() *ManagedIO {
	inBuf := &bytes.Buffer{}
	outBuf := &bytes.Buffer{}

	return &ManagedIO{
		Stdin:  inBuf,
		Stdout: outBuf,
		Stderr: outBuf,
	}
}

// Deprecated: use Hijack instead.
//
// manageCmd overwrites the Stdin, Stdout, and Stderr fields of the Command.
func (m *ManagedIO) manageCmd(cmd *Command) {
	m.Hijack(cmd)
}

// Hijack replaces the Stdin, Stdout, and Stderr of the Command with the
// ManagedIO's buffers.
//
// Writing to the Stdin buffer of ManagedIO will write to the Stdin of the
// Command.
// Reading from the Stdout and Stderr buffer of the ManagedIO will get the
// Stdout and Stderr of the Command.
//
// It also starts goroutines to copy the old std IO (if exists) from/to
// the buffers so that the caller can still read/write to the
// original reader/writer.
func (m *ManagedIO) Hijack(cmd *Command) {
	m.makeNonNil()

	if cmd == nil {
		Logger.Error("ManagedIO.Hijack: cmd is nil. No action taken.")
		return
	}

	cmd.Stdin = m.Stdin
	cmd.Stdout = m.Stdout
	cmd.Stderr = m.Stderr
}

// makeNonNil ensures that the buffers are not nil.
// If any of the buffers is nil, it will be replaced with an empty buffer.
func (m *ManagedIO) makeNonNil() {
	if m.Stdin == nil {
		Logger.Warn("ManagedIO.Stdin is nil. Setting it to an empty buffer.")
		m.Stdin = &bytes.Buffer{}
	}
	if m.Stdout == nil {
		Logger.Warn("ManagedIO.Stdout is nil. Setting it to an empty buffer.")
		m.Stdout = &bytes.Buffer{}
	}
	if m.Stderr == nil {
		Logger.Warn("ManagedIO.Stderr is nil. Setting it to an empty buffer.")
		m.Stderr = &bytes.Buffer{}
	}
}
