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
	t.Run("CreateRefresh", func(t *testing.T) { testCreateRefresh(t, s) })
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

func testCreateRefresh(t *testing.T, s storage.Storage) {
	id := storage.NewNonce()
	refresh := storage.Refresh{
		RefreshToken: id,
		ClientID:     "client_id",
		ConnectorID:  "client_secret",
		Scopes:       []string{"openid", "email", "profile"},
	}
	if err := s.CreateRefresh(refresh); err != nil {
		t.Fatalf("create refresh token: %v", err)
	}
	gotRefresh, err := s.GetRefresh(id)
	if err != nil {
		t.Fatalf("get refresh: %v", err)
	}
	if !reflect.DeepEqual(gotRefresh, refresh) {
		t.Errorf("refresh returned did not match expected")
	}

	if err := s.DeleteRefresh(id); err != nil {
		t.Fatalf("failed to delete refresh request: %v", err)
	}

	if _, err := s.GetRefresh(id); err != storage.ErrNotFound {
		t.Errorf("after deleting refresh expected storage.ErrNotFound, got %v", err)
	}

}
