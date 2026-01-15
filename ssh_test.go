package rexec

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/cdfmlr/rexec/v2/internal/testsshd"
	"golang.org/x/crypto/ssh"
)

// testsshdType indicates which testsshd setup to use.
type testsshdType string

const (
	testsshdDocker   testsshdType = "docker"   // ./testsshd Docker setup
	testsshdInternal testsshdType = "internal" // ./internal/testsshd dummy server
)

// which testsshd setup to use. It can be configurable by env var:
//
//	REXEC_TESTSSHD=internal go test ./...
//
// The only valid values are "docker" and "internal", case-sensitive,
// other values will cause undefined behavior.  // (actually, it will finally fall back to require manual docker setup.)
var useTestsshd testsshdType = testsshdInternal

var hint = `Hint: rexec tests require a running testsshd service on localhost:24622.

There are two setups for testsshd:

1. "internal" dummy SSH server (pure go and programmatic implementation in internal/testsshd).
2. "docker" container setup in ./testsshd directory (a kind of more realistic OpenSSH server).

The choice of which setup to use is controlled by the environment variable
REXEC_TESTSSHD or the in-code defaults var: useTestsshd.

To use the "internal" testsshd, do configure env var:

    REXEC_TESTSSHD=internal go test -v .

To start the "docker" testsshd, run the following commands:

    # install docker and docker-compose-plugin if you don't have them.
    cd ./testsshd
    docker compose -f testsshd-docker-compose.yml up -d

See testsshd/README.md for more details.
`

func init() {
	if v := os.Getenv("REXEC_TESTSSHD"); v != "" {
		useTestsshd = testsshdType(v)
	}

	if useTestsshd == testsshdInternal {
		slog.Warn("⚙️ using internal testsshd server; it may not behave exactly like a real sshd. Consider using docker testsshd for more realistic tests.")
		err := serveInternalTestsshd(context.Background())
		if err != nil {
			slog.Error("⚙️ failed to start internal testsshd server", "error", err)
			os.Exit(1)
		}
	}
	if err := tryConnectTestsshd(); err != nil {
		slog.Error("❌ testsshd is not running on localhost:24622", "error", err)
		slog.Info(hint)
		os.Exit(1)
	}
}

// serveInternalTestsshd creates and starts an internal/testsshd server that
// mimics the ./testsshd Docker setup used in existing tests:
//
// listening on 127.0.0.1:24622 with "root" user authenticated via private key
// from "./testsshd/testsshd.id_rsa" or password "root".
//
// Returns error if the server fails to start immediately, or
// starts a background goroutine to close the server when the context is done.
func serveInternalTestsshd(ctx context.Context) error {
	keyBytes, err := os.ReadFile("./testsshd/testsshd.id_rsa")
	if err != nil {
		return err
	}

	// the service is started in New(), fuck it sucks.
	srv, err := testsshd.New(&testsshd.Config{
		Addr: "127.0.0.1:24622",
		Users: []testsshd.User{
			{Username: "root", Password: "root", PrivateKey: keyBytes},
		},
	})
	if err != nil {
		return err
	}
	slog.Info("⚙️ internal testsshd started", "addr", srv.Addr())

	go func() {
		select {
		case <-ctx.Done():
			slog.Info("⚙️ close internal testsshd")
			_ = srv.Close()
		}
	}()

	return nil
}

// tryConnectTestsshd dials to the testsshd (ssh://localhost:24622).
// Returns error on failure, nil otherwise.
func tryConnectTestsshd() error {
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
		return fmt.Errorf("testsshd is not running on localhost:24622: %v", err)
	}
	defer conn.Close()

	return nil
}

// testSshTestServer checks if the testsshd is running.
// TODO: rename testSshTestServer -> ensureTestsshd
func testSshTestServer(t *testing.T) {
	t.Helper()

	err := tryConnectTestsshd()
	if err != nil {
		t.Fatalf("❌ tryConnectTestsshd failed: %v", err)
	}
	t.Logf("✅ testsshd is running on localhost:24622")
}

// TestImmediateSshExecutor_closing and Test_testsshd takes writer Lock.
// other test takes a reader RLock.
var testSshMu sync.RWMutex

func Test_testsshd(t *testing.T) {
	testSshMu.Lock()
	defer testSshMu.Unlock()

	testSshTestServer(t)
}
