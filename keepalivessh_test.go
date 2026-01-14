package rexec

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// Prerequisites:
//
//	cd ./testsshd && docker compose -f testsshd-docker-compose.yml up
//
// To start a sshd server on localhost:24622 (see testsshd/README.md for more details).
func Test_dialSsh(t *testing.T) {
	testSshMu.RLock()
	defer testSshMu.RUnlock()
	testSshTestServer(t)

	type args struct {
		config *SshClientConfig
	}
	tests := []struct {
		name string
		args args
		// want    *ssh.Client
		wantErr bool
	}{
		{
			name: "good",
			args: args{
				config: &SshClientConfig{
					Addr: "localhost:24622",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
				},
			},
			wantErr: false,
		},
		{
			name: "badHost",
			args: args{
				config: &SshClientConfig{
					Addr: "NotExistHost:24622",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
				},
			},
			wantErr: true,
		},
		{
			name: "badPort",
			args: args{
				config: &SshClientConfig{
					Addr: "localhost:443",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
				},
			},
			wantErr: true,
		},
		{
			name: "badAuth",
			args: args{
				config: &SshClientConfig{
					Addr: "localhost:24622",
					User: "root",
					Auth: []SshAuth{
						{Password: "BADPASSWORD"},
					},
					TimeoutSeconds: 5,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dialSsh(tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("❌ dialSsh() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			t.Logf("✅ got: %v, err: %v", got, err)
			// if !reflect.DeepEqual(got, tt.want) {
			//	t.Errorf("dialSsh() got = %v, want %v", got, tt.want)
			// }
		})
	}
}

// Prerequisites:
//
//	cd ./testsshd && docker compose -f testsshd-docker-compose.yml up
//
// To start a sshd server on localhost:24622 (see testsshd/README.md for more details).
func Test_keepAliveSshClient(t *testing.T) {
	testSshMu.RLock()
	defer testSshMu.RUnlock()
	testSshTestServer(t)

	oldSlogDefault := slog.Default()
	defer slog.SetDefault(oldSlogDefault) // FIXME: it is not restored

	handler := slog.Default().Handler()
	level := new(slog.LevelVar)
	level.Set(slog.LevelDebug)
	handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// runCmdTest runs a command on the SSH client and logs the result.
	runCmdTest := func(client *ssh.Client) {
		session, err := client.NewSession()
		if err != nil {
			t.Errorf("❌ ssh.Client.NewSession() error = %v", err)
		} else {
			t.Logf("✅ session: %v", session)
		}
		defer session.Close()

		_, err = session.Output("echo hello")
		if err != nil {
			t.Errorf("❌ ssh.Session.Output() error = %v", err)
		} else {
			t.Logf("✅ session.Output() success")
		}
	}

	type args struct {
		sshClientConfig *SshClientConfig
	}
	tests := []struct {
		name           string
		skipped        bool
		args           args
		keepAliveDelay time.Duration // wait for keep-alive to kick in
		wantErr        bool
	}{
		{
			name:    "good",
			skipped: false,
			args: args{
				sshClientConfig: &SshClientConfig{
					Addr: "localhost:24622",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
					KeepAlive: SshKeepAliveConfig{
						IntervalSeconds:  3,
						IncrementSeconds: 1,
					},
				},
			},
			keepAliveDelay: 10 * time.Second,
			wantErr:        false,
		},
		{
			name:    "badHost",
			skipped: false,
			args: args{
				sshClientConfig: &SshClientConfig{
					Addr: "NotExistHost:24622",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
					KeepAlive: SshKeepAliveConfig{
						IntervalSeconds:  3,
						IncrementSeconds: 1,
					},
				},
			},
			keepAliveDelay: 10 * time.Second,
			wantErr:        true,
		},
		{
			name:    "badPort",
			skipped: false,
			args: args{
				sshClientConfig: &SshClientConfig{
					Addr: "localhost:443",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
					KeepAlive: SshKeepAliveConfig{
						IntervalSeconds:  3,
						IncrementSeconds: 1,
					},
				},
			},
			keepAliveDelay: 10 * time.Second,
			wantErr:        true,
		},
		{
			name:    "badAuth",
			skipped: false,
			args: args{
				sshClientConfig: &SshClientConfig{
					Addr: "localhost:24622",
					User: "root",
					Auth: []SshAuth{
						{Password: "BADPASSWORD"},
					},
					TimeoutSeconds: 5,
					KeepAlive: SshKeepAliveConfig{
						IntervalSeconds:  3,
						IncrementSeconds: 1,
					},
				},
			},
			keepAliveDelay: 10 * time.Second,
			wantErr:        true,
		},
		{
			name:    "reallyLongKeepAlive",
			skipped: true, // it takes 10 hours to run this test
			args: args{
				sshClientConfig: &SshClientConfig{
					Addr: "localhost:24622",
					User: "root",
					Auth: []SshAuth{
						{PrivateKeyPath: "./testsshd/testsshd.id_rsa"},
					},
					TimeoutSeconds: 5,
					KeepAlive: SshKeepAliveConfig{
						IntervalSeconds:  60,
						IncrementSeconds: 10,
					},
				},
			},
			keepAliveDelay: 10 * time.Hour,
			wantErr:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goroutinesOnTestStart := runtime.NumGoroutine()

			if tt.skipped {
				t.Skip("skipped by tt.skipped == true")
			}

			ka := keepAliveSshClient{
				SshClientConfig: tt.args.sshClientConfig,
			}

			client, err := ka.Client()
			if (err != nil) != tt.wantErr {
				t.Errorf("❌ keepAliveSshClient.Client() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				t.Logf("✅ client: %v, err: %v", sshClientString(client), err)
			}

			if tt.wantErr {
				time.Sleep(1 * time.Second)
				err := ka.Close()
				if err != nil {
					t.Errorf("❌ keepAliveSshClient.Close() error = %v", err)
				} else {
					t.Logf("✅ keepAliveSshClient.Close() success")
				}
				time.Sleep(1 * time.Second)

				goroutinesAfterClose := runtime.NumGoroutine()
				if goroutinesAfterClose > goroutinesOnTestStart {
					t.Errorf("❌ keepAliveSshClient.Close() goroutinesAfterClose > goroutinesOnTestStart: %d > %d", goroutinesAfterClose, goroutinesOnTestStart)
				} else {
					t.Logf("✅ keepAliveSshClient.Close() goroutinesAfterClose <= goroutinesOnTestStart: %d <= %d", goroutinesAfterClose, goroutinesOnTestStart)
				}

				// double close
				err = ka.Close()
				if !errors.Is(err, ErrAlreadyClosed) {
					t.Errorf("❌ keepAliveSshClient.Close() double close error = %v, want %v", err, ErrAlreadyClosed)
				} else {
					t.Logf("✅ keepAliveSshClient.Close() double close success")
				}

				return
			}

			// try to run a command to check if the connection is really good

			// first run
			runCmdTest(client)

			// wait for keep-alive happens
			delay := tt.keepAliveDelay
			if delay <= time.Second {
				delay += time.Second
			}
			t.Logf("sleep for %v to check if the keep-alive is working", delay)
			time.Sleep(delay - 1*time.Second)
			fmt.Print("\a\a\a")
			time.Sleep(1 * time.Second)
			t.Logf("\a\a\awake up after %v. Now check the connection again.", delay)

			// get the client again
			client, err = ka.Client()
			if (err != nil) != tt.wantErr {
				t.Errorf("❌ keepAliveSshClient.Client() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				t.Logf("✅ client: %v, err: %v", sshClientString(client), err)
			}

			if client == nil {
				t.Errorf("❌ keepAliveSshClient.Client() client is nil, want non-nil. err: %v", err)
				return
			}

			// second run
			runCmdTest(client)

			// test close

			time.Sleep(1 * time.Second)
			goroutinesBeforeClose := runtime.NumGoroutine()
			err = ka.Close()
			time.Sleep(1 * time.Second)
			goroutinesAfterClose := runtime.NumGoroutine()
			if err != nil {
				t.Errorf("❌ keepAliveSshClient.Close() error = %v", err)
			} else {
				t.Logf("✅ keepAliveSshClient.Close() success")
			}

			// keep-alive goroutine should be stopped
			if goroutinesAfterClose >= goroutinesBeforeClose {
				t.Errorf("❌ keepAliveSshClient.Close() goroutinesAfterClose >= goroutinesBeforeClose: %d >= %d", goroutinesAfterClose, goroutinesBeforeClose)
			} else {
				t.Logf("✅ keepAliveSshClient.Close() goroutinesAfterClose < goroutinesBeforeClose: %d < %d", goroutinesAfterClose, goroutinesBeforeClose)
			}

			if goroutinesAfterClose > goroutinesOnTestStart {
				t.Errorf("❌ keepAliveSshClient.Close() goroutinesAfterClose > goroutinesOnTestStart: %d > %d", goroutinesAfterClose, goroutinesOnTestStart)
			} else {
				t.Logf("✅ keepAliveSshClient.Close() goroutinesAfterClose <= goroutinesOnTestStart: %d <= %d", goroutinesAfterClose, goroutinesOnTestStart)
			}

			// double close
			err = ka.Close()
			if !errors.Is(err, ErrAlreadyClosed) {
				t.Errorf("❌ keepAliveSshClient.Close() double close error = %v, want %v", err, ErrAlreadyClosed)
			} else {
				t.Logf("✅ keepAliveSshClient.Close() double close success")
			}
		})
	}
}

func Test_keepAlive_interval(t *testing.T) {
	type fields struct {
		IntervalSeconds  int
		IncrementSeconds int
	}
	type args struct {
		retries int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   time.Duration
	}{
		{
			name: "good",
			fields: fields{
				IntervalSeconds:  3,
				IncrementSeconds: 1,
			},
			args: args{retries: 4},
			want: 7 * time.Second,
		},
		{
			name: "negativeInterval",
			fields: fields{
				IntervalSeconds:  -3,
				IncrementSeconds: 1,
			},
			args: args{retries: 0},
			want: MinSshKeepAliveInterval,
		},
		{
			name: "negativeRetries",
			fields: fields{
				IntervalSeconds:  3,
				IncrementSeconds: 1,
			},
			args: args{retries: -1},
			want: 3 * time.Second,
		},
		{
			name: "negativeIncrement",
			fields: fields{
				IntervalSeconds:  3,
				IncrementSeconds: -1,
			},
			args: args{retries: 1},
			want: 2 * time.Second,
		},
		{
			name: "negativeFinal",
			fields: fields{
				IntervalSeconds:  3,
				IncrementSeconds: -1,
			},
			args: args{retries: 10},
			want: MinSshKeepAliveInterval,
		},
		{
			name: "zeroFirstRetry",
			fields: fields{
				IntervalSeconds:  0,
				IncrementSeconds: 0,
			},
			args: args{retries: 0},
			want: MinSshKeepAliveInterval,
		},
		{
			name: "zeroRetries",
			fields: fields{
				IntervalSeconds:  0,
				IncrementSeconds: 0,
			},
			args: args{retries: 8},
			want: MinSshKeepAliveInterval,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := SshKeepAliveConfig{
				IntervalSeconds:  tt.fields.IntervalSeconds,
				IncrementSeconds: tt.fields.IncrementSeconds,
			}
			if got := c.interval(tt.args.retries); got != tt.want {
				t.Errorf("❌ SshKeepAliveConfig.interval() = %v, want %v", got, tt.want)
			} else {
				t.Logf("✅ SshKeepAliveConfig.interval() = %v", got)
			}
		})
	}
}

func Fuzz_keepAlive_interval(f *testing.F) {
	f.Add(60, 10, 0)
	f.Add(10, 3, 5)
	f.Add(3, -1, 4)
	f.Add(0, 0, 0)

	f.Fuzz(func(t *testing.T, intervalSeconds int, incrementSeconds int, retries int) {
		c := SshKeepAliveConfig{
			IntervalSeconds:  intervalSeconds,
			IncrementSeconds: incrementSeconds,
		}

		intervalDuration := time.Duration(intervalSeconds) * time.Second
		minInterval := max(intervalDuration, MinSshKeepAliveInterval)

		got := c.interval(retries)

		if got <= 0 { // NewTicker panic("non-positive interval for NewTicker")
			t.Errorf("❌ SshKeepAliveConfig.interval() = %v <= 0", got)
		}

		if got < MinSshKeepAliveInterval {
			t.Errorf("❌ SshKeepAliveConfig.interval() = %v < %v", got, MinSshKeepAliveInterval)
		}

		if retries <= 0 && got != minInterval {
			t.Errorf("❌ SshKeepAliveConfig.interval() = %v, want = %v", got, minInterval)
		}

		if incrementSeconds == 0 && got != minInterval {
			t.Errorf("❌ SshKeepAliveConfig.interval() = %v, want = %v", got, minInterval)
		}

		if retries > 0 && incrementSeconds > 0 && got < minInterval {
			t.Errorf("❌ SshKeepAliveConfig.interval() = %v, want >= %v", got, minInterval)
		}

		if retries > 0 && incrementSeconds < 0 && got > minInterval {
			t.Errorf("❌ SshKeepAliveConfig.interval() = %v, want <= %v", got, minInterval)
		}
	})
}
