package rexec

import (
	"context"
	"fmt"
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

// the tests will first try preferredTestsshd, and fall back to
// fallbackTestsshd if the first one is not available.
//
//   - set both to testsshdInternal to always use the internal server.
//   - set both to testsshdDocker to always use the Docker setup (the legacy v1
//     behavior; tests will fail if the container is not running).
//
// They are configurable by env var REXEC_TESTSSHD_PREFERRED and
// REXEC_TESTSSHD_FALLBACK, e.g.
//
//	REXEC_TESTSSHD_PREFERRED=internal REXEC_TESTSSHD_FALLBACK=docker go test ./...
//
// The only valid values are "docker" and "internal", case-sensitive,
// other values will cause undefined behavior.  // (actually, it will finally fallback to require manual docker setup.)
var (
	preferredTestsshd = testsshdDocker
	fallbackTestsshd  = testsshdInternal
)

func init() {
	if v := os.Getenv("REXEC_TESTSSHD_PREFERRED"); v != "" {
		preferredTestsshd = testsshdType(v)
	}
	if v := os.Getenv("REXEC_TESTSSHD_FALLBACK"); v != "" {
		fallbackTestsshd = testsshdType(v)
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

	go func() {
		select {
		case <-ctx.Done():
			_ = srv.Close()
		}
	}()

	return nil
}

// setupTestsshd sets up the testsshd according to the preferred and fallback types.
//
// Only internal/testsshd may be created at this time.
//
// It creates and starts an internal dummy SSH server instance only if
//
//  1. preferredTestsshd is testsshdInternal
//  2. preferredTestsshd is not available and fallbackTestsshd is testsshdInternal
//
// The testsshdDocker is always requires manually setup.
//
// Accepts a context to clean up the internal dummy server when the context is done.
func setupTestsshd(ctx context.Context, t *testing.T) error {
	t.Helper()

	if preferredTestsshd == testsshdInternal {
		t.Logf("⚙️  starting internal testsshd server as preferredTestsshd=%v", testsshdInternal)
		return serveInternalTestsshd(ctx)
	}
	if fallbackTestsshd != testsshdInternal {
		return nil
	}
	if err := tryConnectTestsshd(); err != nil {
		t.Logf("⚙️  starting internal testsshd server as fallbackTestsshd=%v and the preferred (%v) setup is not running: %v", testsshdInternal, preferredTestsshd, err)
		return serveInternalTestsshd(ctx)
	}
	return nil
}

// tryConnectTestsshd dails to the testsshd (ssh://localhost:24622).
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
func testSshTestServer(t *testing.T) (cancel context.CancelFunc) {
	t.Helper()

	// create a context to clean up internal testsshd server if any.
	ctx, cancel := context.WithCancel(context.Background())
	err := setupTestsshd(ctx, t)
	if err != nil {
		cancel()
		t.Errorf("⚠️  setupTestsshd failed: %v", err)
	}

	// hint is an error message when the testsshd is not running.
	hint := `Hint: rexec tests require a running testsshd service on localhost:24622.

There are two setups for testsshd:

1. "internal" dummy SSH server (pure go and programmatic implementation in internal/testsshd).
2. "docker" container setup in ./testsshd directory (a kind of more realistic OpenSSH server).

The choice of which setup to use is controlled by the environment variables
REXEC_TESTSSHD_PREFERRED and REXEC_TESTSSHD_FALLBACK,
or the in-code defaults var: preferredTestsshd, fallbackTestsshd.

The current settings make it failed to connect to either setup.

To use the "internal" testsshd, do configure env vars or the in-code defaults properly.

To start the "docker" testsshd, run the following commands:

    # install docker and docker-compose-plugin if you don't have them.
	cd ./testsshd
	docker compose -f testsshd-docker-compose.yml up -d

See testsshd/README.md for more details.
`
	err = tryConnectTestsshd()
	if err != nil {
		cancel()
		t.Fatalf("❌ tryConnectTestsshd failed: %v\n\n%s", err, hint)
	}
	t.Logf("✅ testsshd is running on localhost:24622")

	return cancel
}

// TestImmediateSshExecutor_closing and Test_testsshd takes writer Lock.
// other test takes a reader RLock.
var testSshMu sync.RWMutex

func Test_testsshd(t *testing.T) {
	testSshMu.Lock()
	defer testSshMu.Unlock()

	cancel := testSshTestServer(t)
	defer cancel()
}
