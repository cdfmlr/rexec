package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cdfmlr/rexec/v2"
)

// this file tests examples in README.md

func Example_gettingStarted() {
	ctx := context.Background()

	stdout := &bytes.Buffer{}
	cmd := &rexec.Command{Command: "echo hello", Stdout: stdout}

	exec := &rexec.LocalExecutor{}
	if err := exec.Execute(ctx, cmd); err != nil {
		panic(err)
	}

	fmt.Println("exit status:", cmd.Status)
	fmt.Println("stdout:", stdout.String())

	// Output:
	// exit status: 0
	// stdout: hello
}

func Example_shell() {
	exec := &rexec.ShellExecutor{ShellPath: "/bin/sh", ShellArgs: []string{"-c"}}
	cmd := &rexec.Command{Command: "echo $EDITOR", Stdout: os.Stdout}
	_ = exec.Execute(context.Background(), cmd)

	// Output: vi
}

func Example_ssh() {
	cfg := &rexec.SshClientConfig{
		Addr: "example.com:22",
		User: "root",
		Auth: []rexec.SshAuth{{Password: "secret"}},
	}
	stdout := &bytes.Buffer{}
	cmd := &rexec.Command{Command: "hostname", Stdout: stdout}
	err := cmd.Validate()
	if err != nil {
		panic(err)
	}

	_ = &rexec.ImmediateSshExecutor{Config: cfg}

	ka := &rexec.KeepAliveSshExecutor{Config: cfg}
	defer ka.Close()

	// Output:
}

func Example_factory() {
	factory := rexec.ExecutorFactory{
		Shell: &rexec.ShellExecutor{ShellPath: "/bin/sh", ShellArgs: []string{"-c"}},
	}
	exec, err := factory.Executor()
	if err != nil {
		panic(err)
	}
	err = exec.Execute(context.Background(), &rexec.Command{
		Command: "echo hello",
		Stdout:  os.Stdout,
	})
	if err != nil {
		panic(err)
	}

	// Output: hello
}

func Example_fromJson() {
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

	// Output: 2
}
