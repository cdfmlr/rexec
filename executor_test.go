package rexec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	osexec "os/exec"
	"sync"
	"testing"
	"time"
)

// Prerequisites:
//
//	cd ./testsshd && docker compose -f testsshd-docker-compose.yml up
//
// To start a sshd server on localhost:24622 (see testsshd/README.md for more details).
func TestExecutor_Execute(t *testing.T) {
	testSshMu.RLock()
	defer testSshMu.RUnlock()
	testSshTestServer(t)

	type args struct {
		executor Executor
		ctx      context.Context
		cmd      *Command
	}
	type want struct {
		panic  bool
		err    bool
		status int
		stdout string
		stderr string
	}
	type got struct {
		err error
		cmd *Command
	}
	tests := []struct {
		name           string
		args           args
		want           want
		additionalTest func(t *testing.T, g got)
	}{
		// nil executor: panic
		{
			name: "nilExecutor",
			args: args{
				executor: nil,
				ctx:      context.Background(),
				cmd: &Command{
					Command: "echo hello",
				},
			},
			want: want{
				panic:  true,
				err:    false,
				status: 0,
				stdout: "",
				stderr: "",
			},
			additionalTest: nil,
		},
		// LocalExecutor
		{
			name: "localNilCmd",
			args: args{
				executor: &LocalExecutor{},
				ctx:      context.Background(),
				cmd:      nil,
			},
			want: want{
				panic:  false,
				err:    true,
				status: 0,
				stdout: "",
				stderr: "",
			},
			additionalTest: func(t *testing.T, g got) {
				if !errors.Is(g.err, ErrNilCommand) {
					t.Errorf("❌ Execute() error = %v, wantErr %v", g.err, ErrNilCommand)
				} else {
					t.Logf("✅ Execute() error = %v", g.err)
				}
			},
		},
		{
			name: "localEcho",
			args: args{
				executor: &LocalExecutor{},
				ctx:      context.Background(),
				cmd: &Command{
					Command: "echo hello",
				},
			},
			want: want{
				panic:  false,
				err:    false,
				status: 0,
				stdout: "hello\n",
				stderr: "",
			},
			additionalTest: nil,
		},
		{
			name: "localStdin",
			args: args{
				executor: &LocalExecutor{},
				ctx:      context.Background(),
				cmd: &Command{
					Command: "cat -",
					Stdin:   bytes.NewReader([]byte("hello from stdin")),
				},
			},
			want: want{
				panic:  false,
				err:    false,
				status: 0,
				stdout: "hello from stdin",
				stderr: "",
			},
			additionalTest: nil,
		},
		{
			name: "localDirEnv",
			args: args{
				executor: &LocalExecutor{},
				ctx:      context.Background(),
				cmd: &Command{
					Command: "sh -c \" echo $TEST_ENV $(pwd) \"",
					Workdir: "/usr",
					Env: map[string]string{
						"TEST_ENV": "hello",
					},
				},
			},
			want: want{
				panic:  false,
				err:    false,
				status: 0,
				stdout: "hello /usr\n",
				stderr: "",
			},
			additionalTest: nil,
		},
		{
			name: "localErr",
			args: args{
				executor: &LocalExecutor{},
				ctx:      context.Background(),
				cmd: &Command{
					Command: "ls /not/exist/path",
				},
			},
			want: want{
				panic:  false,
				err:    true,
				status: 1,
				stdout: "",
				stderr: "ls: /not/exist/path: No such file or directory\n",
			},
			additionalTest: func(t *testing.T, g got) {
				t.Logf("👀 Execute() error: %T: %v", g.err, g.err)
			},
		},
		{
			name: "localBadCmd",
			args: args{
				executor: &LocalExecutor{},
				ctx:      context.Background(),
				cmd: &Command{
					Command: "notExistCommand",
				},
			},
			want: want{
				panic:  false,
				err:    true,
				status: -1,
				stdout: "",
				stderr: "",
			},
			additionalTest: func(t *testing.T, g got) {
				t.Logf("👀 Execute() error: %T: %v", g.err, g.err)
			},
		},
		// ShellExecutor: bash
		{
			name: "bashNilCmd",
			args: args{
				executor: &ShellExecutor{
					ShellPath: "/bin/bash",
					ShellArgs: []string{"-c"},
				},
				ctx: context.Background(),
				cmd: nil,
			},
			want: want{
				panic:  false,
				err:    true,
				status: 0,
				stdout: "",
				stderr: "",
			},
			additionalTest: func(t *testing.T, g got) {
				if !errors.Is(g.err, ErrNilCommand) {
					t.Errorf("❌ Execute() error = %v, wantErr %v", g.err, ErrNilCommand)
				} else {
					t.Logf("✅ Execute() error = %v", g.err)
				}
			},
		},
		{
			name: "bashEcho",
			args: args{
				executor: &ShellExecutor{
					ShellPath: "/bin/bash",
					ShellArgs: []string{"-c"},
				},
				ctx: context.Background(),
				cmd: &Command{
					Command: "echo hello",
				},
			},
			want: want{
				panic:  false,
				err:    false,
				status: 0,
				stdout: "hello\n",
				stderr: "",
			},
			additionalTest: nil,
		},
		{
			name: "bashDirEnv",
			args: args{
				executor: &ShellExecutor{
					ShellPath: "/bin/bash",
					ShellArgs: []string{"-c"},
				},
				ctx: context.Background(),
				cmd: &Command{
					Command: "sh -c \" echo $TEST_ENV $(pwd) \"",
					Workdir: "/usr",
					Env: map[string]string{
						"TEST_ENV": "hello",
					},
				},
			},
			want: want{
				panic:  false,
				err:    false,
				status: 0,
				stdout: "hello /usr\n",
				stderr: "",
			},
			additionalTest: nil,
		},
		{
			name: "bashStdin",
			args: args{
				executor: &ShellExecutor{
					ShellPath: "/bin/bash",
					ShellArgs: []string{"-c"},
				},
				ctx: context.Background(),
				cmd: &Command{
					Command: "cat -",
					Stdin:   bytes.NewReader([]byte("hello from stdin")),
				},
			},
			want: want{
				panic:  false,
				err:    false,
				status: 0,
				stdout: "hello from stdin",
				stderr: "",
			},
			additionalTest: nil,
		},
		{
			name: "bashErr",
			args: args{
				executor: &ShellExecutor{
					ShellPath: "/bin/bash",
					ShellArgs: []string{"-c"},
				},
				ctx: context.Background(),
				cmd: &Command{
					Command: "ls /not/exist/path",
				},
			},
			want: want{
				panic:  false,
				err:    true,
				status: 1,
				stdout: "",
				stderr: "ls: /not/exist/path: No such file or directory\n",
			},
			additionalTest: func(t *testing.T, g got) {
				t.Logf("👀 Execute() error: %T: %v", g.err, g.err)
			},
		},
		{
			name: "bashBadCmd",
			args: args{
				executor: &ShellExecutor{
					ShellPath: "/bin/bash",
					ShellArgs: []string{"-c"},
				},
				ctx: context.Background(),
				cmd: &Command{
					Command: "notExistCommand",
				},
			},
			want: want{
				panic:  false,
				err:    true,
				status: 127,
				stdout: "",
				stderr: "/bin/bash: notExistCommand: command not found\n",
			},
			additionalTest: func(t *testing.T, g got) {
				t.Logf("👀 Execute() error: %T: %v", g.err, g.err)
			},
		},
		// ShellExecutor: ssh
		{
			name: "externalSshNilCmd",
			args: args{
				executor: &ShellExecutor{
					ShellPath: "ssh",
					ShellArgs: []string{
						"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-q",
						"-o", "PasswordAuthentication=no",
						"-i", "./testsshd/testsshd.id_rsa",
						"-p", "24622", "root@localhost",
					},
				},
				ctx: context.Background(),
				cmd: nil,
			},
			want: want{
				panic:  false,
				err:    true,
				status: 0,
				stdout: "",
				stderr: "",
			},
			additionalTest: func(t *testing.T, g got) {
				if !errors.Is(g.err, ErrNilCommand) {
					t.Errorf("❌ Execute() error = %v, wantErr %v", g.err, ErrNilCommand)
				} else {
					t.Logf("✅ Execute() error = %v", g.err)
				}
			},
		},
		{
			name: "externalSshEcho",
			args: args{
				executor: &ShellExecutor{
					ShellPath: "ssh",
					ShellArgs: []string{
						"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-q",
						"-o", "PasswordAuthentication=no",
						"-i", "./testsshd/testsshd.id_rsa",
						"-p", "24622", "root@localhost",
					},
				},
				ctx: context.Background(),
				cmd: &Command{
					Command: "echo hello",
				},
			},
			want: want{
				panic:  false,
				err:    false,
				status: 0,
				stdout: "hello\n",
				stderr: "",
			},
			additionalTest: nil,
		},
		{
			name: "externalSshDirEnv",
			args: args{
				executor: &ShellExecutor{
					ShellPath: "ssh",
					ShellArgs: []string{
						"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-q",
						"-o", "PasswordAuthentication=no",
						"-i", "./testsshd/testsshd.id_rsa",
						"-p", "24622", "root@localhost",
					},
				},
				ctx: context.Background(),
				cmd: &Command{
					Command: "sh -c \" echo $TEST_ENV $(pwd) \"",
					Workdir: "/usr",
					Env: map[string]string{
						"TEST_ENV": "hello",
					},
				},
			},
			want: want{
				panic:  false,
				err:    false,
				status: 0,
				stdout: "hello /usr\n",
				stderr: "",
			},
			additionalTest: nil,
		},
		{
			name: "externalSshStdin",
			args: args{
				executor: &ShellExecutor{
					ShellPath: "ssh",
					ShellArgs: []string{
						"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-q",
						"-o", "PasswordAuthentication=no",
						"-i", "./testsshd/testsshd.id_rsa",
						"-p", "24622", "root@localhost",
					},
				},
				ctx: context.Background(),
				cmd: &Command{
					Command: "cat -",
					Stdin:   bytes.NewReader([]byte("hello from stdin")),
				},
			},
			want: want{
				panic:  false,
				err:    false,
				status: 0,
				stdout: "hello from stdin",
				stderr: "",
			},
			additionalTest: nil,
		},
		{
			name: "externalSshErr",
			args: args{
				executor: &ShellExecutor{
					ShellPath: "ssh",
					ShellArgs: []string{
						"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-q",
						"-o", "PasswordAuthentication=no",
						"-i", "./testsshd/testsshd.id_rsa",
						"-p", "24622", "root@localhost",
					},
				},
				ctx: context.Background(),
				cmd: &Command{
					Command: "ls /not/exist/path",
				},
			},
			want: want{
				panic:  false,
				err:    true,
				status: 1,
				stdout: "",
				stderr: "ls: /not/exist/path: No such file or directory\n",
			},
			additionalTest: func(t *testing.T, g got) {
				t.Logf("👀 Execute() error: %T: %v", g.err, g.err)
			},
		},
		{
			name: "externalSshBadCmd",
			args: args{
				executor: &ShellExecutor{
					ShellPath: "ssh",
					ShellArgs: []string{
						"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-q",
						"-o", "PasswordAuthentication=no",
						"-i", "./testsshd/testsshd.id_rsa",
						"-p", "24622", "root@localhost",
					},
				},
				ctx: context.Background(),
				cmd: &Command{
					Command: "notExistCommand",
				},
			},
			want: want{
				panic:  false,
				err:    true,
				status: 127,
				stdout: "",
				stderr: "ash: notExistCommand: not found\n",
			},
			additionalTest: func(t *testing.T, g got) {
				t.Logf("👀 Execute() error: %T: %v", g.err, g.err)
			},
		},
		// ImmediateSshExecutor
		{
			name: "immSshNilCmd",
			args: args{
				executor: &ImmediateSshExecutor{Config: &SshClientConfig{
					Addr: "localhost:24622",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
				}},
				ctx: context.Background(),
				cmd: &Command{
					Command: "echo hello",
				},
			},
			want: want{
				panic:  false,
				err:    false,
				status: 0,
				stdout: "hello\n",
				stderr: "",
			},
			additionalTest: nil,
		},
		{
			name: "immSshEcho",
			args: args{
				executor: &ImmediateSshExecutor{Config: &SshClientConfig{
					Addr: "localhost:24622",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
				}},
				ctx: context.Background(),
				cmd: &Command{
					Command: "echo hello",
				},
			},
			want: want{
				panic:  false,
				err:    false,
				status: 0,
				stdout: "hello\n",
				stderr: "",
			},
			additionalTest: nil,
		},
		{
			name: "immSshDirEnv",
			args: args{
				executor: &ImmediateSshExecutor{Config: &SshClientConfig{
					Addr: "localhost:24622",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
				}},
				ctx: context.Background(),
				cmd: &Command{
					Command: "sh -c \" echo $TEST_ENV $(pwd) \"",
					Workdir: "/usr",
					Env: map[string]string{
						"TEST_ENV": "hello",
					},
				},
			},
			want: want{
				panic:  false,
				err:    false,
				status: 0,
				stdout: "hello /usr\n",
				stderr: "",
			},
			additionalTest: nil,
		},
		{
			name: "immSshStdin",
			args: args{
				executor: &ImmediateSshExecutor{Config: &SshClientConfig{
					Addr: "localhost:24622",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
				}},
				ctx: context.Background(),
				cmd: &Command{
					Command: "cat -",
					Stdin:   bytes.NewReader([]byte("hello from stdin")),
				},
			},
			want: want{
				panic:  false,
				err:    false,
				status: 0,
				stdout: "hello from stdin",
				stderr: "",
			},
			additionalTest: nil,
		},
		{
			name: "immSshErr",
			args: args{
				executor: &ImmediateSshExecutor{Config: &SshClientConfig{
					Addr: "localhost:24622",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
				}},
				ctx: context.Background(),
				cmd: &Command{
					Command: "ls /not/exist/path",
				},
			},
			want: want{
				panic:  false,
				err:    true,
				status: 1,
				stdout: "",
				stderr: "ls: /not/exist/path: No such file or directory\n",
			},
			additionalTest: func(t *testing.T, g got) {
				t.Logf("👀 Execute() error: %T: %v", g.err, g.err)
			},
		},
		{
			name: "immSshBadCmd",
			args: args{
				executor: &ImmediateSshExecutor{Config: &SshClientConfig{
					Addr: "localhost:24622",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
				}}, ctx: context.Background(),
				cmd: &Command{
					Command: "notExistCommand",
				},
			},
			want: want{
				panic:  false,
				err:    true,
				status: 127,
				stdout: "",
				stderr: "ash: notExistCommand: not found\n",
			},
			additionalTest: func(t *testing.T, g got) {
				t.Logf("👀 Execute() error: %T: %v", g.err, g.err)
			},
		},
		// KeepAliveSshExecutor
		{
			name: "keepAliveSshNilCmd",
			args: args{
				executor: &KeepAliveSshExecutor{Config: &SshClientConfig{
					Addr: "localhost:24622",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
				}},
				ctx: context.Background(),
				cmd: &Command{
					Command: "echo hello",
				},
			},
			want: want{
				panic:  false,
				err:    false,
				status: 0,
				stdout: "hello\n",
				stderr: "",
			},
			additionalTest: nil,
		},
		{
			name: "keepAliveSshEcho",
			args: args{
				executor: &KeepAliveSshExecutor{Config: &SshClientConfig{
					Addr: "localhost:24622",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
				}},
				ctx: context.Background(),
				cmd: &Command{
					Command: "echo hello",
				},
			},
			want: want{
				panic:  false,
				err:    false,
				status: 0,
				stdout: "hello\n",
				stderr: "",
			},
			additionalTest: nil,
		},
		{
			name: "keepAliveSshDirEnv",
			args: args{
				executor: &KeepAliveSshExecutor{Config: &SshClientConfig{
					Addr: "localhost:24622",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
				}},
				ctx: context.Background(),
				cmd: &Command{
					Command: "sh -c \" echo $TEST_ENV $(pwd) \"",
					Workdir: "/usr",
					Env: map[string]string{
						"TEST_ENV": "hello",
					},
				},
			},
			want: want{
				panic:  false,
				err:    false,
				status: 0,
				stdout: "hello /usr\n",
				stderr: "",
			},
			additionalTest: nil,
		},
		{
			name: "keepAliveSshStdin",
			args: args{
				executor: &KeepAliveSshExecutor{Config: &SshClientConfig{
					Addr: "localhost:24622",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
				}},
				ctx: context.Background(),
				cmd: &Command{
					Command: "cat -",
					Stdin:   bytes.NewReader([]byte("hello from stdin")),
				},
			},
			want: want{
				panic:  false,
				err:    false,
				status: 0,
				stdout: "hello from stdin",
				stderr: "",
			},
			additionalTest: nil,
		},
		{
			name: "keepAliveSshErr",
			args: args{
				executor: &KeepAliveSshExecutor{Config: &SshClientConfig{
					Addr: "localhost:24622",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
				}},
				ctx: context.Background(),
				cmd: &Command{
					Command: "ls /not/exist/path",
				},
			},
			want: want{
				panic:  false,
				err:    true,
				status: 1,
				stdout: "",
				stderr: "ls: /not/exist/path: No such file or directory\n",
			},
			additionalTest: func(t *testing.T, g got) {
				t.Logf("👀 Execute() error: %T: %v", g.err, g.err)
			},
		},
		{
			name: "keepAliveSshBadCmd",
			args: args{
				executor: &KeepAliveSshExecutor{Config: &SshClientConfig{
					Addr: "localhost:24622",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
				}}, ctx: context.Background(),
				cmd: &Command{
					Command: "notExistCommand",
				},
			},
			want: want{
				panic:  false,
				err:    true,
				status: 127,
				stdout: "",
				stderr: "ash: notExistCommand: not found\n",
			},
			additionalTest: func(t *testing.T, g got) {
				t.Logf("👀 Execute() error: %T: %v", g.err, g.err)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tt.want.panic {
					t.Errorf("❌ Execute() panic = %v, wantPanic %v", r, tt.want.panic)
				} else {
					t.Logf("✅ Execute() panic = %v", r)
				}
			}()

			e := tt.args.executor
			eJson, _ := json.Marshal(e)
			t.Logf("🔍 executor: %T: %v", e, string(eJson))

			cmd := tt.args.cmd

			var err error

			err = cmd.Validate()
			if err != nil {
				t.Logf("🔍 Validate() error = %v", err)
			}
			// if (err != nil) != tt.want.err {
			//	t.Errorf("❌ Validate() error = %v, wantErr %v", err, tt.want.err)
			// } else {
			//	t.Logf("✅ Validate() error = %v", err)
			// }

			if err == nil {
				t.Logf("🔍 cmd ShellString: %v", cmd.ShellString())
			}

			// Hijack the IO

			var skipIOHijack bool
			if cmd == nil {
				skipIOHijack = true
			}

			var oldStdin io.Reader = &bytes.Buffer{}
			var oldStdout io.Writer = &bytes.Buffer{}
			var oldStderr io.Writer = &bytes.Buffer{}

			var stdin, stdout, stderr bytes.Buffer

			if !skipIOHijack {
				oldStdin = cmd.Stdin
				oldStdout = cmd.Stdout
				oldStderr = cmd.Stderr

				cmd.Stdin = &stdin
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr
			}

			go func() {
				_, _ = io.Copy(&stdin, oldStdin)
			}()

			go func() {
				_, _ = io.Copy(oldStdout, &stdout)
			}()

			go func() {
				_, _ = io.Copy(oldStderr, &stderr)
			}()

			if err = e.Execute(tt.args.ctx, tt.args.cmd); (err != nil) != tt.want.err {
				t.Errorf("❌ Execute() error = %v, wantErr %v", err, tt.want.err)
			} else {
				t.Logf("✅ Execute() error = %v", err)
			}

			if cmd != nil {
				if got := cmd.Status; got != tt.want.status {
					t.Errorf("❌ Execute() status = %v, want %v", got, tt.want.status)
				} else {
					t.Logf("✅ Execute() status = %v", got)
				}
			} else {
				t.Logf("⏩ cmd is nil, skipping status check")
			}

			if got := stdout.String(); got != tt.want.stdout {
				t.Errorf("❌ Execute() stdout = %q, want %q", got, tt.want.stdout)
			} else {
				t.Logf("✅ Execute() stdout = %q", got)
			}

			if got := stderr.String(); got != tt.want.stderr {
				t.Errorf("❌ Execute() stderr = %q, want %q", got, tt.want.stderr)
			} else {
				t.Logf("✅ Execute() stderr = %q", got)
			}

			if tt.additionalTest != nil {
				tt.additionalTest(t, got{err: err, cmd: cmd})
			}
		})
	}
}

