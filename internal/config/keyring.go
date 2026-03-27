package config

import (
	"fmt"
	"os"
	"path/filepath"

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
		if err == keyring.ErrKeyNotFound {
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
	if err == keyring.ErrKeyNotFound {
		return nil
	}
	return err
}

// FileKeyring stores credentials in a plain file (fallback).
type FileKeyring struct {
	dir string
}

// NewFileKeyring creates a file-based credential store.
func NewFileKeyring(dir string) *FileKeyring {
	return &FileKeyring{dir: dir}
}

func (f *FileKeyring) Get() (string, error) {
	data, err := os.ReadFile(filepath.Join(f.dir, credsFile))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading credentials file: %w", err)
	}
	return string(data), nil
}

func (f *FileKeyring) Set(apiKey string) error {
	if err := os.MkdirAll(f.dir, 0700); err != nil {
		return fmt.Errorf("creating credentials directory: %w", err)
	}
	return os.WriteFile(filepath.Join(f.dir, credsFile), []byte(apiKey), 0600)
}

func (f *FileKeyring) Remove() error {
	err := os.Remove(filepath.Join(f.dir, credsFile))
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
