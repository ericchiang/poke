package kubernetes

import (
	"time"

	"github.com/ericchiang/poke/storage/kubernetes/types/unversioned"
	"github.com/ericchiang/poke/storage/kubernetes/types/v1"

	jose "gopkg.in/square/go-jose.v2"
)

// Client is a mirrored struct from storage with JSON struct tags and
// Kubernetes type metadata.
type Client struct {
	unversioned.TypeMeta `json:",inline"`
	v1.ObjectMeta        `json:"metadata,omitempty"`

	Secret       string   `json:"secret,omitempty"`
	RedirectURIs []string `json:"redirectURIs,omitempty"`
	TrustedPeers []string `json:"trustedPeers,omitempty"`

	Public bool `json:"public"`

	Name    string `json:"name,omitempty"`
	LogoURL string `json:"logoURL,omitempty"`
}

// Identity is a mirrored struct from storage with JSON struct tags.
type Identity struct {
	UserID        string   `json:"userID"`
	Username      string   `json:"username"`
	Email         string   `json:"email"`
	EmailVerified bool     `json:"emailVerified"`
	Groups        []string `json:"groups,omitempty"`

	ConnectorData []byte `json:"connectorData,omitempty"`
}

// AuthRequest is a mirrored struct from storage with JSON struct tags and
// Kubernetes type metadata.
type AuthRequest struct {
	unversioned.TypeMeta `json:",inline"`
	v1.ObjectMeta        `json:"metadata,omitempty"`

	ClientID      string   `json:"clientID"`
	ResponseTypes []string `json:"responseTypes,omitempty"`
	Scopes        []string `json:"scopes,omitempty"`
	RedirectURI   string   `json:"redirectURI"`

	Nonce string `json:"nonce,omitempty"`
	State string `json:"state,omitempty"`

	// The client has indicated that the end user must be shown an approval prompt
	// on all requests. The server cannot cache their initial action for subsequent
	// attempts.
	ForceApprovalPrompt bool `json:"forceApprovalPrompt,omitempty"`

	// The identity of the end user. Generally nil until the user authenticates
	// with a backend.
	Identity *Identity `json:"identity,omitempty"`
	// The connector used to login the user. Set when the user authenticates.
	ConnectorID string `json:"connectorID,omitempty"`

	Expiry time.Time `json:"expiry"`
}

// AuthCode is a mirrored struct from storage with JSON struct tags and
// Kubernetes type metadata.
type AuthCode struct {
	unversioned.TypeMeta `json:",inline"`
	v1.ObjectMeta        `json:"metadata,omitempty"`

	ClientID    string   `json:"clientID"`
	Scopes      []string `json:"scopes,omitempty"`
	RedirectURI string   `json:"redirectURI"`

	Nonce string `json:"nonce,omitempty"`
	State string `json:"state,omitempty"`

	Identity    Identity `json:"identity,omitempty"`
	ConnectorID string   `json:"connectorID,omitempty"`

	Expiry time.Time `json:"expiry"`
}

// Refresh is a mirrored struct from storage with JSON struct tags and
// Kubernetes type metadata.
type Refresh struct {
	unversioned.TypeMeta `json:",inline"`
	v1.ObjectMeta        `json:"metadata,omitempty"`

	ClientID    string   `json:"clientID"`
	Scopes      []string `json:"scopes,omitempty"`
	RedirectURI string   `json:"redirectURI"`

	Nonce string `json:"nonce,omitempty"`
	State string `json:"state,omitempty"`

	Identity    Identity `json:"identity,omitempty"`
	ConnectorID string   `json:"connectorID,omitempty"`
}

// Keys is a mirrored struct from storage with JSON struct tags and Kubernetes
// type metadata.
type Keys struct {
	unversioned.TypeMeta `json:",inline"`
	v1.ObjectMeta        `json:"metadata,omitempty"`

	// Key for creating and verifying signatures. These may be nil.
	SigningKey    *jose.JSONWebKey `json:"signingKey,omitempty"`
	SigningKeyPub *jose.JSONWebKey `json:"signingKeyPub,omitempty"`
	// Old signing keys which have been rotated but can still be used to validate
	// existing signatures.
	VerificationKeys []VerificationKey `json:"verificationKeys,omitempty"`

	// Key for encryption messages.
	EncryptionKey *[32]byte `json:"encryptionKey,omitempty"`
	// Old encrpytion keys which have been rotated but can still be used to
	// decrypt existing messages.
	DecryptionKeys []DecryptionKey `json:"decryptionKeys,omitempty"`

	// The next time the signing key will rotate.
	//
	// For caching purposes, implementations MUST NOT update keys before this time.
	NextRotation time.Time `json:"nextRotation"`
}

// DecryptionKey is a rotated encryption key which can still be used to decrypt
// existing messages.
type DecryptionKey struct {
	Key    *[32]byte `json:"key"`
	Expiry time.Time `json:"expiry"`
}

// VerificationKey is a rotated signing key which can still be used to verify
// signatures.
type VerificationKey struct {
	PublicKey *jose.JSONWebKey `json:"publicKey"`
	Expiry    time.Time        `json:"expiry"`
}
