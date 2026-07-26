// SPDX-License-Identifier: MIT
//
// Adapted from Maxime Rivest's remagic pure-Go device package.
package device

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const DefaultUSBAddr = "10.11.99.1"

type Device struct {
	Addr   string
	client *ssh.Client
}

func nonInteractiveAuths() []ssh.AuthMethod {
	var auths []ssh.AuthMethod
	if socket := os.Getenv("SSH_AUTH_SOCK"); socket != "" {
		if connection, err := net.Dial("unix", socket); err == nil {
			auths = append(auths, ssh.PublicKeysCallback(agent.NewClient(connection).Signers))
		}
	}
	home, _ := os.UserHomeDir()
	for _, name := range []string{"id_ed25519", "id_rsa"} {
		privateKey, err := os.ReadFile(filepath.Join(home, ".ssh", name))
		if err != nil {
			continue
		}
		if signer, err := ssh.ParsePrivateKey(privateKey); err == nil {
			auths = append(auths, ssh.PublicKeys(signer))
		}
	}
	return auths
}

// Connect uses local SSH keys first and the explicitly supplied developer-mode
// password second. The password is not persisted. reMarkable regenerates its
// host key after a factory reset, matching remagic's trust-on-connect policy.
func Connect(address, password string) (*Device, error) {
	auths := nonInteractiveAuths()
	if password != "" {
		auths = append(auths, ssh.Password(password))
	}
	if len(auths) == 0 {
		return nil, fmt.Errorf("no SSH key or developer-mode password is available")
	}
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // See comment above.
		Timeout:         8 * time.Second,
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort(address, "22"), config)
	if err != nil {
		return nil, fmt.Errorf("root@%s: %w", address, err)
	}
	return &Device{Addr: address, client: client}, nil
}

func (d *Device) Close() {
	if d.client != nil {
		_ = d.client.Close()
	}
}

func (d *Device) Run(command string) (string, error) {
	return d.RunIn(command, nil)
}

func (d *Device) RunIn(command string, input io.Reader) (string, error) {
	session, err := d.client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	var output bytes.Buffer
	writer := &lockedWriter{writer: &output}
	session.Stdout = writer
	session.Stderr = writer
	session.Stdin = input
	err = session.Run(command)
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return output.String(), err
}

func (d *Device) RunStreaming(
	ctx context.Context,
	command string,
	input io.Reader,
	onOutput func(string),
) (string, error) {
	session, err := d.client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	var output bytes.Buffer
	writer := &lockedWriter{writer: &output, onWrite: onOutput}
	session.Stdout = writer
	session.Stderr = writer
	session.Stdin = input
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = session.Close()
			// Dropbear can leave a foreground process alive when only the SSH
			// channel is closed. Closing the transport makes cancellation
			// observable by the remote session and also guarantees Run returns.
			_ = d.client.Close()
		case <-done:
		}
	}()
	err = session.Run(command)
	close(done)
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if ctx.Err() != nil {
		return output.String(), ctx.Err()
	}
	return output.String(), err
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

// Push streams through cat so the Move does not need an SFTP subsystem.
func (d *Device) Push(input io.Reader, remotePath string, mode os.FileMode) error {
	if !strings.HasPrefix(remotePath, "/") {
		return fmt.Errorf("remote path must be absolute")
	}
	parent := filepath.ToSlash(filepath.Dir(remotePath))
	command := fmt.Sprintf(
		"mkdir -p %s && cat > %s && chmod %04o %s",
		shellQuote(parent),
		shellQuote(remotePath),
		mode.Perm(),
		shellQuote(remotePath),
	)
	output, err := d.RunIn(command, input)
	if err != nil {
		return fmt.Errorf("upload %s: %w: %s", remotePath, err, strings.TrimSpace(output))
	}
	return nil
}

func (d *Device) PushFile(localPath, remotePath string, mode os.FileMode) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()
	return d.Push(file, remotePath, mode)
}

type lockedWriter struct {
	mu      sync.Mutex
	writer  io.Writer
	onWrite func(string)
}

func (w *lockedWriter) Write(value []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	written, err := w.writer.Write(value)
	if w.onWrite != nil && written > 0 {
		w.onWrite(string(value[:written]))
	}
	return written, err
}
