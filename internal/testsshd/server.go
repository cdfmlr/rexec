package testsshd

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net"
	"os/exec"

	"golang.org/x/crypto/ssh"
)

// Server is a simple SSH server for testing purposes.
//
// It supports password and public key authentication.
// Executed commands are run locally on the host machine.
//
// The zero value or literal is not usable. Use New to create a decent server.
type Server struct {
	listener net.Listener
	config   *ssh.ServerConfig
}

// Config for the test SSH server.
type Config struct {
	// Addr is the address to listen on. Use "127.0.0.1:0" for a random port.
	// Default: "127.0.0.1:0"
	Addr string

	// Users is the list of users to accept. If empty, a default user "testuser:test" is created.
	Users []User

	// HostKey is the private key for the server. If nil, a new RSA key is generated.
	HostKey ssh.Signer
}

// User account on the test SSH server.
type User struct {
	// Username is the username to accept.
	Username string

	// Password is the password to accept. If empty, password auth is disabled for this user.
	Password string

	// PrivateKey is the PEM-encoded private key for public key authentication.
	// If empty, public key auth is disabled for this user.
	PrivateKey []byte
}

// New creates an SSH server with custom configuration.
//
// Pass nil to use default settings: creates an SSH server with random port, user "testuser" and password "test".
//
// New starts the service immediately in a new goroutine and returns the Server instance.
func New(cfg *Config) (*Server, error) {
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

// Port returns the port number the server is listening on.
// If the port cannot be determined, it returns -1.
func (s *Server) Port() int {
	addr, ok := s.listener.Addr().(*net.TCPAddr)
	if ok {
		return addr.Port
	}
	// Fallback: parse from string
	_, portStr, err := net.SplitHostPort(s.listener.Addr().String())
	if err == nil && portStr != "" {
		var port int
		if _, err := fmt.Sscanf(portStr, "%d", &port); err == nil {
			return port
		}
	}
	// ???
	return -1
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
			var payload struct{ Cmd string }
			ssh.Unmarshal(req.Payload, &payload)
			req.Reply(true, nil)
			cmd := exec.Command("sh", "-c", payload.Cmd)
			cmd.Stdin = ch
			cmd.Stdout = ch
			cmd.Stderr = ch.Stderr()

			status := struct{ Status uint32 }{Status: 0}
			if err := cmd.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					status.Status = uint32(exitErr.ExitCode())
				} else {
					status.Status = 1
				}
			}

			ch.SendRequest("exit-status", false, ssh.Marshal(status))
			return
		}
		req.Reply(false, nil)
	}
}

// Generate an ephemeral RSA key for the SSH server host key
func generateHostKey() (ssh.Signer, error) {
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
