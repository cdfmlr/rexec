package rexec

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// This file implements an SSH client that will keep the connection alive and
// do automatic reconnection if the connection is lost.

type keepAliveSshClient struct {
	// SshClientConfig is the configuration for the SSH client.
	SshClientConfig *SshClientConfig

	client *ssh.Client // the underlying SSH client.
	mu     sync.Mutex
	closed bool
	wg     sync.WaitGroup
	stopCh chan struct{}
}

// redial the SSH client.
func (c *keepAliveSshClient) redial() {
	logger := Logger.With("addr", c.SshClientConfig.Addr, "user", c.SshClientConfig.User)
	logger.Debug("keepAliveSshClient redialing ssh client")

	client, err := dialSsh(c.SshClientConfig)
	if err != nil {
		logger.Warn("keepAliveSshClient redial ssh client failed", "err", err)
		return
	}

	c.mu.Lock()
	c.client = client
	c.mu.Unlock()

	// redialing is a thing, report it.
	logger.Info("keepAliveSshClient redial ssh client succeeded.", "client", sshClientString(client))
}

// tryKeepAlive sends a keep-alive message to the SSH server.
// It will close the client if the keep-alive fails, which
// will cause redial in keepAlive loop or Client() call.
func (c *keepAliveSshClient) tryKeepAlive() {
	logger := Logger.With("addr", c.SshClientConfig.Addr, "user", c.SshClientConfig.User, "client", sshClientString(c.client))

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client == nil {
		logger.Error("keepAliveSshClient tryKeepAlive failed, client is nil", "tip", "i think this should not happen, please check the code logic.")
		return
	}

	_, _, err := c.client.SendRequest("keepalive@openssh.com", true, nil)
	if err != nil {
		logger.Warn("keep-alive failed, closing client", "err", err)
		_ = c.client.Close()
		c.client = nil
	} else {
		logger.Debug("keep-alive succeeded")
	}
}

// keepAlive loops forever to keep the SSH connection alive until closed.
func (c *keepAliveSshClient) keepAlive() {
	logger := Logger.With("addr", c.SshClientConfig.Addr, "user", c.SshClientConfig.User, "client", sshClientString(c.client))

	defer c.wg.Done()

	retries := 0
	ticker := time.NewTicker(c.SshClientConfig.KeepAlive.interval(0))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			logger.Debug("keepAliveSshClient keepAlive at tick")

			if c.client == nil {
				logger.Debug("keepAliveSshClient redialing...")
				c.redial()
			}

			c.tryKeepAlive()

			if c.client == nil {
				retries++
				interval := c.SshClientConfig.KeepAlive.interval(retries)
				logger.Debug("keepAliveSshClient keepAlive failed, will retry", "retries", retries, "interval", interval)
				ticker.Reset(interval)
			} else if retries != 0 {
				interval := c.SshClientConfig.KeepAlive.interval(0)
				logger.Debug("keepAliveSshClient keepAlive succeeded after retries. Reset retries & interval", "retries", retries, "interval", interval)
				retries = 0
				ticker.Reset(interval)
			}
			// else: keep-alive succeeded, no need to modify the retry or ticker interval.
		case <-c.stopCh:
			logger.Debug("keepAliveSshClient keepAlive stopped")
			return
		}
	}
}

// stopKeepAlive signals the keep-alive routine to stop.
func (c *keepAliveSshClient) stopKeepAlive() {
	logger := Logger.With("addr", c.SshClientConfig.Addr, "user", c.SshClientConfig.User)

	if c.stopCh == nil {
		c.stopCh = make(chan struct{})
	}

	select {
	case <-c.stopCh:
		// Already closed
		logger.Warn("keepAliveSshClient stop already closed")
	default:
		logger.Debug("keepAliveSshClient signals the keep-alive routine to stop")
		close(c.stopCh)
		c.wg.Wait()
	}
	c.stopCh = make(chan struct{})
}

// Client tries to get a living SSH client. It will redial if needed.
func (c *keepAliveSshClient) Client() (*ssh.Client, error) {
	logger := Logger.With("addr", c.SshClientConfig.Addr, "user", c.SshClientConfig.User)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client != nil {
		logger.Debug("keepAliveSshClient client already exists. return it.", "client", sshClientString(c.client))
		return c.client, nil
	}

	logger.Debug("keepAliveSshClient dialing ssh client...")

	client, err := dialSsh(c.SshClientConfig)
	if err != nil {
		logger.Error("keepAliveSshClient dial ssh client failed", "err", err)
		return nil, err
	}

	logger.Info("keepAliveSshClient dial ssh client succeeded", "client", sshClientString(client))

	c.client = client
	c.stopKeepAlive()
	c.wg.Add(1)
	go c.keepAlive()

	logger.Debug("keepAliveSshClient keepAlive started", "client", sshClientString(client))

	return c.client, nil
}

// Close the SSH client and stop the keep-alive loop.
func (c *keepAliveSshClient) Close() error {
	logger := Logger.With("addr", c.SshClientConfig.Addr, "user", c.SshClientConfig.User)
	logger.Debug("keepAliveSshClient closing...")

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		logger.Warn("keepAliveSshClient already closed")
		return ErrAlreadyClosed
	}
	c.closed = true

	c.stopKeepAlive()

	var err error
	if c.client != nil {
		err = c.client.Close()
	}
	c.client = nil

	logger.Info("keepAliveSshClient closed", "err", err)

	return err
}

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

// keep-alive ssh client errors
var (
	// ErrAlreadyClosed is returned when calling Close() on an already closed client.
	ErrAlreadyClosed = fmt.Errorf("already closed")
)
