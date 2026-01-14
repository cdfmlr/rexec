package rexec

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

// Prerequisites:
//
//	cd ./testsshd && docker compose -f testsshd-docker-compose.yml up
//
// To start a sshd server on localhost:24622 (see testsshd/README.md for more details).
func TestExecutorFactory_Executor(t *testing.T) {
	testSshMu.RLock()
	defer testSshMu.RUnlock()
	testSshTestServer(t)

	tests := []struct {
		name    string
		f       ExecutorFactory
		wantErr error
		want    ExecuteCloser
	}{
		{
			name:    "allNil",
			f:       ExecutorFactory{},
			wantErr: ErrExecutorNotSet,
			want:    nil,
		},
		{
			name: "Local",
			f: ExecutorFactory{
				Local: &LocalExecutor{},
			},
			wantErr: nil,
			want:    &LocalExecutor{},
		},
		{
			name: "Shell",
			f: ExecutorFactory{
				Shell: &ShellExecutor{
					ShellPath: "/bin/sh",
					ShellArgs: []string{"-c"},
				},
			},
			wantErr: nil,
			want: &ShellExecutor{
				ShellPath: "/bin/sh",
				ShellArgs: []string{"-c"},
			},
		},
		{
			name: "ShellBad",
			f: ExecutorFactory{
				Shell: &ShellExecutor{},
			},
			wantErr: ErrExecutorBadConfig,
			want:    nil,
		},
		{
			name: "ImmediateSsh",
			f: ExecutorFactory{
				ImmediateSsh: &ImmediateSshExecutor{
					Config: &SshClientConfig{
						Addr: "localhost:24622",
						User: "root",
						Auth: []SshAuth{
							{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
						},
						TimeoutSeconds: 5,
					},
				},
			},
			wantErr: nil,
			want: &ImmediateSshExecutor{
				Config: &SshClientConfig{
					Addr: "localhost:24622",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
				},
			},
		},
		{
			name: "ImmediateSshBad",
			f: ExecutorFactory{
				ImmediateSsh: &ImmediateSshExecutor{},
			},
			wantErr: ErrExecutorBadConfig,
			want:    nil,
		},
		{
			name: "KeepAliveSsh",
			f: ExecutorFactory{
				KeepAliveSsh: &KeepAliveSshExecutor{
					Config: &SshClientConfig{
						Addr: "localhost:24622",
						User: "root",
						Auth: []SshAuth{
							{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
						},
						TimeoutSeconds: 5,
						KeepAlive: SshKeepAliveConfig{
							IntervalSeconds: 10,
						},
					},
				},
			},
			want: &KeepAliveSshExecutor{
				Config: &SshClientConfig{
					Addr: "localhost:24622",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
					KeepAlive: SshKeepAliveConfig{
						IntervalSeconds: 10,
					},
				},
			},
		},
		{
			name: "KeepAliveSshBad",
			f: ExecutorFactory{
				KeepAliveSsh: &KeepAliveSshExecutor{},
			},
			wantErr: ErrExecutorBadConfig,
			want:    nil,
		},
		{
			name: "multiple",
			f: ExecutorFactory{
				Local: &LocalExecutor{},
				Shell: &ShellExecutor{
					ShellPath: "/bin/sh",
					ShellArgs: []string{"-c"},
				},
			},
			wantErr: ErrMultipleExecutors,
			want:    nil,
		},
		{
			name: "multipleBad",
			f: ExecutorFactory{
				Local: &LocalExecutor{},
				Shell: &ShellExecutor{
					// left empty
				},
			},
			wantErr: ErrExecutorBadConfig,
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.f.Executor()

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("❌ Executor() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else {
				t.Logf("✅ Executor() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("❌ Executor() got = %v, want %v", got, tt.want)
			} else {
				t.Logf("✅ Executor() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func ExampleExecutorFactory_Executor() {
	// Create an ExecutorFactory with LocalExecutor
	f := ExecutorFactory{
		Local: &LocalExecutor{},
	}

	// Get the Executor
	executor, err := f.Executor()
	if err != nil {
		panic(err)
	}
	defer executor.Close()

	// Use the Executor
	err = executor.Execute(context.Background(), &Command{
		Command: "echo hello",
		Stdout:  os.Stdout,
	})
	if err != nil {
		panic(err)
	}

	// Output: hello
}

func ExampleExecutorFactory_Executor_fromJson() {
	// Create an ExecutorFactory with ShellExecutor
	jsonConfig := []byte(`{
		"Shell": {
			"ShellPath": "/bin/sh",
			"ShellArgs": ["-c"]
        }
	}`)

	var f ExecutorFactory
	if err := json.Unmarshal(jsonConfig, &f); err != nil {
		panic(err)
	}

	// Get the Executor
	executor, err := f.Executor()
	if err != nil {
		panic(err)
	}
	defer executor.Close()

	// Use the Executor
	err = executor.Execute(context.Background(), &Command{
		Command: "echo hello",
		Stdout:  os.Stdout,
	})
	if err != nil {
		panic(err)
	}

	// Output: hello
}