// Prerequisites:
//
//	cd ./testsshd && docker compose -f testsshd-docker-compose.yml up
//
// To start a sshd server on localhost:24622 (see testsshd/README.md for more details).
func TestExecutor_Execute_cancel(t *testing.T) {
	testSshMu.RLock()
	defer testSshMu.RUnlock()
	testSshTestServer(t)

	type args struct {
		executor    Executor
		ctx         context.Context
		cmd         *Command
		cancelAfter time.Duration
	}
	type want struct {
		panic  bool
		err    bool
		status int
		stdout string
		stderr string
	}
	type got struct {
		err error
		cmd *Command
	}
	tests := []struct {
		name           string
		args           args
		want           want
		additionalTest func(t *testing.T, g got)
	}{
		// LocalExecutor
		{
			name: "localCancel",
			args: args{
				executor: &LocalExecutor{},
				ctx:      context.Background(),
				cmd: &Command{
					Command: "sleep 10",
				},
				cancelAfter: 1 * time.Second,
			},
			want: want{
				panic:  false,
				err:    true,
				status: -1,
				stdout: "",
				stderr: "",
			},
			additionalTest: func(t *testing.T, g got) {
				if !errors.Is(g.err, context.Canceled) {
					t.Errorf("❌ Execute() error = %v, wantErr %v", g.err, context.Canceled)
				} else {
					t.Logf("✅ Execute() error = %v", g.err)
				}
			},
		},
		// ShellExecutor: bash
		{
			name: "bashCancel",
			args: args{
				executor: &ShellExecutor{
					ShellPath: "/bin/bash",
					ShellArgs: []string{"-c"},
				},
				ctx: context.Background(),
				cmd: &Command{
					Command: "sleep 10",
				},
				cancelAfter: 1 * time.Second,
			},
			want: want{
				panic:  false,
				err:    true,
				status: -1,
				stdout: "",
				stderr: "",
			},
			additionalTest: func(t *testing.T, g got) {
				if !errors.Is(g.err, context.Canceled) {
					t.Errorf("❌ Execute() error = %v, wantErr %v", g.err, context.Canceled)
				} else {
					t.Logf("✅ Execute() error = %v", g.err)
				}
			},
		},
		// ShellExecutor: ssh
		{
			name: "externalSshCancel",
			args: args{
				executor: &ShellExecutor{
					ShellPath: "ssh",
					ShellArgs: []string{
						"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-q",
						"-o", "PasswordAuthentication=no",
						"-i", "./testsshd/testsshd.id_rsa",
						"-p", "24622", "root@localhost",
					},
				},
				ctx: context.Background(),
				cmd: &Command{
					Command: "sleep 10",
				},
				cancelAfter: 1 * time.Second,
			},
			want: want{
				panic:  false,
				err:    true,
				status: -1,
				stdout: "",
				stderr: "",
			},
			additionalTest: func(t *testing.T, g got) {
				if !errors.Is(g.err, context.Canceled) {
					t.Errorf("❌ Execute() error = %v, wantErr %v", g.err, context.Canceled)
				} else {
					t.Logf("✅ Execute() error = %v", g.err)
				}
			},
		},
		// ImmediateSshExecutor
		{
			name: "immSshCancel",
			args: args{
				executor: &ImmediateSshExecutor{Config: &SshClientConfig{
					Addr: "localhost:24622",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
				}},
				ctx: context.Background(),
				cmd: &Command{
					Command: "sleep 10",
				},
				cancelAfter: 1 * time.Second,
			},
			want: want{
				panic:  false,
				err:    true,
				status: -1,
				stdout: "",
				stderr: "",
			},
			additionalTest: func(t *testing.T, g got) {
				if !errors.Is(g.err, context.Canceled) {
					t.Errorf("❌ Execute() error = %v, wantErr %v", g.err, context.Canceled)
				} else {
					t.Logf("✅ Execute() error = %v", g.err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.want.panic {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("❌ Execute() did not panic")
					} else {
						t.Logf("✅ Execute() did panic: %#v", r)
					}
				}()
			}

			e := tt.args.executor
			eJson, _ := json.Marshal(e)
			t.Logf("🔍 executor: %T: %v", e, string(eJson))

			cmd := tt.args.cmd

			var err error

			err = cmd.Validate()
			if err != nil {
				t.Errorf("❌ Validate() error = %v, wantErr %v", err, false)
			} else {
				t.Logf("✅ Validate() error = %v", err)
			}

			if err == nil {
				t.Logf("🔍 cmd ShellString: %v", cmd.ShellString())
			}

			// Hijack the IO

			var skipIOHijack bool
			if cmd == nil {
				skipIOHijack = true
			}

			var oldStdin io.Reader = &bytes.Buffer{}
			var oldStdout io.Writer = &bytes.Buffer{}
			var oldStderr io.Writer = &bytes.Buffer{}

			var stdin, stdout, stderr bytes.Buffer

			if !skipIOHijack {
				oldStdin = cmd.Stdin
				oldStdout = cmd.Stdout
				oldStderr = cmd.Stderr

				cmd.Stdin = &stdin
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr
			}

			go func() {
				_, _ = io.Copy(&stdin, oldStdin)
			}()

			go func() {
				_, _ = io.Copy(oldStdout, &stdout)
			}()

			go func() {
				_, _ = io.Copy(oldStderr, &stderr)
			}()

			if tt.args.cancelAfter <= 0 {
				t.Fatalf("❌ cancelAfter must be positive")
			}
			ctx, cancel := context.WithCancel(tt.args.ctx)

			go func() {
				time.Sleep(tt.args.cancelAfter)
				t.Logf("⏰ cancelling after %v", tt.args.cancelAfter)
				cancel()
			}()

			if err = e.Execute(ctx, tt.args.cmd); (err != nil) != tt.want.err {
				t.Errorf("❌ Execute() error = %v, wantErr %v", err, tt.want.err)
			} else {
				t.Logf("✅ Execute() error = %v", err)
			}

			if cmd != nil {
				if got := cmd.Status; got != tt.want.status {
					t.Errorf("❌ Execute() status = %v, want %v", got, tt.want.status)
				} else {
					t.Logf("✅ Execute() status = %v", got)
				}
			} else {
				t.Logf("⏩ cmd is nil, skipping status check")
			}

			if got := stdout.String(); got != tt.want.stdout {
				t.Errorf("❌ Execute() stdout = %q, want %q", got, tt.want.stdout)
			} else {
				t.Logf("✅ Execute() stdout = %q", got)
			}

			if got := stderr.String(); got != tt.want.stderr {
				t.Errorf("❌ Execute() stderr = %q, want %q", got, tt.want.stderr)
			} else {
				t.Logf("✅ Execute() stderr = %q", got)
			}

			if tt.additionalTest != nil {
				tt.additionalTest(t, got{err: err, cmd: cmd})
			}
		})
	}
}

