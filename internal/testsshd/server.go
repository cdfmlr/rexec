package testsshd

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net"
	"os"
	"os/exec"

	"golang.org/x/crypto/ssh"
)

type Server struct {
	listener net.Listener
	config   *ssh.ServerConfig
}

// User represents a user account on the test SSH server.
type User struct {
	// Username is the username to accept.
	Username string

	// Password is the password to accept. If empty, password auth is disabled for this user.
	Password string

	// PrivateKey is the PEM-encoded private key for public key authentication.
	// If empty, public key auth is disabled for this user.
	PrivateKey []byte
}

// Config holds the configuration for the test SSH server.
type Config struct {
	// Addr is the address to listen on. Use "127.0.0.1:0" for a random port.
	// Default: "127.0.0.1:0"
	Addr string

	// Users is the list of users to accept. If empty, a default user "testuser:test" is created.
	Users []User

	// HostKey is the private key for the server. If nil, a new RSA key is generated.
	HostKey ssh.Signer
}

// NewTestServer creates an SSH server with default settings (random port, password "test").
func NewTestServer() (*Server, error) {
	return NewTestServerWithConfig(nil)
}

// Deprecated: TODO: NOT THE BUSINESS OF THIS PACKAGE.
// NewDockerCompatibleServer creates an SSH server that mimics the Docker testsshd
// setup used in existing tests: listening on 127.0.0.1:24622 with root user
// authenticated via ./testsshd/testsshd.id_rsa private key.
//
// This is a convenience function to replace the Docker-based test server without
// changing existing test code.
//
// If the private key file is not found or port 24622 is busy, it falls back to
// a random port with default password authentication.
func NewDockerCompatibleServer() (*Server, error) {
	keyBytes, err := os.ReadFile("./testsshd/testsshd.id_rsa")
	if err != nil {
		// Fall back to default if key not found
		return NewTestServerWithConfig(&Config{
			Addr: "127.0.0.1:0",
			Users: []User{
				{Username: "root", Password: "test"},
			},
		})
	}

	cfg := &Config{
		Addr: "127.0.0.1:24622",
		Users: []User{
			{Username: "root", PrivateKey: keyBytes},
		},
	}

	srv, err := NewTestServerWithConfig(cfg)
	if err != nil {
		// Fall back to random port if 24622 is busy
		cfg.Addr = "127.0.0.1:0"
		return NewTestServerWithConfig(cfg)
	}

	return srv, nil
}

// NewTestServerWithConfig creates an SSH server with custom configuration.
func NewTestServerWithConfig(cfg *Config) (*Server, error) {
	if cfg == nil {
		cfg = &Config{}
	}

	// Apply defaults
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:0"
	}
	if len(cfg.Users) == 0 {
		// Default user if none specified
		cfg.Users = []User{
			{Username: "testuser", Password: "test"},
		}
	}

	sshConfig := &ssh.ServerConfig{}

	// Build maps of users and their credentials for quick lookup
	passwordUsers := make(map[string]string)         // username -> password
	publicKeyUsers := make(map[string]ssh.PublicKey) // username -> public key

	for _, user := range cfg.Users {
		if user.Password != "" {
			passwordUsers[user.Username] = user.Password
		}
		if user.PrivateKey != nil {
			signer, err := ssh.ParsePrivateKey(user.PrivateKey)
			if err != nil {
				return nil, fmt.Errorf("failed to parse private key for user %q: %w", user.Username, err)
			}
			publicKeyUsers[user.Username] = signer.PublicKey()
		}
	}

	// Setup password authentication if any user has a password
	if len(passwordUsers) > 0 {
		sshConfig.PasswordCallback = func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if expectedPass, ok := passwordUsers[c.User()]; ok && expectedPass == string(pass) {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for user %q", c.User())
		}
	}

	// Setup public key authentication if any user has a public key
	if len(publicKeyUsers) > 0 {
		sshConfig.PublicKeyCallback = func(c ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
			if authorizedKey, ok := publicKeyUsers[c.User()]; ok && string(pubKey.Marshal()) == string(authorizedKey.Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("public key rejected for user %q", c.User())
		}
	}

	// Setup host key
	hostKey := cfg.HostKey
	if hostKey == nil {
		var err error
		hostKey, err = generateHostKey()
		if err != nil {
			return nil, err
		}
	}
	sshConfig.AddHostKey(hostKey)

	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return nil, err
	}

	s := &Server{listener: listener, config: sshConfig}
	go s.serve()
	return s, nil
}

func (s *Server) Addr() string {
	return s.listener.Addr().String()
}

func (s *Server) Close() error {
	return s.listener.Close()
}

func (s *Server) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(netConn net.Conn) {
	sshConn, chans, reqs, err := ssh.NewServerConn(netConn, s.config)
	if err != nil {
		return
	}
	defer sshConn.Close()

	go ssh.DiscardRequests(reqs)

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			newChan.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		ch, reqs, _ := newChan.Accept()
		go handleSession(ch, reqs)
	}
}

func handleSession(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()
	for req := range reqs {
		if req.Type == "exec" {
			cmd := string(req.Payload[4:]) // skip length prefix
			req.Reply(true, nil)

			// Execute command locally or return mock output
			out, _ := exec.Command("sh", "-c", cmd).CombinedOutput()
			ch.Write(out)
			ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
			return
		}
		req.Reply(false, nil)
	}
}

func generateHostKey() (ssh.Signer, error) {
	// Generate an ephemeral RSA key for the SSH server host key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create signer from key: %w", err)
	}

	return signer, nil
}
