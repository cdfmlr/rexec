# testsshd - In-Process SSH Server for Testing

This package provides a lightweight, in-process SSH server for testing without requiring Docker or external dependencies.

## Features

- **No Docker required**: Pure Go implementation using `golang.org/x/crypto/ssh`
- **Multiple users**: Support for multiple user accounts with different credentials
- **Flexible authentication**: Password and public key authentication
- **Random ports**: Automatically assigns free ports (or use fixed ports)
- **Command execution**: Executes real shell commands via `sh -c`

## Usage

### Basic Usage (Default Server)

```go
import "github.com/cdfmlr/rexec/v2/internal/testsshd"

func TestMySSHFeature(t *testing.T) {
    // Create server with default settings
    // - Random port (127.0.0.1:0)
    // - Username: "testuser"
    // - Password: "test"
    srv, err := testsshd.NewTestServer()
    if err != nil {
        t.Fatal(err)
    }
    defer srv.Close()

    // Get the actual address
    addr := srv.Addr() // e.g., "127.0.0.1:54321"

    // Connect to it...
}
```

### Custom Configuration

```go
// Single user with password
srv, err := testsshd.NewTestServerWithConfig(&testsshd.Config{
    Addr: "127.0.0.1:0", // random port
    Users: []testsshd.User{
        {Username: "alice", Password: "secret123"},
    },
})

// Multiple users with different auth methods
keyBytes, _ := os.ReadFile("./test_key")
srv, err := testsshd.NewTestServerWithConfig(&testsshd.Config{
    Addr: "127.0.0.1:2222", // fixed port
    Users: []testsshd.User{
        {Username: "alice", Password: "alice123"},
        {Username: "bob", Password: "bob456"},
        {Username: "root", PrivateKey: keyBytes}, // public key auth
    },
})
```

### Docker-Compatible Server

For backward compatibility with existing tests that expect a Docker-based SSH server on port 24622:

```go
// Attempts to start on 127.0.0.1:24622 with root user
// Falls back to random port if 24622 is busy
// Uses ./testsshd/testsshd.id_rsa if available, otherwise password auth
srv, err := testsshd.NewDockerCompatibleServer()
if err != nil {
    t.Fatal(err)
}
defer srv.Close()

// Connect as "root" with either:
// - Private key from ./testsshd/testsshd.id_rsa
// - Password "test" (fallback)
```

## API

### Types

**`User`**
```go
type User struct {
    Username   string // Required
    Password   string // Optional: enables password auth
    PrivateKey []byte // Optional: enables public key auth (PEM format)
}
```

**`Config`**
```go
type Config struct {
    Addr     string      // Default: "127.0.0.1:0" (random port)
    Users    []User      // Default: [{Username: "testuser", Password: "test"}]
    HostKey  ssh.Signer  // Default: auto-generated RSA key
}
```

**`Server`**
```go
type Server struct {
    // ...
}

func (s *Server) Addr() string      // Get actual listening address
func (s *Server) Close() error      // Stop server
```

### Functions

- `NewTestServer() (*Server, error)` - Create server with default settings
- `NewTestServerWithConfig(cfg *Config) (*Server, error)` - Create server with custom config
- `NewDockerCompatibleServer() (*Server, error)` - Create Docker-compatible server

## Examples

See `server_test.go` for comprehensive examples including:
- Default server usage
- Custom passwords
- Public key authentication
- Multiple users
- Wrong credentials rejection
- Command execution

## Notes

- The server executes commands via `sh -c`, so shell features like pipes and redirects work
- Each test gets an isolated server instance
- No cleanup needed - closing the server releases the port immediately
- Perfect for parallel test execution

