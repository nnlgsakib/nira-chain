package node

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var hexKeyRe = regexp.MustCompile(`^(0x)?[0-9a-fA-F]{64}$`)

// ImportPrivateKey restores the operator key from a hex private key (the
// Ethereum format: 64 hex characters, with or without 0x). The same key then
// signs on both the Cosmos and the EVM side.
func (m *Manager) ImportPrivateKey(key, pass string) (string, error) {
	if len(pass) < 8 {
		return "", errors.New("the password must have at least 8 characters")
	}
	key = strings.TrimSpace(key)
	if !hexKeyRe.MatchString(key) {
		return "", errors.New("a private key is 64 hex characters, with or without 0x")
	}
	key = strings.TrimPrefix(strings.TrimPrefix(key, "0x"), "0X")
	// `unsafe-import-eth-key` first reads a passphrase that encrypts the key only
	// in transit to the keyring (a random one is fine) through its own buffered
	// reader, then the keyring reads its password twice. Without a terminal no
	// prompts are printed, so the first line is sent alone and the rest after the
	// command has had time to read it; a too-early write is retried with a longer pause.
	armor := make([]byte, 16)
	_, _ = rand.Read(armor)
	var err error
	for _, pause := range []time.Duration{1500 * time.Millisecond, 4 * time.Second} {
		_, err = m.runStaged(context.Background(), pause,
			[]string{hex.EncodeToString(armor), pass + "\n" + pass},
			"keys", "unsafe-import-eth-key", KeyName, key, "--keyring-backend", "file", "--home", m.Home())
		if err == nil {
			break
		}
	}
	if err != nil {
		return "", errors.New("import failed: " + strings.ReplaceAll(err.Error(), key, "<key>"))
	}
	out, err := m.run(context.Background(), pass+"\n", "keys", "show", KeyName, "-a", "--keyring-backend", "file", "--home", m.Home())
	if err != nil {
		return "", err
	}
	addr := strings.TrimSpace(lastLines(out, 1))
	return addr, m.keyCreated(addr, pass)
}

// runStaged runs nirad, writing parts[0], then each later part after `pause`.
func (m *Manager) runStaged(ctx context.Context, pause time.Duration, parts []string, args ...string) (string, error) {
	if m.Nirad == "" {
		return "", errors.New("nirad not found")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.Nirad, args...)
	hideWindow(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		return "", err
	}
	go func() {
		defer stdin.Close()
		for i, p := range parts {
			if i > 0 {
				time.Sleep(pause)
			}
			if _, err := io.WriteString(stdin, p+"\n"); err != nil {
				return
			}
		}
	}()
	if err := cmd.Wait(); err != nil {
		return out.String(), fmt.Errorf("%v: %s", err, lastLines(out.String(), 2))
	}
	return out.String(), nil
}
