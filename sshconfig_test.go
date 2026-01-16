package rexec

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/cdfmlr/rexec/v2/internal/testsshd"
	"golang.org/x/crypto/ssh"
)

// Prerequisites:
//
//	cd ./testsshd && docker compose -f testsshd-docker-compose.yml up
//
// To start a sshd server on localhost:24622 (see testsshd/README.md for more details).
func TestSshAuth(t *testing.T) {
	testSshMu.RLock()
	defer testSshMu.RUnlock()
	testSshTestServer(t)

	goodPrivateKeyPath := "./testsshd/testsshd.id_rsa"

	goodPrivateKeyPathAuth := &SshAuth{
		PrivateKeyPath: goodPrivateKeyPath,
	}

	goodPrivateKey, err := os.ReadFile(goodPrivateKeyPath)
	if err != nil {
		log.Fatalf("unable to read private key: %v", err)
	}

	goodPrivateKeyAuth := &SshAuth{
		PrivateKey: string(goodPrivateKey),
	}

	goodPasswordAuth := &SshAuth{
		Password: "root",
	}

	tests := []struct {
		name                 string
		auth                 *SshAuth
		expectedPrepareError bool
		expectedSshDialError bool
	}{
		{
			name:                 "goodPrivateKeyPathAuth",
			auth:                 goodPrivateKeyPathAuth,
			expectedPrepareError: false,
			expectedSshDialError: false,
		},
		{
			name:                 "goodPrivateKeyAuth",
			auth:                 goodPrivateKeyAuth,
			expectedPrepareError: false,
			expectedSshDialError: false,
		},
		{
			name:                 "goodPasswordAuth",
			auth:                 goodPasswordAuth,
			expectedPrepareError: false,
			expectedSshDialError: false,
		},
		{
			name: "badPrivateKeyPathPrepare",
			auth: &SshAuth{
				PrivateKeyPath: "./testsshd/thisIsNOTexist.id_rsa",
			},
			expectedPrepareError: true,
			expectedSshDialError: true, // unreachable
		},
		{
			name: "badPrivateKeyPrepare",
			auth: &SshAuth{
				PrivateKey: `-----BEGIN RSA PRIVATE KEY----- thisIsNOTaValidPrivateKey -----END RSA PRIVATE KEY-----`,
			},
			expectedPrepareError: true,
			expectedSshDialError: true, // unreachable
		},
		{
			name: "badPrivateKeyAuth",
			auth: &SshAuth{
				PrivateKey: `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAlwAAAAdzc2gtcn
NhAAAAAwEAAQAAAIEA2kMqLfXIPDqmveN3W//QLmLjegoEn5E4fKEnf3ovKpEYH9VHm9k6
AqxBDZdeqOXZLIVpeaCpGzNyPsg1mR8uWq1D0tYhRjMLnjQAiO2zmSRaZKe1ZfSQfulQfh
0VPs71BLd9orVfMDye8JHZQxhil0VHfZbzNiZ3eIEuiUxLPNEAAAIIV/xQHFf8UBwAAAAH
c3NoLXJzYQAAAIEA2kMqLfXIPDqmveN3W//QLmLjegoEn5E4fKEnf3ovKpEYH9VHm9k6Aq
xBDZdeqOXZLIVpeaCpGzNyPsg1mR8uWq1D0tYhRjMLnjQAiO2zmSRaZKe1ZfSQfulQfh0V
Ps71BLd9orVfMDye8JHZQxhil0VHfZbzNiZ3eIEuiUxLPNEAAAADAQABAAAAgCwk33gSOO
BtoGHRiseRssJe/9EkC5FWZs1WLs3qoXWDiRSPJ3+O7NuziSi9j8irTERj61RNOUamHhoy
lhyVIOOYb8jO7T+KEydVEAN/bwP8g5CsNnKIHCETnFuXG4YeE8/LvgwPajnO/eiO9OUBgZ
VfnLyBRckqjeOie/jm1n3BAAAAQQCN5oxebbjGiH7RwLaalTYeD3oAfPVIzFyo4BQrsgcD
sduRer+UhlFN2gbzaEJzcnptjPdf3r969oW4PX/PITzFAAAAQQD0+vnYYbzGn0gXQ+ckF5
xvvIc+TmWvn5quheLoB5GbtZCdQQr2np+d3e/lrajs3K8kqiVXR7a9S5eyGUhi3DLJAAAA
QQDkFIXQTXFzA9e4gzi2sQfbxhJphBKgKj+4+ivlDgoHRcCNxdt840RT5n+uNXPr7oe19q
rQSJW/+/8V0Qfr5fXJAAAAEnRlc3RlckByZXhlYy5sb2NhbA==
-----END OPENSSH PRIVATE KEY-----
`,
			},
			expectedPrepareError: false,
			expectedSshDialError: true,
		},
		{
			name: "badPasswordAuth",
			auth: &SshAuth{
				Password: "badPassword",
			},
			expectedPrepareError: false,
			expectedSshDialError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// test to prepare the auth method

			err := tt.auth.Prepare()
			if (err != nil) != tt.expectedPrepareError {
				t.Errorf("Prepare() error = %v, expectedPrepareError %v", err, tt.expectedPrepareError)
			} else {
				t.Logf("✅ Prepare() error = %v, expectedPrepareError %v", err, tt.expectedPrepareError)
			}

			if tt.expectedPrepareError {
				return
			}

			// test to dial the ssh server with the auth method

			remote, err := ssh.Dial(
				"tcp", "localhost:24622",
				&ssh.ClientConfig{
					User:            "root",
					HostKeyCallback: ssh.InsecureIgnoreHostKey(),
					Auth: []ssh.AuthMethod{
						tt.auth.AuthMethod(),
					},
				})
			defer func() {
				if remote != nil {
					_ = remote.Close()
				}
			}()

			if (err != nil) != tt.expectedSshDialError {
				t.Errorf("ssh.Dial() error = %v, expectedSshDialError %v", err, tt.expectedSshDialError)
			} else {
				t.Logf("✅ ssh.Dial() error = %v, expectedSshDialError %v", err, tt.expectedSshDialError)
			}

			if tt.expectedSshDialError {
				return
			}

			// test to run a command on the remote server

			s, err := remote.NewSession()
			if err != nil {
				t.Fatalf("unable to create session: %v", err)
			}

			r, err := s.Output("echo hello")
			if err != nil {
				t.Fatalf("unable to run command: %v", err)
			}

			if string(r) != "hello\n" {
				t.Errorf("Output() returned %q, expected %q", r, "hello\n")
			}

			t.Logf("✅ Output() returned %q, expected %q", r, "hello\n")
		})
	}

}

