package storage

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"

	jose "gopkg.in/square/go-jose.v2"
)

var (
	drivers = make(map[string]Driver)

	// stubbed out for testing
	now = time.Now
)

// ErrNotFound is the error returned by storages if a resource cannot be found.
var ErrNotFound = errors.New("not found")

// NewNonce returns a 64 bit nonce suitable for object IDs.
func NewNonce() string {
	buff := make([]byte, 8) // 64 bit random ID.
	if _, err := io.ReadFull(rand.Reader, buff); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(buff)
}

// Driver is the interface implemented by storage drivers.
type Driver interface {
	// Open returns a storage implementation. It should only validate its
	// arguments and not return an error if the underlying storage is
	// unavailable.
	Open(config map[string]string) (Storage, error)
}

// Register makes a storage driver available by the provided name. If Register
// is called twice with the same name or if driver is nil, it panics.
func Register(name string, driver Driver) {
	if driver == nil {
		panic("driver cannot be nil")
	}
	if _, ok := drivers[name]; ok {
		panic("driver " + name + " is already registered")
	}
	drivers[name] = driver
}

// Open returns a new storage object with a given key rotation strategy.
func Open(driverName string, config map[string]string) (Storage, error) {
	driver, ok := drivers[driverName]
	if !ok {
		return nil, fmt.Errorf("no driver of type %s found", driverName)
	}
	return driver.Open(config)
}

// Storage is the storage interface used by the server. Implementations, at minimum
// require compare-and-swap atomic actions.
//
// Implementations are expected to perform their own garbage collection of
// expired objects (expect keys which are handled by rotation).
type Storage interface {
	Close() error

	CreateAuthRequest(a AuthRequest) error
	CreateClient(c Client) error
	CreateAuthCode(c AuthCode) error
	CreateRefresh(r Refresh) error
	CreateNonce(n Nonce) error

	GetAuthRequest(id string) (AuthRequest, error)
	GetAuthCode(id string) (AuthCode, error)
	GetClient(id string) (Client, error)
	GetKeys() (Keys, error)
	GetRefresh(id string) (Refresh, error)

	ListClients() ([]Client, error)
	ListRefreshTokens() ([]Refresh, error)

	// Delete methods MUST be atomic.
	DeleteAuthRequest(id string) error
	DeleteAuthCode(code string) error
	DeleteClient(id string) error
	DeleteRefresh(id string) error
	DeleteNonce(nonce string) error

	// Update functions are assumed to be a performed within a single object transaction.
	UpdateClient(id string, updater func(old Client) (Client, error)) error
	UpdateKeys(updater func(old Keys) (Keys, error)) error
	UpdateAuthRequest(id string, updater func(a AuthRequest) (AuthRequest, error)) error
}

// Client is an OAuth2 client.
//
// For further reading see:
//   * Trusted peers: https://developers.google.com/identity/protocols/CrossClientAuth
//   * Public clients: https://developers.google.com/api-client-library/python/auth/installed-app
type Client struct {
	ID           string
	Secret       string
	RedirectURIs []string

	// TrustedPeers are a list of peers which can issue tokens on this client's behalf.
	// Clients inherently trust themselves.
	TrustedPeers []string

	// Public clients must use either use a redirectURL 127.0.0.1:X or "urn:ietf:wg:oauth:2.0:oob"
	Public bool

	Name    string
	LogoURL string
}

// Identity represents the ID Token claims supported by the server.
type Identity struct {
	UserID        string
	Username      string
	Email         string
	EmailVerified bool

	Groups []string

	// ConnectorData holds data used by the connector for subsequent requests after initial
	// authentication, such as access tokens for upstream provides.
	//
	// This data is never shared with end users, OAuth clients, or through the API.
	ConnectorData []byte
}

// AuthRequest represents a OAuth2 client authorization request. It holds the state
// of a single auth flow up to the point that the user authorizes the client.
type AuthRequest struct {
	ID       string
	ClientID string

	ResponseTypes []string
	Scopes        []string
	RedirectURI   string

	Nonce string
	State string

	// The client has indicated that the end user must be shown an approval prompt
	// on all requests. The server cannot cache their initial action for subsequent
	// attempts.
	ForceApprovalPrompt bool

	// The identity of the end user. Generally nil until the user authenticates
	// with a backend.
	Identity *Identity
	// The connector used to login the user. Set when the user authenticates.
	ConnectorID string

	Expiry time.Time
}

// AuthCode represents a code which can be exchanged for an OAuth2 token response.
type AuthCode struct {
	ID string

	ClientID    string
	RedirectURI string
	ConnectorID string

	Nonce string

	Scopes []string

	Identity Identity

	Expiry time.Time
}

// Refresh is an OAuth2 refresh token.
type Refresh struct {
	// The actual refresh token.
	RefreshToken string

	// Client this refresh token is valid for.
	ClientID    string
	ConnectorID string

	// Scopes present in the initial request. Refresh requests may specify a set
	// of scopes different from the initial request when refreshing a token,
	// however those scopes must be encompassed by this set.
	Scopes []string

	Nonce string

	Identity Identity

	Expiry time.Time
}

// Nonce represents a token which can be claimed exactly once.
type Nonce struct {
	Nonce  string
	Expiry time.Time
}

// DecryptionKey is a rotated encryption key which can still be used to decrypt
// existing messages.
type DecryptionKey struct {
	Key    *[32]byte
	Expiry time.Time
}

// VerificationKey is a rotated signing key which can still be used to verify
// signatures.
type VerificationKey struct {
	PublicKey *jose.JSONWebKey
	Expiry    time.Time
}

// Keys hold encryption and signing keys.
type Keys struct {
	// Key for creating and verifying signatures. These may be nil.
	SigningKey    *jose.JSONWebKey
	SigningKeyPub *jose.JSONWebKey
	// Old signing keys which have been rotated but can still be used to validate
	// existing signatures.
	VerificationKeys []VerificationKey

	// Key for encryption messages.
	EncryptionKey *[32]byte
	// Old encrpytion keys which have been rotated but can still be used to
	// decrypt existing messages.
	DecryptionKeys []DecryptionKey

	// The next time the signing key will rotate.
	//
	// For caching purposes, implementations MUST NOT update keys before this time.
	NextRotation time.Time
}
