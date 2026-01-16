package rexec

import (
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SshClientConfig contains the configuration for the SSH client.
//
// It is a wrapper around ssh.ClientConfig plus the address of the remote host
// to make it easier to bind values from sources like configuration files.
type SshClientConfig struct {
	// Addr is the address of the remote host: "host:port".
	Addr string
	// User contains the username to authenticate as.
	User string
	// Auth contains the authentication methods to use.
	Auth []SshAuth
	// TimeoutSeconds is the maximum amount of time for the TCP connection to
	// establish. A Timeout of zero means no timeout.
	TimeoutSeconds int
	// KeepAlive contains the configuration for the SSH client to keep the
	// connection alive.
	// As for now, only KeepAliveSshExecutor supports this.
	KeepAlive SshKeepAliveConfig

	// HostKeyCheck is the configuration for host key checking.
	// If nil, host key checking is disabled (insecure, do not use in production).
	// If not nil, host key checking is enabled according to the configuration.
	HostKeyCheck *SshHostKeyCheckConfig
}

// SshHostKeyCheckConfig contains the configuration for host key checking.
//
// One of FixedHostKey or KnownHostsPath should be set to enable host key
// checking.
//
// A nil/zero config means using the default known_hosts,
// which trys to read from ~/.ssh/known_hosts and /etc/ssh/ssh_known_hosts if
// exist, or it denies all host keys (which makes all connections fail).
//
// If multiple fields are set, the priority is:
//
//	FixedHostKey > KnownHostsPath
//
// That is, the first non-empty field will be used for host key checking, and
// the rest will be ignored.
type SshHostKeyCheckConfig struct {
	// FixedHostKey is an "ssh-ed25519 ..." you got from
	// `ssh-keyscan <server-ip>` (excluding the IP address part)
	FixedHostKey string
	// KnownHostsPath is a list of paths to the known_hosts files,
	// usually ~/.ssh/known_hosts and /etc/ssh/ssh_known_hosts
	KnownHostsPath []string
	// InsecureIgnore can be set to true to disable host key checking.
	// Insecure, do not use in production.
	InsecureIgnore bool
}

// Timeout converts the TimeoutSeconds to time.Duration.
func (c SshClientConfig) Timeout() time.Duration {
	return time.Duration(c.TimeoutSeconds) * time.Second
}

// validateSshClientConfig checks if SshClientConfig is not nil or
// contains empty Addr.
func validateSshClientConfig(c *SshClientConfig) error {
	if c == nil {
		return fmt.Errorf("nil ssh client config")
	}
	if c.Addr == "" {
		return fmt.Errorf("addr is empty")
	}
	// user is not required.
	// if c.User == "" {
	//	return fmt.Errorf("user is empty")
	// }
	// so is auth.
	// if len(c.Auth) == 0 {
	//	return fmt.Errorf("auth is empty")
	// }
	return nil
}

// SshKeepAliveConfig contains the configuration for the SSH client to keep the
// connection alive.
//
// The final interval between keep-alive will be:
//
//	max(IntervalSeconds + IncrementSeconds * retries, MinSshKeepAliveInterval)
//
// Special cases:
//   - If IntervalSeconds < 0, it will be defaulted to 0.
//   - If IncrementSeconds == 0, the interval will be fixed.
//   - If IncrementSeconds < 0, the interval will be decreased.
//   - If the calculated interval is less than MinSshKeepAliveInterval, it will
//     be defaulted to MinSshKeepAliveInterval.
type SshKeepAliveConfig struct {
	IntervalSeconds  int // the initial interval between keep-alive, in seconds
	IncrementSeconds int // the increment of interval between keep-alive, in seconds
}

// MinSshKeepAliveInterval is the minimum interval between keep-alive.
// This is used as the minimum return value for the interval() function.
var MinSshKeepAliveInterval = 1 * time.Second

// interval calculates the final interval between keep-alive:
//
//	max(IntervalSeconds + IncrementSeconds * retries, MinSshKeepAliveInterval)
func (c SshKeepAliveConfig) interval(retries int) time.Duration {
	if c.IntervalSeconds < 0 {
		c.IntervalSeconds = 0
	}
	if retries < 0 { // non-sense
		retries = 0
	}

	i := time.Duration(c.IntervalSeconds) * time.Second
	i += time.Duration(c.IncrementSeconds) * time.Second * time.Duration(retries)

	if i < MinSshKeepAliveInterval {
		i = MinSshKeepAliveInterval
	}

	return i
}

// SshAuth wraps the ssh.AuthMethod to make it easier to bind values from
// configuration files or databases.
//
// It's OK to construct it manually, by
//
//	auth := &SshAuth{Password: "password"}
//
// Set exactly one of Password, PrivateKey, PrivateKeyPath field to
// authenticate with RFC 4252 password or public key authentication.
//
// For other authentication methods, use NewSshAuth() to set a custom auth
// method.
type SshAuth struct {
	// Password is the password to use for authentication.
	Password string

	// PrivateKey is the private key to use for authentication.
	PrivateKey string
	// PrivateKeyPath is the path to the private key to use for authentication.
	PrivateKeyPath string

	// Retries is the number of times to retry the connection for this auth method.
	// If Retries < 0, will retry indefinitely.
	Retries int

	// authMethod is the prepared auth method.
	// Or it is possible to set a custom ssh.AuthMethod by calling NewSshAuth().
	authMethod ssh.AuthMethod
}

// NewSshAuth returns a new SshAuth wrapping the given underlying ssh.AuthMethod.
// It is useful to set a custom auth method that is not covered by Password,
// PrivateKey, or PrivateKeyPath.
//
// Example:
//
//	auth := NewSshAuth(ssh.PasswordCallback(func() (string, error) {
//		return "password", nil
//	})
func NewSshAuth(authMethod ssh.AuthMethod) *SshAuth {
	return &SshAuth{
		authMethod: authMethod,
	}
}

// Prepare prepares the SshAuth for AuthMethod() call.
func (a *SshAuth) Prepare() (err error) {
	if a.authMethod != nil {
		if a.Password != "" || a.PrivateKey != "" || a.PrivateKeyPath != "" {
			return ErrSshAuthMutex
		}
		return nil
	}

	if a.Password != "" {
		if a.PrivateKey != "" || a.PrivateKeyPath != "" {
			return ErrSshAuthMutex
		}
		a.Password = strings.TrimSpace(a.Password)
		if a.Password == "" {
			return ErrSshAuthEmptyPassword
		}

		a.authMethod = ssh.Password(a.Password)

		return nil
	}

	if a.PrivateKey != "" && a.PrivateKeyPath != "" {
		return ErrSshAuthMutex
	}

	// if PrivateKeyPath is set, read the private key from the file, and set PrivateKey.
	if a.PrivateKeyPath != "" {
		key, err := os.ReadFile(a.PrivateKeyPath)
		if err != nil {
			// log.Fatalf("unable to read private key: %v", err)
			return fmt.Errorf("unable to read private key: %w", err)
		}
		if len(key) == 0 {
			return ErrSshAuthEmptyPrivateKey
		}
		a.PrivateKey = string(key)
	}

	// parse the private key, set signer.
	if a.PrivateKey != "" {
		a.PrivateKey = strings.TrimSpace(a.PrivateKey)
		if a.PrivateKey == "" {
			return ErrSshAuthEmptyPrivateKey
		}

		key := []byte(a.PrivateKey)
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			// log.Fatalf("unable to parse private key: %v", err)
			return fmt.Errorf("unable to parse private key: %w", err)
		}

		a.authMethod = ssh.PublicKeys(signer)

		return nil
	}

	// none of the auth methods are set.
	return ErrSshAuthMutex
}