// Prerequisites:
//
//	cd ./testsshd && docker compose -f testsshd-docker-compose.yml up
//
// To start a sshd server on localhost:24622 (see testsshd/README.md for more details).
func TestSshClientConfig_FromJson(t *testing.T) {
	testSshMu.RLock()
	defer testSshMu.RUnlock()
	testSshTestServer(t)

	jsonConfig := `{
		"Addr": "localhost:24622",
		"User": "root",
		"Auth": [
			{"Password": "badPassword"},
			{"PrivateKeyPath": "./testsshd/testsshd.id_rsa"}
		],
		"Timeout": "5s"
	}`

	var config SshClientConfig

	err := json.Unmarshal([]byte(jsonConfig), &config)
	if err != nil {
		t.Fatalf("unable to unmarshal json: %v", err)
	}

	authMethods := make([]ssh.AuthMethod, 0, len(config.Auth))

	for _, auth := range config.Auth {
		err := auth.Prepare()
		if err != nil {
			t.Fatalf("unable to prepare auth: %v", err)
		}
		authMethods = append(authMethods, auth.AuthMethod())
	}

	remote, err := ssh.Dial("tcp", config.Addr,
		&ssh.ClientConfig{
			User:            config.User,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Auth:            authMethods,
			Timeout:         config.Timeout(),
		})
	defer func() {
		if remote != nil {
			_ = remote.Close()
		}
	}()

	if err != nil {
		t.Fatalf("ssh.Dial() error = %v, expectedSshDialError %v", err, false)
	}

	t.Logf("✅ ssh.Dial() error = %v, expectedSshDialError %v", err, false)

	// test to run a command on the remote server

	s, err := remote.NewSession()
	if err != nil {
		t.Fatalf("unable to create session: %v", err)
	}

	r, err := s.Output("echo hello")
	if err != nil {
		t.Fatalf("unable to run command: %v", err)
	}

	if string(r) != "hello\n" {
		t.Errorf("Output() returned %q, expected %q", r, "hello\n")
	}

	t.Logf("✅ Output() returned %q, expected %q", r, "hello\n")
}

