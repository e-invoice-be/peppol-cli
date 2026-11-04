package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/99designs/keyring"
)

const (
	keyringService = "peppol-cli"
	keyringKey     = "api-key"
	credsFile      = "credentials"
)

// KeyringBackend abstracts credential storage for testability.
type KeyringBackend interface {
	Get() (string, error)
	Set(apiKey string) error
	Remove() error
}

// OSKeyring uses the OS keychain via 99designs/keyring with file fallback.
type OSKeyring struct {
	ring keyring.Keyring
}

// NewOSKeyring creates a keyring backed by OS keychain with file fallback.
func NewOSKeyring() (*OSKeyring, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}

	ring, err := keyring.Open(keyring.Config{
		ServiceName: keyringService,
		// File-based fallback when no system keychain is available.
		FileDir:          dir,
		FilePasswordFunc: func(_ string) (string, error) { return "", nil },
		// Prefer system keychains, fall back to file.
		AllowedBackends: []keyring.BackendType{
			keyring.KeychainBackend,
			keyring.SecretServiceBackend,
			keyring.WinCredBackend,
			keyring.FileBackend,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("opening keyring: %w", err)
	}

	return &OSKeyring{ring: ring}, nil
}

func (k *OSKeyring) Get() (string, error) {
	item, err := k.ring.Get(keyringKey)
	if err != nil {
		if errors.Is(err, keyring.ErrKeyNotFound) {
			return "", nil
		}
		return "", fmt.Errorf("reading from keyring: %w", err)
	}
	return string(item.Data), nil
}

func (k *OSKeyring) Set(apiKey string) error {
	return k.ring.Set(keyring.Item{
		Key:  keyringKey,
		Data: []byte(apiKey),
	})
}

func (k *OSKeyring) Remove() error {
	err := k.ring.Remove(keyringKey)
	if errors.Is(err, keyring.ErrKeyNotFound) {
		return nil
	}
	return err
}

// FileKeyring stores credentials in a plain file (fallback).
type FileKeyring struct {
	dir      string
	filename string
	// err is set when the keyring was built from an unsafe workspace name;
	// every operation then fails instead of touching the filesystem.
	err error
}

// NewFileKeyring creates a file-based credential store using the default credentials file.
func NewFileKeyring(dir string) *FileKeyring {
	return &FileKeyring{dir: dir, filename: credsFile}
}

// NewFileKeyringForWorkspace creates a file-based credential store for a specific workspace.
// Uses "credentials-<workspace>" as the filename. If the resulting file would
// not sit directly inside dir (e.g. the name contains a path separator or
// ".."), every operation on the returned keyring fails.
func NewFileKeyringForWorkspace(dir, workspace string) *FileKeyring {
	filename := credsFile + "-" + workspace
	kr := &FileKeyring{dir: dir, filename: filename}
	if workspace == "" || strings.ContainsRune(workspace, 0) ||
		filepath.Dir(filepath.Join(dir, filename)) != filepath.Clean(dir) {
		kr.err = fmt.Errorf("invalid workspace name %q: credentials path would escape the config directory", workspace)
	}
	return kr
}

func (f *FileKeyring) Get() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	data, err := os.ReadFile(filepath.Join(f.dir, f.filename))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading credentials file: %w", err)
	}
	return string(data), nil
}

func (f *FileKeyring) Set(apiKey string) error {
	if f.err != nil {
		return f.err
	}
	if err := os.MkdirAll(f.dir, 0o700); err != nil {
		return fmt.Errorf("creating credentials directory: %w", err)
	}
	return os.WriteFile(filepath.Join(f.dir, f.filename), []byte(apiKey), 0o600)
}

func (f *FileKeyring) Remove() error {
	if f.err != nil {
		return f.err
	}
	err := os.Remove(filepath.Join(f.dir, f.filename))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ResolveAPIKey returns the API key from the highest-priority source.
// Priority: PEPPOL_API_KEY env var > keyring backend.
func ResolveAPIKey(backend KeyringBackend) (string, error) {
	if envKey := os.Getenv("PEPPOL_API_KEY"); envKey != "" {
		return envKey, nil
	}
	return backend.Get()
}

// MaxWorkspaceNameLen is the longest accepted workspace name. The name ends up
// in the credentials filename ("credentials-<name>"), so the limit keeps that
// filename within the common 255-byte filesystem limit.
const MaxWorkspaceNameLen = 255 - len(credsFile+"-")

// ValidateWorkspaceName reports whether name is safe to use as a workspace
// name. Allowed characters are ASCII letters, digits, '-', '_' and '.'; the
// name must not be empty, "." or "..", and must be at most
// MaxWorkspaceNameLen bytes long.
func ValidateWorkspaceName(name string) error {
	if name == "" {
		return errors.New("invalid workspace name: must not be empty")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("invalid workspace name %q", name)
	}
	if len(name) > MaxWorkspaceNameLen {
		return fmt.Errorf("invalid workspace name: longer than %d characters", MaxWorkspaceNameLen)
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
		default:
			return fmt.Errorf("invalid workspace name %q: only letters, digits, '-', '_' and '.' are allowed", name)
		}
	}
	return nil
}