// AuthMethod returns the prepared ssh.AuthMethod.
// It panics if Prepare() was not called before.
func (a *SshAuth) AuthMethod() ssh.AuthMethod {
	if a.authMethod == nil {
		// if err := a.Prepare(); err != nil {
		//	panic(err)
		// }
		// always panic to force the user to call Prepare()
		panic("AuthMethod called before prepare()")
	}

	var am = a.authMethod

	if a.Retries != 0 {
		am = ssh.RetryableAuthMethod(am, a.Retries)
	}

	return am
}

// SshAuth errors that can be returned by Prepare().
var (
	ErrSshAuthMutex           = fmt.Errorf("exactly one of Password, PrivateKey, PrivateKeyPath must be set or use NewSshAuth() to set a custom auth method")
	ErrSshAuthEmptyPassword   = fmt.Errorf("password is empty")
	ErrSshAuthEmptyPrivateKey = fmt.Errorf("private key is empty")
)

func prepareSshAuthMethods(auths []SshAuth) ([]ssh.AuthMethod, []error) {
	authMethods := make([]ssh.AuthMethod, 0, len(auths))
	errs := make([]error, 0)

	for _, auth := range auths {
		err := auth.Prepare()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		authMethods = append(authMethods, auth.AuthMethod())
	}

	return authMethods, errs
}