func TestHostKey(t *testing.T) {
	// shared test user and host keys

	testUser := testsshd.User{Username: "foo", Password: "bar"}

	hostKey1, err := testsshd.GenerateHostKey()
	if err != nil {
		t.Fatalf("❌ failed to generate a host key: %v", err)
	}
	hostKey2, err := testsshd.GenerateHostKey()
	if err != nil {
		t.Fatalf("❌ failed to generate a host key: %v", err)
	}

	// assert hostKey1 != hostKey2
	if string(hostKey1.PublicKey().Marshal()) == string(hostKey2.PublicKey().Marshal()) {
		t.Fatalf("❌ generated host keys are the same, expected different")
	}

	// sshd1 use hostKey1
	sshd1, err := testsshd.New(&testsshd.Config{
		Users:   []testsshd.User{testUser},
		HostKey: hostKey1,
	})
	if err != nil {
		t.Fatalf("❌ failed to start a random testsshd: %v", err)
	}
	// sshd2 use hostKey2
	sshd2, err := testsshd.New(&testsshd.Config{
		Users:   []testsshd.User{testUser},
		HostKey: hostKey2,
	})
	if err != nil {
		t.Fatalf("❌ failed to start a random testsshd: %v", err)
	}

	t.Run("defaultConfig", func(t *testing.T) {
		testcases := []hostKeyTestcase{
			{
				name:                "nil",
				addr:                sshd1.Addr(),
				user:                testUser,
				checking:            nil,
				expectedError:       true,
				expectedErrContains: []string{"handshake failed", "key is unknown"},
			},
			{
				name:                "empty",
				addr:                sshd2.Addr(),
				user:                testUser,
				checking:            &SshHostKeyCheckConfig{},
				expectedError:       true,
				expectedErrContains: []string{"handshake failed", "key is unknown"},
			},
			{
				name: "zero",
				addr: sshd1.Addr(),
				user: testUser,
				checking: &SshHostKeyCheckConfig{
					FixedHostKey:   "",
					KnownHostsPath: []string{},
					InsecureIgnore: false,
				},
				expectedError:       true,
				expectedErrContains: []string{"handshake failed", "key is unknown"},
			},
		}

		for _, tt := range testcases {
			t.Run(tt.name, func(t *testing.T) {
				testHostKeyCase(t, tt)
			})
		}
	})
	// TODO: test default allow known host keys in ~/.ssh/known_hosts
	t.Run("fixedHostKey", func(t *testing.T) {
		t.Run("testHostKeyParsing", func(t *testing.T) {
			pk := hostKey2.PublicKey()
			t.Logf("hostKey2.PublicKey: type=%s, publicKey=%v", pk.Type(), pk)

			kString := string(ssh.MarshalAuthorizedKey(pk))
			t.Logf("hostKey2.PublicKey -> authorized key: %s", kString)

			// k, err := ssh.ParsePublicKey(kBytes)
			k, _, _, _, err := ssh.ParseAuthorizedKey([]byte(kString))
			if err != nil {
				t.Errorf("❌ failed to parse authorized key: %v", err)
			}
			t.Logf("ssh.ParseAuthorizedKey: type=%s, publicKey=%v", k.Type(), k)

			gotKString := string(ssh.MarshalAuthorizedKey(k))
			if gotKString != kString {
				t.Errorf("❌ parsed key does not match original key: got %q, want %q", gotKString, kString)
			}
			t.Logf("✅ host key parsing works: type=%s", k.Type())
			t.Logf("   original key: %q", kString)
			t.Logf("   parsed   key: %q", gotKString)
		})

		var fixedHostKeyCheckConfig = func(hostKey ssh.Signer) *SshHostKeyCheckConfig {
			ak := string(ssh.MarshalAuthorizedKey(hostKey.PublicKey()))
			return &SshHostKeyCheckConfig{
				FixedHostKey: ak,
			}
		}

		testcases := []hostKeyTestcase{
			{
				name:          "hostKey1_to_sshd1",
				addr:          sshd1.Addr(),
				user:          testUser,
				checking:      fixedHostKeyCheckConfig(hostKey1),
				expectedError: false,
			},
			{
				name:                "hostKey1_to_sshd2",
				addr:                sshd2.Addr(),
				user:                testUser,
				checking:            fixedHostKeyCheckConfig(hostKey1),
				expectedError:       true,
				expectedErrContains: []string{"handshake failed", "host key mismatch"},
			},
			{
				name:                "hostKey2_to_sshd1",
				addr:                sshd1.Addr(),
				user:                testUser,
				checking:            fixedHostKeyCheckConfig(hostKey2),
				expectedError:       true,
				expectedErrContains: []string{"handshake failed", "host key mismatch"},
			},
			{
				name:          "hostKey2_to_sshd2",
				addr:          sshd2.Addr(),
				user:          testUser,
				checking:      fixedHostKeyCheckConfig(hostKey2),
				expectedError: false,
			},
			{
				name:                "badHostKey",
				addr:                sshd1.Addr(),
				user:                testUser,
				checking:            &SshHostKeyCheckConfig{FixedHostKey: "ssh-rsa not-a-valid-key"},
				expectedError:       true,
				expectedErrContains: []string{"failed to prepare SSH host key callback", "no key found"},
			},
			{
				name:                "emptyHostKey",
				addr:                sshd1.Addr(),
				user:                testUser,
				checking:            &SshHostKeyCheckConfig{FixedHostKey: ""},
				expectedError:       true,
				expectedErrContains: []string{"handshake failed", "key is unknown"},
			},
		}

		for _, tt := range testcases {
			t.Run(tt.name, func(t *testing.T) {
				testHostKeyCase(t, tt)
			})
		}
	})
	t.Run("insecureIgnore", func(t *testing.T) {
		testcases := []hostKeyTestcase{
			{
				name: "sshd1_insecureIgnore",
				addr: sshd1.Addr(),
				user: testUser,
				checking: &SshHostKeyCheckConfig{
					InsecureIgnore: true,
				},
				expectedError: false,
			},
			{
				name: "sshd2_insecureIgnore",
				addr: sshd2.Addr(),
				user: testUser,
				checking: &SshHostKeyCheckConfig{
					InsecureIgnore: true,
				},
				expectedError: false,
			},
		}

		for _, tt := range testcases {
			t.Run(tt.name, func(t *testing.T) {
				testHostKeyCase(t, tt)
			})
		}
	})
	t.Run("knownHostsPath", func(t *testing.T) {
		// create a temporary known_hosts file with hostKey1
		goodKnownHostFile, err := os.CreateTemp(t.TempDir(), "known_hosts_")
		if err != nil {
			t.Fatalf("❌ failed to create a temporary known_hosts file: %v", err)
		}
		defer func() {
			_ = goodKnownHostFile.Close()
			_ = os.Remove(goodKnownHostFile.Name())
		}()

		// notice: the format is: "<host> <keytype> <base64-encoded key> [comment]"
		hostKey1Line := fmt.Sprintf("%s %s\n",
			sshd1.Addr(),
			strings.TrimSpace(string(ssh.MarshalAuthorizedKey(hostKey1.PublicKey()))),
		)
		t.Logf("ℹ️ writing line to temporary known_hosts file %s: %s", goodKnownHostFile.Name(), hostKey1Line)
		_, err = goodKnownHostFile.WriteString(hostKey1Line)
		if err != nil {
			t.Fatalf("❌ failed to write to the temporary known_hosts file: %v", err)
		}

		badKnownHostFile, err := os.CreateTemp(t.TempDir(), "known_hosts_")
		if err != nil {
			t.Fatalf("❌ failed to create a temporary known_hosts file: %v", err)
		}
		defer func() {
			_ = badKnownHostFile.Close()
			_ = os.Remove(badKnownHostFile.Name())
		}()
		// write a bad line to the bad known_hosts file
		_, err = badKnownHostFile.WriteString("this is not a valid known_hosts line\n")
		if err != nil {
			t.Fatalf("❌ failed to write to the temporary known_hosts file: %v", err)
		}

		if !strings.Contains(sshd1.Addr(), "127.0.0.1") {
			t.Fatalf("❌ sshd1.Addr() should be 127.0.0.1 for hostname_mismatch test, got: %s", sshd1.Addr())
		}

		testcases := []hostKeyTestcase{
			{
				name: "sshd1_in_knownHostsPath",
				addr: sshd1.Addr(),
				user: testUser,
				checking: &SshHostKeyCheckConfig{
					KnownHostsPath: []string{goodKnownHostFile.Name()},
				},
				expectedError: false,
			},
			{
				name: "hostname_mismatch",
				addr: strings.ReplaceAll(sshd1.Addr(), "127.0.0.1", "localhost"),
				user: testUser,
				checking: &SshHostKeyCheckConfig{
					KnownHostsPath: []string{goodKnownHostFile.Name()},
				},
				expectedError:       true,
				expectedErrContains: []string{"handshake failed", "key is unknown"},
			},
			{
				name:                "sshd2_not_in_knownHostsPath",
				addr:                sshd2.Addr(),
				user:                testUser,
				checking:            &SshHostKeyCheckConfig{KnownHostsPath: []string{goodKnownHostFile.Name()}},
				expectedError:       true,
				expectedErrContains: []string{"handshake failed", "key is unknown"},
			},
			{
				name:                "bad_knownHostsFile",
				addr:                sshd1.Addr(),
				user:                testUser,
				checking:            &SshHostKeyCheckConfig{KnownHostsPath: []string{badKnownHostFile.Name()}},
				expectedError:       true,
				expectedErrContains: []string{"failed to prepare SSH host key callback", "illegal base64 data"},
			},
			{
				name:                "nonexistent_knownHostsFile",
				addr:                sshd1.Addr(),
				user:                testUser,
				checking:            &SshHostKeyCheckConfig{KnownHostsPath: []string{"./this_file_does_not_exist_known_hosts"}},
				expectedError:       true,
				expectedErrContains: []string{"failed to prepare SSH host key callback", "no such file or directory"},
			},
		}

		for _, tt := range testcases {
			t.Run(tt.name, func(t *testing.T) {
				testHostKeyCase(t, tt)
			})
		}
	})
}

