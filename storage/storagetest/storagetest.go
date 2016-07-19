// +build go1.7

// Package storagetest provides conformance tests for storage implementations.
package storagetest

import (
	"reflect"
	"testing"
	"time"

	"github.com/ericchiang/poke/storage"
)

var neverExpire = time.Now().Add(time.Hour * 24 * 365 * 100)

// RunTestSuite runs a set of conformance tests against a storage.
func RunTestSuite(t *testing.T, s storage.Storage) {
	t.Run("UpdateAuthRequest", func(t *testing.T) { testUpdateAuthRequest(t, s) })
}

func testUpdateAuthRequest(t *testing.T, s storage.Storage) {
	a := storage.AuthRequest{
		ID:            storage.NewNonce(),
		ClientID:      "foobar",
		ResponseTypes: []string{"code"},
		Scopes:        []string{"openid", "email"},
		RedirectURI:   "https://localhost:80/callback",
		Expiry:        neverExpire,
	}

	identity := storage.Identity{Email: "foobar"}

	if err := s.CreateAuthRequest(a); err != nil {
		t.Fatalf("failed creating auth request: %v", err)
	}
	if err := s.UpdateAuthRequest(a.ID, func(old storage.AuthRequest) (storage.AuthRequest, error) {
		old.Identity = &identity
		old.ConnectorID = "connID"
		return old, nil
	}); err != nil {
		t.Fatalf("failed to update auth request: %v", err)
	}

	got, err := s.GetAuthRequest(a.ID)
	if err != nil {
		t.Fatalf("failed to get auth req: %v", err)
	}
	if got.Identity == nil {
		t.Fatalf("no identity in auth request")
	}
	if !reflect.DeepEqual(*got.Identity, identity) {
		t.Fatalf("update failed, wanted identity=%#v got %#v", identity, *got.Identity)
	}
}
