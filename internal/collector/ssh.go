package collector

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSHClient struct {
	client *ssh.Client
}

func DialSSH(host, user string, port int, identityFile string) (*SSHClient, error) {
	if port == 0 {
		port = 22
	}
	auth, err := sshAuth(identityFile)
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // trusted LAN / Tailscale path
		Timeout:         12 * time.Second,
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, err
	}
	return &SSHClient{client: client}, nil
}

func sshAuth(identityFile string) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	paths := []string{}
	if identityFile != "" {
		paths = append(paths, identityFile)
	}
	home, _ := os.UserHomeDir()
	paths = append(paths,
		home+"/.ssh/id_ed25519",
		home+"/.ssh/id_rsa",
		"/config/ssh/id_ed25519",
		"/config/ssh/id_rsa",
		"/root/.ssh/id_ed25519",
		"/root/.ssh/id_rsa",
	)
	seen := map[string]bool{}
	for _, p := range paths {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		key, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			continue
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("no SSH private key found")
	}
	return methods, nil
}

func (c *SSHClient) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

func (c *SSHClient) Run(cmd string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	out, err := session.CombinedOutput(cmd)
	return string(out), err
}

func (c *SSHClient) RunOK(cmd string) (string, error) {
	out, err := c.Run(cmd)
	if err != nil {
		return strings.TrimSpace(out), err
	}
	return strings.TrimSpace(out), nil
}