type hostKeyTestcase struct {
	name string

	addr     string
	user     testsshd.User
	checking *SshHostKeyCheckConfig

	expectedError       bool
	expectedErrContains []string
}

func testHostKeyCase(t *testing.T, tt hostKeyTestcase) {
	t.Logf("➡️ trying host key checking: addr=%s, hostKeyChecking=%#v",
		tt.addr, tt.checking)

	cfg := &SshClientConfig{
		Addr: tt.addr,
		User: tt.user.Username, Auth: []SshAuth{{Password: tt.user.Password}},
		HostKeyCheck: tt.checking,
	}
	sshClient := &ImmediateSshExecutor{Config: cfg}

	cmd := &Command{Command: "echo hello"}

	io := NewManagedIO()
	io.Hijack(cmd)

	if err := cmd.Validate(); err != nil {
		t.Fatalf("❌ cmd.Validate() failed: %v", err)
	}

	err := sshClient.Execute(context.Background(), cmd)
	if (err != nil) != tt.expectedError {
		t.Errorf("❌ sshClient.Execute() error = %v, expectedError %v",
			err, tt.expectedError)
		return
	}
	for _, expectStr := range tt.expectedErrContains {
		if (err != nil) && !strings.Contains(err.Error(), expectStr) {
			t.Errorf("❌ sshClient.Execute() error = %v, expected to contain %q",
				err, tt.expectedErrContains)
			return
		}
	}

	t.Logf("✅ sshClient.Execute() got expected error %v", err)

	// cmd result check is actually not a part of host key checking test
	if !tt.expectedError {
		stdout := io.Stdout.String()
		if stdout != "hello\n" {
			t.Errorf("❌ stdout got %q, expected %q", stdout, "hello\n")
		} else {
			t.Logf("✅ stdout got expected %q", stdout)
		}
	}
}

