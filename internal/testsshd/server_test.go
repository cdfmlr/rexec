package testsshd

import (
	"os"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestNew_default(t *testing.T) {
	srv, err := New(nil)
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

func TestNew_customPassword(t *testing.T) {
	srv, err := New(&Config{
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

func TestNew_customPrivateKey(t *testing.T) {
	// Read the test private key
	keyBytes, err := os.ReadFile("../../testsshd/testsshd.id_rsa")
	if err != nil {
		t.Skipf("skipping test: testsshd.id_rsa not found: %v", err)
	}

	srv, err := New(&Config{
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

func TestNew_fixedPort(t *testing.T) {
	srv, err := New(&Config{
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

func TestNew_multipleUsers(t *testing.T) {
	// Read the test private key
	keyBytes, err := os.ReadFile("../../testsshd/testsshd.id_rsa")
	if err != nil {
		t.Skipf("skipping test: testsshd.id_rsa not found: %v", err)
	}

	srv, err := New(&Config{
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

func TestServer_execCommand(t *testing.T) {
	srv, err := New(nil)
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

func TestServer_wrongPassword(t *testing.T) {
	srv, err := New(&Config{
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

func TestServer_bothPasswordAndKey(t *testing.T) {
	// Read the test private key
	keyBytes, err := os.ReadFile("../../testsshd/testsshd.id_rsa")
	if err != nil {
		t.Skipf("skipping test: testsshd.id_rsa not found: %v", err)
	}

	srv, err := New(&Config{
		Users: []User{
			{Username: "root", Password: "root", PrivateKey: keyBytes},
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
		t.Fatalf("failed to connect with public key: %v", err)
	}
	client.Close()
	t.Logf("✅ Connected with private key")

	// Test connection with password auth
	client, err = ssh.Dial("tcp", srv.Addr(), &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.Password("root"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		t.Fatalf("failed to connect with password: %v", err)
	}
	client.Close()
	t.Logf("✅ Connected with password")
}
