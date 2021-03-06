/*
This is an example application to demonstrate verifying an ID Token with a nonce.
*/
package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/ericchiang/oidc"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

var (
	clientID     = os.Getenv("GOOGLE_OAUTH2_CLIENT_ID")
	clientSecret = os.Getenv("GOOGLE_OAUTH2_CLIENT_SECRET")
)

const appNonce = "a super secret nonce"

// Create a nonce source.
type nonceSource struct{}

func (n nonceSource) ClaimNonce(nonce string) error {
	if nonce != appNonce {
		return errors.New("unregonized nonce")
	}
	return nil
}

func main() {
	ctx := context.Background()

	provider, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		log.Fatal(err)
	}

	// Use the nonce source to create a custom ID Token verifier.
	nonceEnabledVerifier := provider.NewVerifier(ctx, oidc.VerifyNonce(nonceSource{}))

	config := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  "http://127.0.0.1:5556/auth/google/callback",
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	state := "foobar" // Don't do this in production.

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, config.AuthCodeURL(state, oidc.Nonce(appNonce)), http.StatusFound)
	})

	http.HandleFunc("/auth/google/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "state did not match", http.StatusBadRequest)
			return
		}

		oauth2Token, err := config.Exchange(ctx, r.URL.Query().Get("code"))
		if err != nil {
			http.Error(w, "Failed to exchange token: "+err.Error(), http.StatusInternalServerError)
			return
		}
		rawIDToken, ok := oauth2Token.Extra("id_token").(string)
		if !ok {
			http.Error(w, "No id_token field in oauth2 token.", http.StatusInternalServerError)
			return
		}
		// Verify the ID Token signature and nonce.
		idTokenPayload, err := nonceEnabledVerifier.Verify(rawIDToken)
		if err != nil {
			http.Error(w, "Failed to verify ID Token: "+err.Error(), http.StatusInternalServerError)
			return
		}

		rawMessage := json.RawMessage(idTokenPayload)
		resp := struct {
			OAuth2Token *oauth2.Token
			IDToken     *json.RawMessage // ID Token payload is just JSON.
		}{oauth2Token, &rawMessage}
		data, err := json.MarshalIndent(resp, "", "    ")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(data)
	})

	log.Printf("listening on http://%s/", "127.0.0.1:5556")
	log.Fatal(http.ListenAndServe("127.0.0.1:5556", nil))
}