// An example to use SshAuth.AuthMethod with
// golang.org/x/crypto/ssh.Dial().
//
// Prerequisites:
//
//	cd ./testsshd && docker compose -f testsshd-docker-compose.yml up
//
// To start a sshd server on localhost:24622 (see testsshd/README.md for more details).
func ExampleSshAuth_AuthMethod() {
	auth := &SshAuth{
		PrivateKeyPath: "./testsshd/testsshd.id_rsa",
	}

	// Prepare the auth method
	if err := auth.Prepare(); err != nil {
		log.Fatalf("unable to prepare auth: %v", err)
	}

	cli, err := ssh.Dial("tcp", "localhost:24622", &ssh.ClientConfig{
		User:            "root",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			auth.AuthMethod(), // AuthMethod is ready to call after Prepare()
		},
	})
	if err != nil {
		log.Fatalf("unable to dial: %v", err)
	}
	s, err := cli.NewSession()
	if err != nil {
		log.Fatalf("unable to create session: %v", err)
	}

	r, err := s.Output("echo hello")
	if err != nil {
		log.Fatalf("unable to run command: %v", err)
	}

	fmt.Println(string(r))

	// Output: hello
}

func ExampleNewSshAuth() {
	auth := NewSshAuth(ssh.Password("root"))

	// Prepare the auth method
	if err := auth.Prepare(); err != nil {
		log.Fatalf("unable to prepare auth: %v", err)
	}

	cli, err := ssh.Dial("tcp", "localhost:24622", &ssh.ClientConfig{
		User:            "root",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			auth.AuthMethod(), // AuthMethod is ready to call after Prepare()
		},
	})
	if err != nil {
		log.Fatalf("unable to dial: %v", err)
	}
	s, err := cli.NewSession()
	if err != nil {
		log.Fatalf("unable to create session: %v", err)
	}

	r, err := s.Output("echo hello")
	if err != nil {
		log.Fatalf("unable to run command: %v", err)
	}

	fmt.Println(string(r))

	// Output: hello
}