// Prerequisites:
//
//	cd ./testsshd && docker compose -f testsshd-docker-compose.yml up
//
// To start a sshd server on localhost:24622 (see testsshd/README.md for more details).
func Test_singleCommandMultiExecute(t *testing.T) {
	testSshMu.RLock()
	defer testSshMu.RUnlock()
	testSshTestServer(t)

	executors := []Executor{
		&LocalExecutor{},
		&ShellExecutor{
			ShellPath: "/bin/bash",
			ShellArgs: []string{"-c"},
		},
		&ShellExecutor{
			ShellPath: "ssh",
			ShellArgs: []string{
				"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-q",
				"-o", "PasswordAuthentication=no",
				"-i", "./testsshd/testsshd.id_rsa",
				"-p", "24622", "root@localhost",
			},
		},
		&ImmediateSshExecutor{Config: &SshClientConfig{
			Addr: "localhost:24622",
			User: "root",
			Auth: []SshAuth{
				{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
			},
			TimeoutSeconds: 5,
		}},
	}

	ctx := context.Background()

	t.Run("sequential", func(t *testing.T) {
		cmd := &Command{
			Command: "echo hello",
		}

		for i, executor := range executors {
			err := executor.Execute(ctx, cmd)
			if i == 0 && err != nil {
				t.Fatalf("❌ executors[%v] (%T) Execute err: %v", i, executor, err)
			}
			if i != 0 && !errors.Is(err, ErrStartedCommand) {
				t.Fatalf("❌ executors[%v] (%T) Execute err: %v", i, executor, err)
			}
		}
	})

	t.Run("concurrent", func(t *testing.T) {
		cmd := &Command{
			Command: "echo hello",
		}

		var result struct {
			mu     sync.Mutex
			errors []error
		}

		var wg sync.WaitGroup

		for _, executor := range executors {
			wg.Add(1)
			go func() {
				defer wg.Done()

				err := executor.Execute(ctx, cmd)

				result.mu.Lock()
				defer result.mu.Unlock()

				result.errors = append(result.errors, err)
			}()
		}

		wg.Wait()

		result.mu.Lock()
		defer result.mu.Unlock()

		errorCount := 0
		for _, err := range result.errors {
			if err != nil {
				errorCount += 1
			}
		}

		if errorCount != len(executors)-1 {
			t.Errorf("❌ errorCount = %v, expected %v", errorCount, len(executors)-1)
		}

		t.Logf("👀 errors: %v", result.errors)
	})

}

// a common error for go ssh stuff is forgetting to close the client,
// causing a leak that will eventually hang the remote host.
//
// this kind of error had hurt me a lot. So this test examines it.
//
// Prerequisites:
//
//	cd ./testsshd && docker compose -f testsshd-docker-compose.yml up
//
// To start a sshd server on localhost:24622 (see testsshd/README.md for more details).
func TestImmediateSshExecutor_closing(t *testing.T) {
	t.Skipf("Skipping TestImmediateSshExecutor_closing temporarily: it take ~1min to run.")

	testSshMu.Lock()
	defer testSshMu.Unlock()
	testSshTestServer(t)

	executor := &ImmediateSshExecutor{Config: &SshClientConfig{
		Addr: "localhost:24622",
		User: "root",
		Auth: []SshAuth{
			{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
		},
		TimeoutSeconds: 5,
	}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// run lsof @ remote host to count open connections
	var lsofCount = func() (int, error) {
		// lsofCmdStr := "lsof -i :22 | wc -l" // alpine busybox ignores -i, but it's fine for this test
		// lsofCmdStr := "for p in 22 24622; do lsof -i :${p}; done | tee rexec_TestImmediateSshExecuator_closing_dbg.$(date +%s) | wc -l"
		lsofCmdStr := "for p in 22 24622; do lsof -i :${p}; done | wc -l"
		var stdout bytes.Buffer
		cmd := &Command{
			Command: lsofCmdStr,
			Stdout:  &stdout,
		}
		if err := executor.Execute(ctx, cmd); err != nil {
			return 0, err
		}
		var count int
		_, err := fmt.Sscanf(stdout.String(), "%d", &count)
		return count, err
	}

	// 尽量等其他测试的连接释放掉，不影响该测试。
	t.Logf("Sleep for 10 second to start this test...")
	time.Sleep(10 * time.Second)

	// check the initial number of open connections
	initialCount, err := lsofCount()
	if err != nil {
		t.Errorf("❌ get initialCount error = %v", err)
	}
	t.Logf("✅ initial lsof count = %d", initialCount)

	pool := make(chan struct{}, 8) // limit the number of concurrent goroutines
	wg := sync.WaitGroup{}

	for i := 0; i < 100; i++ {
		go func() {
			pool <- struct{}{}
			wg.Add(1)
			defer func() {
				<-pool
				wg.Done()
			}()

			cmd := &Command{
				Command: "sleep 3; echo hello",
			}
			if err := executor.Execute(ctx, cmd); err != nil {
				t.Errorf("❌ Execute() error = %v", err)
			} else {
				t.Logf("✅ Execute() error = %v", err)
			}

			// check the number of open connections after each command
			count, err := lsofCount()
			if err != nil {
				t.Errorf("❌ get count error = %v", err)
			}
			if count < initialCount {
				// 这个错可能是其他测试的连接释放的影响，单独重测一次这个 Test 就行了。
				// 虽然这个错误有可能意外发生，但是意外发生这个错误不太可能了，
				// 目前应该大概是已经正确地使用读写锁来避免此测试和其他 SSH 相关测试冲突了。
				t.Errorf("❌ count = %d, want >= %d", count, initialCount)
			} else {
				t.Logf("✅ lsof count = %d", count)
			}
		}()
	}

	wg.Wait()
	time.Sleep(3 * time.Second) // wait a bit more for the lsof count falling

	// check the final number of open connections
	finalCount, err := lsofCount()
	if err != nil {
		t.Errorf("❌ get finalCount error = %v", err)
	}

	if finalCount > initialCount {
		// 这个错可能是其他测试的连接释放的影响，单独重测一次这个 Test 就行了。
		// 虽然这个错误有可能意外发生，但是意外发生这个错误不太可能了，
		// 目前应该大概是已经正确地使用读写锁来避免此测试和其他 SSH 相关测试冲突了。
		t.Errorf("❌ finalCount = %d, want <= %d", finalCount, initialCount)
	}

	t.Logf("✅ all commands executed, final lsof count = %d", finalCount)

}

// Benchmarks

// Benchmark the performance of the LocalExecutor, ShellExecutor,
// ImmediateSshExecutor and KeepAliveSshExecutor.
//
// A result (goos: darwin, goarch: arm64):
//
//	BenchmarkExecutors/osexec-8         	     866	   1174200 ns/op	    9587 B/op	      65 allocs/op
//	BenchmarkExecutors/local-8          	     726	   1668309 ns/op	   13807 B/op	     142 allocs/op
//	BenchmarkExecutors/bash-8           	     853	   2559325 ns/op	   11517 B/op	      96 allocs/op
//	BenchmarkExecutors/externalSsh-8    	       6	 174750854 ns/op	   16104 B/op	     133 allocs/op
//	BenchmarkExecutors/immSsh-8         	      13	  92671224 ns/op	   99758 B/op	     785 allocs/op
//	BenchmarkExecutors/keepAliveSsh-8   	     816	   1353029 ns/op	   42973 B/op	     195 allocs/op
//	BenchmarkExecutors/keepAliveSshImmClose-8  	  12	  86293160 ns/op	  104712 B/op	     895 allocs/op
func BenchmarkExecutors(b *testing.B) {
	testSshMu.RLock()
	defer testSshMu.RUnlock()

	executors := map[string]Executor{ // name: executor
		"local": &LocalExecutor{},
		"bash": &ShellExecutor{
			ShellPath: "/bin/bash",
			ShellArgs: []string{"-c"},
		},
		"externalSsh": &ShellExecutor{
			ShellPath: "ssh",
			ShellArgs: []string{
				"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-q",
				"-o", "PasswordAuthentication=no",
				"-i", "./testsshd/testsshd.id_rsa",
				"-p", "24622", "root@localhost",
			},
		},
		"immSsh": &ImmediateSshExecutor{Config: &SshClientConfig{
			Addr: "localhost:24622",
			User: "root",
			Auth: []SshAuth{
				{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
			},
			TimeoutSeconds: 5,
		}},
		"keepAliveSsh": &KeepAliveSshExecutor{Config: &SshClientConfig{
			Addr: "localhost:24622",
			User: "root",
			Auth: []SshAuth{
				{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
			},
			TimeoutSeconds: 5,
			KeepAlive: SshKeepAliveConfig{
				IntervalSeconds: 10,
			},
		}},
	}

	ctx := context.Background()

	cmd := Command{
		Command: "echo hello",
	}

	// baseline
	b.Run("osexec", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := osexec.Command("echo", "hello").Run()
			if err != nil {
				b.Fatalf("❌ Execute() error = %v", err)
			}
		}
	})

	// executors
	for name, executor := range executors {
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				cmdCopy := cmd
				err := executor.Execute(ctx, &cmdCopy)
				if err != nil {
					b.Fatalf("❌ Execute() error = %v", err)
				}
			}
		})
	}

	// keepAliveSsh as immSsh
	// close the connection after each command
	b.Run("keepAliveSshImmClose", func(b *testing.B) {
		executor := &KeepAliveSshExecutor{Config: &SshClientConfig{
			Addr: "localhost:24622",
			User: "root",
			Auth: []SshAuth{
				{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
			},
			TimeoutSeconds: 5,
			KeepAlive: SshKeepAliveConfig{
				IntervalSeconds: 10,
			},
		}}
		for i := 0; i < b.N; i++ {
			cmdCopy := cmd
			err := executor.Execute(ctx, &cmdCopy)
			if err != nil {
				b.Fatalf("❌ Execute() error = %v", err)
			}
			executor.Close()
		}
	})
}

// Examples

func ExampleLocalExecutor_Execute() {
	executor := &LocalExecutor{}

	ctx := context.Background()

	var stdout bytes.Buffer

	cmd := &Command{
		Command: "echo hello",
		Stdout:  &stdout,
	}

	err := executor.Execute(ctx, cmd)
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	fmt.Printf("stdout: %q", stdout.String())
	// Output: stdout: "hello\n"
}

func ExampleShellExecutor_Execute_bash() {
	executor := &ShellExecutor{
		ShellPath: "/bin/bash",
		ShellArgs: []string{"-c"},
	}

	ctx := context.Background()

	var stdout bytes.Buffer

	cmd := &Command{
		Command: "echo $TEST_ENV from $(pwd)",
		Workdir: "/usr",
		Env: map[string]string{
			"TEST_ENV": "hello",
		},
		Stdout: &stdout,
	}

	err := executor.Execute(ctx, cmd)
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	fmt.Printf("stdout: %q", stdout.String())
	// Output: stdout: "hello from /usr\n"
}

func ExampleShellExecutor_Execute_ssh() {
	executor := &ShellExecutor{
		ShellPath: "ssh",
		ShellArgs: []string{
			"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-q",
			"-o", "PasswordAuthentication=no",
			"-i", "./testsshd/testsshd.id_rsa",
			"-p", "24622", "root@localhost",
		},
	}

	ctx := context.Background()

	var stdin = bytes.NewReader([]byte("hello from stdin"))
	var stdout bytes.Buffer

	cmd := &Command{
		Command: "cat -", // read from stdin
		Stdin:   stdin,
		Stdout:  &stdout,
	}

	err := executor.Execute(ctx, cmd)
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	fmt.Printf("stdout: %q", stdout.String())
	// Output: stdout: "hello from stdin"
}

func ExampleImmediateSshExecutor_Execute() {
	executor := &ImmediateSshExecutor{Config: &SshClientConfig{
		Addr: "localhost:24622",
		User: "root",
		Auth: []SshAuth{
			{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
		},
		TimeoutSeconds: 5,
	}}

	ctx := context.Background()

	var stdin = bytes.NewReader([]byte("stdin"))
	var stdout bytes.Buffer

	cmd := &Command{
		Command: "echo $ENV1 $ENV2 from $(pwd) and $(cat -)",
		Workdir: "/usr",
		Env: map[string]string{
			"ENV1": "hello",
			"ENV2": "world",
		},
		Stdin:  stdin,
		Stdout: &stdout,
	}

	err := executor.Execute(ctx, cmd)
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	fmt.Printf("stdout: %q", stdout.String())
	// Output: stdout: "hello world from /usr and stdin\n"
}

func ExampleImmediateSshExecutor_Execute_timeout() {
	executor := &ImmediateSshExecutor{Config: &SshClientConfig{
		Addr: "localhost:24622",
		User: "root",
		Auth: []SshAuth{
			{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
		},
		TimeoutSeconds: 5,
	}}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := &Command{
		Command: "sleep 10; echo hello",
		Stdout:  &stdout,
		Stderr:  &stderr,
	}

	err := executor.Execute(ctx, cmd)

	fmt.Printf("error: %v\n", err)
	fmt.Printf("stdout: %q\n", stdout.String())
	fmt.Printf("stderr: %q\n", stderr.String())

	// Output:
	// error: context deadline exceeded
	// stdout: ""
	// stderr: ""
}

func ExampleImmediateSshExecutor_Execute_cancel() {
	executor := &ImmediateSshExecutor{Config: &SshClientConfig{
		Addr: "localhost:24622",
		User: "root",
		Auth: []SshAuth{
			{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
		},
		TimeoutSeconds: 5,
	}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := &Command{
		Command: "sleep 10; echo hello",
		Stdout:  &stdout,
		Stderr:  &stderr,
	}

	time.AfterFunc(2*time.Second, cancel)

	err := executor.Execute(ctx, cmd)

	fmt.Printf("error: %v\n", err)
	fmt.Printf("stdout: %q\n", stdout.String())
	fmt.Printf("stderr: %q\n", stderr.String())

	// Output:
	// error: context canceled
	// stdout: ""
	// stderr: ""
}

// This example demonstrates using KeepAliveSshExecutor for periodic tasks.
func ExampleKeepAliveSshExecutor_Execute() {
	executor := &KeepAliveSshExecutor{Config: &SshClientConfig{
		Addr: "localhost:24622",
		User: "root",
		Auth: []SshAuth{
			{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
		},
		TimeoutSeconds: 5,
		KeepAlive: SshKeepAliveConfig{
			IntervalSeconds:  10,
			IncrementSeconds: 3,
		},
	}}
	defer executor.Close() // remember to close the KeepAliveSshExecutor

	// for demonstration, we only run the loop for 3 times.
	ctx, cancel := context.WithTimeout(context.Background(), 16*time.Second)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done(): // cancel at the 16th second
			return
		case <-ticker.C: // tick at the 5th, 10th, and 15th seconds
			cmd := &Command{
				Command: "echo T",
			}

			managedIO := NewManagedIO()
			managedIO.Hijack(cmd)

			err := executor.Execute(ctx, cmd)
			if err != nil {
				panic(err)
			}

			fmt.Printf("stdout: %q\n", managedIO.Stdout.String())
		}
	}

	// Output:
	// stdout: "T\n"
	// stdout: "T\n"
	// stdout: "T\n"
}
