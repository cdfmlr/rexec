package rexec

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

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

// testSshTestServer checks if the testsshd is running.
func testSshTestServer(t *testing.T) {
	t.Helper()

	// hint is an error message when the testsshd is not running.
	hint := `rexec tests require a running testsshd service on localhost:24622.
To start it, run the following commands:

    # install docker and docker-compose-plugin if you don't have them.
	cd ./testsshd
	docker compose -f testsshd-docker-compose.yml up -d

See testsshd/README.md for more details.
`

	// we should avoid using any of the rexec features here
	// because we are testing rexec itself.
	conn, err := ssh.Dial("tcp", "localhost:24622", &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
				key, err := os.ReadFile("./testsshd/testsshd.id_rsa")
				if err != nil {
					return nil, err
				}
				signer, err := ssh.ParsePrivateKey(key)
				if err != nil {
					return nil, err
				}
				return []ssh.Signer{signer}, nil
			}),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	})
	if err != nil {
		t.Fatalf("❌ testsshd is not running on localhost:24622: %v\n\n%s", err, hint)
	}
	defer conn.Close()
	t.Logf("✅ testsshd is running on localhost:24622")
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
