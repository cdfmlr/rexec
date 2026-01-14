package main

import (
	"bytes"
	"context"
	"fmt"

	"github.com/cdfmlr/rexec/v2"
)

func main() {
	stdout := &bytes.Buffer{}
	cmd := &rexec.Command{Command: "echo hello", Stdout: stdout}

	exec := &rexec.LocalExecutor{}
	if err := exec.Execute(context.Background(), cmd); err != nil {
		panic(err)
	}

	fmt.Println("exit status:", cmd.Status)
	fmt.Println("stdout:", stdout.String())
}
