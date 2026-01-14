package rexec

import (
	"errors"
	"fmt"
	"reflect"
)

// This file provides a ExecutorFactory that helps create any
// Executor implementation from a single configuration.

// ExecutorFactory is a factory for creating executors.
// It helps caller to create an Executor without programming the exact type.
//
// It includes the configuration for the Local, Shell, ImmediateSsh,
// and KeepAliveSsh executors.
// Exactly one of these fields must be set to a non-nil value.
// ExecutorFactory.Executor() will create a new corresponding Executor based on
// this non-nil fields.
//
// Executor literals are not flexible to the executor type
// (e.g. ShellExecutor here):
//
//	executor := &rexec.ShellExecutor{ShellPath: "/bin/sh", ShellArgs: []string{"-c"}}
//
// With ExecutorFactory, the type of executor is determined by the non-nil
// field, it is easier to change (by configuration file, for example).
//
//	executor, _ := rexec.ExecutorFactory{
//	   Shell: &ShellExecutor{ShellPath: "/bin/sh", ShellArgs: []string{"-c"}},
//	}.Executor()
type ExecutorFactory struct {
	// All exported fields must be an ExecuteCloser.

	Local        *LocalExecutor
	Shell        *ShellExecutor
	ImmediateSsh *ImmediateSshExecutor
	KeepAliveSsh *KeepAliveSshExecutor

	// NOTE: Add a new executor:
	//   1. Add a new field here.
	//   2. Implement the ExecuteCloser interface for the new executor.
	// (3). Executor() will automatically pick up the new executor since
	//      it uses reflection (see executorsMap).
}

// panic if any exported field in ExecutorFactory is not an ExecuteCloser.
func init() {
	v := reflect.ValueOf(ExecutorFactory{})
	for i := 0; i < v.NumField(); i++ {
		if !v.Type().Field(i).IsExported() {
			continue
		}

		_, ok := v.Field(i).Interface().(ExecuteCloser)
		if !ok {
			panic(fmt.Sprintf(
				"ExecutorFactory: field %s is not ExecuteCloser",
				v.Type().Field(i).Name))
		}
	}
}

// executorsMap returns a map of executor names to executors for
// each field in the ExecutorFactory.
//   - key: "f.Field.Name", e.g. "Local"
//   - value: ExecuteCloser(f.Field.Value), e.g. ExecuteCloser(f.Local)
//
// executorsMap depends on reflection.
func (f ExecutorFactory) executorsMap() map[string]ExecuteCloser {
	logger := Logger.With("field", "rexec.ExecutorFactory.executorsMap")

	//executors := map[string] ExecuteCloser {
	//	"Local":        f.Local,
	//	"Shell":        f.Shell,
	//	"ImmediateSsh": f.ImmediateSsh,
	//	"KeepAliveSsh": f.KeepAliveSsh,
	//}

	executors := make(map[string]ExecuteCloser)

	v := reflect.ValueOf(f)
	for i := 0; i < v.NumField(); i++ {
		executor, ok := v.Field(i).Interface().(ExecuteCloser)
		if !ok {
			logger.Warn("field is not ExecuteCloser. Unexpected, skipping", "field", v.Type().Field(i).Name)
			continue
		}
		executors[v.Type().Field(i).Name] = executor
	}
	return executors
}

// Executor creates the corresponding Executor.
//
// It returns an error if no executor is properly set,
// or multiple executors are set.
func (f ExecutorFactory) Executor() (ExecuteCloser, error) {
	logger := Logger.With("field", "rexec.ExecutorFactory.Executor")

	executors := f.executorsMap() // "FieldName": ExecuteCloser(f.Field)

	nonNilExecutors := make([]string, 0, 1)

	for name, executor := range executors {
		logger := logger.With("executorKind", name) // intended shadowing

		err := executor.validate()
		if errors.Is(err, ErrNilExecutor) {
			logger.Debug("validate executor: nil. Continue.")
			continue
		}
		if err != nil {
			logger.Error("validate executor: bad configuration. Abort.", "err", err)
			return nil, fmt.Errorf("%w: (%s) %w", ErrExecutorBadConfig, name, err)
		}

		logger.Debug("validate executor: ok")
		nonNilExecutors = append(nonNilExecutors, name)
	}

	switch len(nonNilExecutors) {
	case 0:
		logger.Error("no executor is properly set. Error.")
		return nil, fmt.Errorf("%w: all nil", ErrExecutorNotSet)
	case 1:
		name := nonNilExecutors[0]
		executor := executors[name]
		logger.Info("executor created", "executorKind", name, "executor", executor)
		return executor, nil
	default:
		logger.Error("multiple executors are set. Error.", "executors", nonNilExecutors)
		return nil, fmt.Errorf("%w: %v", ErrMultipleExecutors, nonNilExecutors)
	}
}

// ExecuteCloser is an interface that combines Executor and Closer.
//
// ExecutorFactory will create executors that implement this interface.
type ExecuteCloser interface {
	Executor
	Close() error
	validate() error // validate checks if the executors are properly set and ready to use.
}

var (
	_ ExecuteCloser = (*LocalExecutor)(nil)
	_ ExecuteCloser = (*ShellExecutor)(nil)
	_ ExecuteCloser = (*ImmediateSshExecutor)(nil)
	_ ExecuteCloser = (*KeepAliveSshExecutor)(nil)
)

// impl Close() for each executor

func (e *LocalExecutor) Close() error { return nil }

func (e *ShellExecutor) Close() error { return nil }

func (e *ImmediateSshExecutor) Close() error { return nil }

// impl validate() for each executor.
// notice that the nil check is required. See also ExecutorFactory.Executor().

func (e *LocalExecutor) validate() error {
	if e == nil {
		return ErrNilExecutor
	}
	return nil
}

func (e *ShellExecutor) validate() error {
	if e == nil {
		return ErrNilExecutor
	}
	if e.ShellPath == "" {
		return fmt.Errorf("%w: shell path is empty", ErrExecutorBadConfig)
	}
	return nil
}

func (e *ImmediateSshExecutor) validate() error {
	if e == nil {
		return ErrNilExecutor
	}
	if e.Config == nil {
		return fmt.Errorf("%w: ssh config is nil", ErrExecutorBadConfig)
	}
	err := validateSshClientConfig(e.Config)
	if err != nil {
		return fmt.Errorf("%w: ssh config is invalid: %w", ErrExecutorBadConfig, err)
	}
	return nil

}

func (e *KeepAliveSshExecutor) validate() error {
	if e == nil {
		return ErrNilExecutor
	}
	if e.Config == nil {
		return fmt.Errorf("%w: ssh config is nil", ErrExecutorBadConfig)
	}
	err := validateSshClientConfig(e.Config)
	if err != nil {
		return fmt.Errorf("%w: ssh config is invalid: %w", ErrExecutorBadConfig, err)
	}
	return nil
}

// ExecutorFactory errors
var (
	ErrExecutorNotSet    = fmt.Errorf("no executor is properly set")
	ErrMultipleExecutors = fmt.Errorf("multiple executors are set")
	ErrNilExecutor       = fmt.Errorf("executor is nil")
	ErrExecutorBadConfig = fmt.Errorf("executor has bad configuration")
)
