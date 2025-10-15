package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/go-containerregistry/pkg/authn"
	oras_auth "oras.land/oras-go/v2/registry/remote/auth"
)

// credentialStoreFileName is the filename used to persist dynactl registry credentials.
const credentialStoreFileName = "credentials.json"

var credentialStoreOnce sync.Once

// RegistryCredential represents the persisted credential fields for a registry.
type RegistryCredential struct {
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	IdentityToken string `json:"identity_token,omitempty"`
	AccessToken   string `json:"access_token,omitempty"`
}

type credentialStore struct {
	Credentials map[string]RegistryCredential `json:"credentials"`
}

var cachedCredentialStore *credentialStore
var credentialStoreErr error

// SaveRegistryCredential stores credentials for a registry in the dynactl credential store.
func SaveRegistryCredential(registry string, cred RegistryCredential) error {
	if registry == "" {
		return fmt.Errorf("registry cannot be empty")
	}

	store, err := loadCredentialStore()
	if err != nil {
		return fmt.Errorf("failed to load credential store: %w", err)
	}

	if store.Credentials == nil {
		store.Credentials = make(map[string]RegistryCredential)
	}

	store.Credentials[registry] = cred

	path, err := credentialStorePath()
	if err != nil {
		return fmt.Errorf("failed to resolve credential store path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to ensure credential directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open credential store for writing: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(store); err != nil {
		return fmt.Errorf("failed to write credential store: %w", err)
	}

	// Update cached store
	cachedCredentialStore = store
	return nil
}

// GetRegistryCredential retrieves a credential from the dynactl credential store.
func GetRegistryCredential(registry string) (RegistryCredential, bool, error) {
	if registry == "" {
		return RegistryCredential{}, false, fmt.Errorf("registry cannot be empty")
	}

	store, err := loadCredentialStore()
	if err != nil {
		return RegistryCredential{}, false, err
	}

	cred, ok := store.Credentials[registry]
	return cred, ok, nil
}

// resolveRegistryCredential merges credentials from docker/oras config and the dynactl store.
func resolveRegistryCredential(registry string) (oras_auth.Credential, error) {
	if registry == "" {
		return oras_auth.Credential{}, fmt.Errorf("registry cannot be empty")
	}

	cred, found, err := resolveFromDefaultKeychain(registry)
	if err != nil {
		return oras_auth.Credential{}, err
	}
	if found {
		return cred, nil
	}

	storeCred, ok, err := GetRegistryCredential(registry)
	if err != nil {
		return oras_auth.Credential{}, err
	}
	if ok {
		return convertStoredCredential(storeCred), nil
	}

	return oras_auth.EmptyCredential, nil
}

func resolveFromDefaultKeychain(registry string) (oras_auth.Credential, bool, error) {
	authenticator, err := authn.DefaultKeychain.Resolve(simpleRegistry{registry: registry})
	if err != nil {
		// The default keychain returns an error when credentials are not found, so treat
		// this as "not found" rather than a fatal error.
		return oras_auth.EmptyCredential, false, nil
	}

	if authenticator == authn.Anonymous {
		return oras_auth.EmptyCredential, false, nil
	}

	cfg, err := authenticator.Authorization()
	if err != nil {
		LogDebug("Failed to read credentials for %s from default keychain: %v", registry, err)
		return oras_auth.EmptyCredential, false, nil
	}
	if cfg == nil {
		return oras_auth.EmptyCredential, false, nil
	}

	cred := oras_auth.Credential{
		Username:     cfg.Username,
		Password:     cfg.Password,
		RefreshToken: cfg.IdentityToken,
		AccessToken:  cfg.RegistryToken,
	}

	if cred.Username == "" && cred.Password == "" && cred.RefreshToken == "" && cred.AccessToken == "" {
		return oras_auth.EmptyCredential, false, nil
	}

	return cred, true, nil
}

func convertStoredCredential(storeCred RegistryCredential) oras_auth.Credential {
	return oras_auth.Credential{
		Username:     storeCred.Username,
		Password:     storeCred.Password,
		RefreshToken: storeCred.IdentityToken,
		AccessToken:  storeCred.AccessToken,
	}
}

// NewDynactlKeychain returns an authn.Keychain that prefers Docker credentials but
// falls back to dynactl's credential store.
func NewDynactlKeychain() authn.Keychain {
	return dynactlKeychain{}
}

type dynactlKeychain struct{}

func (k dynactlKeychain) Resolve(target authn.Resource) (authn.Authenticator, error) {
	if auth, err := authn.DefaultKeychain.Resolve(target); err == nil && auth != nil && auth != authn.Anonymous {
		return auth, nil
	}

	registry := target.RegistryStr()
	storeCred, ok, err := GetRegistryCredential(registry)
	if err != nil {
		LogDebug("Failed to resolve stored credentials for %s: %v", registry, err)
		return authn.Anonymous, nil
	}
	if !ok {
		return authn.Anonymous, nil
	}

	cfg := authn.AuthConfig{
		Username:      storeCred.Username,
		Password:      storeCred.Password,
		IdentityToken: storeCred.IdentityToken,
		RegistryToken: storeCred.AccessToken,
	}
	return authn.FromConfig(cfg), nil
}

func loadCredentialStore() (*credentialStore, error) {
	credentialStoreOnce.Do(func() {
		path, err := credentialStorePath()
		if err != nil {
			credentialStoreErr = err
			return
		}

		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				cachedCredentialStore = &credentialStore{Credentials: make(map[string]RegistryCredential)}
				return
			}
			credentialStoreErr = fmt.Errorf("failed to read credential store: %w", err)
			return
		}

		var store credentialStore
		if err := json.Unmarshal(data, &store); err != nil {
			credentialStoreErr = fmt.Errorf("failed to parse credential store: %w", err)
			return
		}
		if store.Credentials == nil {
			store.Credentials = make(map[string]RegistryCredential)
		}
		cachedCredentialStore = &store
	})

	if credentialStoreErr != nil {
		return nil, credentialStoreErr
	}
	return cachedCredentialStore, nil
}

func credentialStorePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine user home directory: %w", err)
	}
	return filepath.Join(homeDir, ".dynactl", credentialStoreFileName), nil
}

// simpleRegistry wraps a registry string to satisfy the authn.Resource interface.
type simpleRegistry struct {
	registry string
}

func (r simpleRegistry) String() string      { return r.registry }
func (r simpleRegistry) RegistryStr() string { return r.registry }
