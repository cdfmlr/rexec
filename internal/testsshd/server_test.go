package testsshd

import (
	"os"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestNewTestServer_Default(t *testing.T) {
	srv, err := NewTestServer()
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}
	defer srv.Close()

	if srv.Addr() == "" {
		t.Fatal("server address should not be empty")
	}
	t.Logf("✅ Server listening on %s", srv.Addr())

	// Test connection with default credentials (testuser:test)
	client, err := ssh.Dial("tcp", srv.Addr(), &ssh.ClientConfig{
		User: "testuser",
		Auth: []ssh.AuthMethod{
			ssh.Password("test"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	t.Logf("✅ Connected with default credentials")
}

func TestNewTestServerWithConfig_CustomPassword(t *testing.T) {
	srv, err := NewTestServerWithConfig(&Config{
		Users: []User{
			{Username: "myuser", Password: "mypassword"},
		},
	})
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}
	defer srv.Close()

	// Test connection with custom credentials
	client, err := ssh.Dial("tcp", srv.Addr(), &ssh.ClientConfig{
		User: "myuser",
		Auth: []ssh.AuthMethod{
			ssh.Password("mypassword"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	t.Logf("✅ Connected with custom password")
}

func TestNewTestServerWithConfig_CustomPrivateKey(t *testing.T) {
	// Read the test private key
	keyBytes, err := os.ReadFile("../../testsshd/testsshd.id_rsa")
	if err != nil {
		t.Skipf("skipping test: testsshd.id_rsa not found: %v", err)
	}

	srv, err := NewTestServerWithConfig(&Config{
		Users: []User{
			{Username: "root", PrivateKey: keyBytes},
		},
	})
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}
	defer srv.Close()

	// Parse the private key for authentication
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}

	// Test connection with public key auth
	client, err := ssh.Dial("tcp", srv.Addr(), &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	t.Logf("✅ Connected with custom private key")
}

func TestNewTestServerWithConfig_FixedPort(t *testing.T) {
	srv, err := NewTestServerWithConfig(&Config{
		Addr: "127.0.0.1:24622",
		Users: []User{
			{Username: "testuser", Password: "test"},
		},
	})
	if err != nil {
		t.Skipf("skipping test: port 24622 might be in use: %v", err)
	}
	defer srv.Close()

	if srv.Addr() != "127.0.0.1:24622" {
		t.Errorf("expected address 127.0.0.1:24622, got %s", srv.Addr())
	}

	t.Logf("✅ Server listening on fixed port %s", srv.Addr())
}

func TestNewTestServerWithConfig_ExecCommand(t *testing.T) {
	srv, err := NewTestServer()
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}
	defer srv.Close()

	client, err := ssh.Dial("tcp", srv.Addr(), &ssh.ClientConfig{
		User: "testuser",
		Auth: []ssh.AuthMethod{
			ssh.Password("test"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	output, err := session.Output("echo hello")
	if err != nil {
		t.Fatalf("failed to execute command: %v", err)
	}

	expected := "hello\n"
	if string(output) != expected {
		t.Errorf("expected output %q, got %q", expected, string(output))
	}

	t.Logf("✅ Command executed successfully: %q", string(output))
}

func TestNewTestServerWithConfig_WrongPassword(t *testing.T) {
	srv, err := NewTestServerWithConfig(&Config{
		Users: []User{
			{Username: "myuser", Password: "correct"},
		},
	})
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}
	defer srv.Close()

	// Try to connect with wrong password
	_, err = ssh.Dial("tcp", srv.Addr(), &ssh.ClientConfig{
		User: "myuser",
		Auth: []ssh.AuthMethod{
			ssh.Password("wrong"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err == nil {
		t.Fatal("expected connection to fail with wrong password")
	}

	t.Logf("✅ Connection correctly rejected with wrong password: %v", err)
}

func TestNewTestServerWithConfig_MultipleUsers(t *testing.T) {
	// Read the test private key
	keyBytes, err := os.ReadFile("../../testsshd/testsshd.id_rsa")
	if err != nil {
		t.Skipf("skipping test: testsshd.id_rsa not found: %v", err)
	}

	srv, err := NewTestServerWithConfig(&Config{
		Users: []User{
			{Username: "alice", Password: "alice123"},
			{Username: "bob", Password: "bob456"},
			{Username: "root", PrivateKey: keyBytes},
		},
	})
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}
	defer srv.Close()

	// Test alice with password
	aliceClient, err := ssh.Dial("tcp", srv.Addr(), &ssh.ClientConfig{
		User: "alice",
		Auth: []ssh.AuthMethod{
			ssh.Password("alice123"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		t.Fatalf("failed to connect as alice: %v", err)
	}
	aliceClient.Close()
	t.Logf("✅ Alice connected with password")

	// Test bob with password
	bobClient, err := ssh.Dial("tcp", srv.Addr(), &ssh.ClientConfig{
		User: "bob",
		Auth: []ssh.AuthMethod{
			ssh.Password("bob456"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		t.Fatalf("failed to connect as bob: %v", err)
	}
	bobClient.Close()
	t.Logf("✅ Bob connected with password")

	// Test root with private key
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}

	rootClient, err := ssh.Dial("tcp", srv.Addr(), &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		t.Fatalf("failed to connect as root: %v", err)
	}
	rootClient.Close()
	t.Logf("✅ Root connected with private key")

	// Test wrong user
	_, err = ssh.Dial("tcp", srv.Addr(), &ssh.ClientConfig{
		User: "charlie",
		Auth: []ssh.AuthMethod{
			ssh.Password("anypassword"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err == nil {
		t.Fatal("expected connection to fail for non-existent user")
	}
	t.Logf("✅ Non-existent user correctly rejected: %v", err)
}

func TestNewDockerCompatibleServer(t *testing.T) {
	srv, err := NewDockerCompatibleServer()
	if err != nil {
		t.Fatalf("failed to create Docker-compatible server: %v", err)
	}
	defer srv.Close()

	t.Logf("✅ Server listening on %s", srv.Addr())

	// Read the test private key for authentication
	keyBytes, err := os.ReadFile("../../testsshd/testsshd.id_rsa")
	if err != nil {
		t.Logf("Note: testsshd.id_rsa not found, server has fallen back to password auth")
		// Try password auth as fallback
		client, err := ssh.Dial("tcp", srv.Addr(), &ssh.ClientConfig{
			User: "root",
			Auth: []ssh.AuthMethod{
				ssh.Password("test"),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		})
		if err != nil {
			t.Fatalf("failed to connect with fallback password: %v", err)
		}
		defer client.Close()
		t.Logf("✅ Connected with fallback password authentication")

		// Test running a command
		session, err := client.NewSession()
		if err != nil {
			t.Fatalf("failed to create session: %v", err)
		}
		defer session.Close()

		output, err := session.Output("echo hello")
		if err != nil {
			t.Fatalf("failed to execute command: %v", err)
		}

		if string(output) != "hello\n" {
			t.Errorf("expected output %q, got %q", "hello\n", string(output))
		}

		t.Logf("✅ Command executed successfully")
		return
	}

	// Parse the private key for authentication
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}

	// Test connection with public key auth (also try password as fallback)
	client, err := ssh.Dial("tcp", srv.Addr(), &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
			ssh.Password("test"), // fallback
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	t.Logf("✅ Connected with authentication (Docker-compatible)")

	// Test running a command
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	output, err := session.Output("echo hello")
	if err != nil {
		t.Fatalf("failed to execute command: %v", err)
	}

	if string(output) != "hello\n" {
		t.Errorf("expected output %q, got %q", "hello\n", string(output))
	}

	t.Logf("✅ Command executed successfully")
}
