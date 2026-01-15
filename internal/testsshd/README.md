# testsshd - In-Process SSH Server for Testing

This package provides a lightweight, in-process SSH server for testing without requiring external dependencies.

## Features

- **No Docker required**: Pure Go implementation using `golang.org/x/crypto/ssh`
- **Multiple users**: Support for multiple user accounts with different credentials
- **Flexible authentication**: Password and public key authentication
- **Random ports**: Automatically assigns free ports (or use fixed ports)
- **Command execution**: Executes real shell commands via `sh -c` on the local machine (localhost that runs the server)

## Usage

### Basic Usage (Default Server)

```go
import "github.com/cdfmlr/rexec/v2/internal/testsshd"

func TestMySSHFeature(t *testing.T) {
    // Create server with default settings
    // - Random port (127.0.0.1:0)
    // - Username: "testuser"
    // - Password: "test"
    srv, err := testsshd.New(nil)
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
srv, err := testsshd.New(&testsshd.Config{
    Addr: "127.0.0.1:0", // random port
    Users: []testsshd.User{
        {Username: "alice", Password: "secret123"},
    },
})

// Multiple users with different auth methods
keyBytes, _ := os.ReadFile("./test_key")
srv, err := testsshd.New(&testsshd.Config{
    Addr: "127.0.0.1:2222", // fixed port
    Users: []testsshd.User{
        {Username: "alice", Password: "alice123"},
        {Username: "bob", Password: "bob456"},
        {Username: "root", PrivateKey: keyBytes}, // public key auth
    },
})
```

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

