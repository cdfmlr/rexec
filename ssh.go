package rexec

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// This file implements common SSH login logic,
// including ssh dialing, host key checking, and other helpers.

// The remote exec over SSH is implemented in executor.go.

// // // ssh dialing // // //

// dialSsh is a helper function to prepare authentication methods and
// dial the SSH client.
func dialSsh(config *SshClientConfig) (*ssh.Client, error) {
	authMethods, errs := prepareSshAuthMethods(config.Auth)
	for _, authErr := range errs {
		if authErr != nil {
			// It's totally fine to error here, since there can be multiple auth methods.
			// And if all of them failed, the connection will fail and a well-formed error
			// will be returned by ssh.Dial.
			Logger.Warn("failed to prepare SSH auth methods", "err", authErr)
		}
	}
	hostKeyCheck, err := hostKeyCallback(config.HostKeyCheck)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare SSH host key callback: %w", err)
	}
	clientConfig := &ssh.ClientConfig{
		User:            config.User,
		Auth:            authMethods,
		Timeout:         config.Timeout(),
		HostKeyCallback: hostKeyCheck,
	}

	return ssh.Dial("tcp", config.Addr, clientConfig)
}

// // // host key checking // // //

// hostKeyCallback returns the ssh.HostKeyCallback according to the
// SshHostKeyCheckConfig:
//
//	FixedHostKey > KnownHostsPath > InsecureIgnore > default known_hosts > deny all
//
// Make it a function instead of a method of SshHostKeyCheckConfig is by design
// to allow nil config.
//
// A nil/zero config means using the default known_hosts,
// which trys to read from ~/.ssh/known_hosts and /etc/ssh/ssh_known_hosts if
// exist, or it denies all host keys (which makes all connections fail).
func hostKeyCallback(config *SshHostKeyCheckConfig) (ssh.HostKeyCallback, error) {
	if config == nil {
		return defaultKnownHostsCallback()
	}

	if config.FixedHostKey != "" {
		hostKeyString := strings.TrimSpace(config.FixedHostKey)
		publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(hostKeyString))
		if err != nil {
			return nil, err
		}
		return ssh.FixedHostKey(publicKey), nil
	}

	if len(config.KnownHostsPath) != 0 {
		return knownhosts.New(config.KnownHostsPath...)
	}

	if config.InsecureIgnore {
		return ssh.InsecureIgnoreHostKey(), nil
	}

	return defaultKnownHostsCallback()
}

// defaultKnownHostsCallback returns the ssh.HostKeyCallback that uses the
// default known_hosts file paths (see defaultKnownHostsPaths).
//
// If no known_hosts file exists, it returns a callback that denies all host keys.
func defaultKnownHostsCallback() (ssh.HostKeyCallback, error) {
	knownHostsPaths := defaultKnownHostsPaths()
	if len(knownHostsPaths) == 0 {
		return denyAllHostKeys("make sure you have a known_hosts file at ~/.ssh/known_hosts or /etc/ssh/ssh_known_hosts"), nil
	}

	return knownhosts.New(knownHostsPaths...)
}

// defaultKnownHostsPaths returns the **existing** default known_hosts file paths:
//
//   - ~/.ssh/known_hosts: Hosts the user has logged into that are not already in the systemwide list
//   - /etc/ssh/ssh_known_hosts: Systemwide list of known host keys
func defaultKnownHostsPaths() []string {
	var files []string

	// System-wide known_hosts
	if runtime.GOOS != "windows" {
		files = append(files,
			"/etc/ssh/ssh_known_hosts",
			"/etc/ssh/ssh_known_hosts2",
		)
	}

	// User-specific known_hosts
	if home, err := os.UserHomeDir(); err == nil {
		files = append(files,
			filepath.Join(home, ".ssh", "known_hosts"),
			filepath.Join(home, ".ssh", "known_hosts2"),
		)
	}

	var existingFiles []string
	for _, file := range files {
		if _, err := os.Stat(file); err == nil {
			existingFiles = append(existingFiles, file)
		}
	}

	return existingFiles
}

func denyAllHostKeys(msg string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		return fmt.Errorf("ssh: all host keys are denied: %s", msg)
	}
}

// // // helper methods // // //

// sshClientString returns a string representation of the SSH client.
// For logging purpose.
func sshClientString(client *ssh.Client) string {
	if client == nil {
		return "*ssh.Client(nil)"
	}
	return fmt.Sprintf("*ssh.Client(%x: %s/%s => %s@%s/%s)",
		client.SessionID(),
		client.LocalAddr(), client.ClientVersion(),
		client.User(), client.RemoteAddr(), client.ServerVersion(),
	)
}
