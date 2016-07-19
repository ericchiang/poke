package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	"github.com/ericchiang/poke/connector"
	"github.com/ericchiang/poke/storage"
	"github.com/gorilla/mux"

	jose "gopkg.in/square/go-jose.v2"
)

func (s *Server) handlePublicKeys(w http.ResponseWriter, r *http.Request) {
	// TODO(ericchiang): Cache this.
	keys, err := s.storage.GetKeys()
	if err != nil {
		log.Printf("failed to get keys: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if keys.SigningKeyPub == nil {
		log.Printf("No public keys found.")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	jwks := jose.JSONWebKeySet{
		Keys: make([]jose.JSONWebKey, len(keys.VerificationKeys)+1),
	}
	jwks.Keys[0] = *keys.SigningKeyPub
	for i, verificationKey := range keys.VerificationKeys {
		jwks.Keys[i+1] = *verificationKey.PublicKey
	}

	data, err := json.MarshalIndent(jwks, "", "  ")
	if err != nil {
		log.Printf("failed to marshal discovery data: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	maxAge := keys.NextRotation.Sub(s.now())
	if maxAge < (time.Minute * 2) {
		maxAge = time.Minute * 2
	}

	w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d, must-revalidate", maxAge))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

type discovery struct {
	Issuer        string   `json:"issuer"`
	Auth          string   `json:"authorization_endpoint"`
	Token         string   `json:"token_endpoint"`
	Keys          string   `json:"jwks_uri"`
	ResponseTypes []string `json:"response_types_supported"`
	Subjects      []string `json:"subject_types_supported"`
	IDTokenAlgs   []string `json:"id_token_signing_alg_values_supported"`
	Scopes        []string `json:"scopes_supported"`
	AuthMethods   []string `json:"token_endpoint_auth_methods_supported"`
	Claims        []string `json:"claims_supported"`
}

func (s *Server) handleDiscovery(w http.ResponseWriter, r *http.Request) {
	// TODO(ericchiang): Cache this
	d := discovery{
		Issuer:        s.issuerURL.String(),
		Auth:          s.absURL("/auth"),
		Token:         s.absURL("/token"),
		Keys:          s.absURL("/keys"),
		ResponseTypes: []string{"code"},
		Subjects:      []string{"public"},
		IDTokenAlgs:   []string{string(jose.RS256)},
		Scopes:        []string{"openid", "email", "profile"},
		AuthMethods:   []string{"client_secret_basic"},
		Claims: []string{
			"aud", "email", "email_verified", "exp", "family_name", "given_name",
			"iat", "iss", "locale", "name", "sub",
		},
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		log.Printf("failed to marshal discovery data: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

// handleAuthorization handles the OAuth2 auth endpoint.
func (s *Server) handleAuthorization(w http.ResponseWriter, r *http.Request) {
	if len(s.connectors) == 1 {
		for id := range s.connectors {
			http.Redirect(w, r, s.absPath("/auth", id)+"?"+r.URL.RawQuery, http.StatusFound)
			return
		}
	}

	connectorInfos := make([]connectorInfo, len(s.connectors))
	i := 0
	for id := range s.connectors {
		connectorInfos[i] = connectorInfo{
			DisplayName: id,
			URL:         s.absPath("/auth", id) + "?" + r.URL.RawQuery,
		}
		i++
	}

	renderLoginOptions(w, connectorInfos, r.URL.Query())
}

func (s *Server) handleConnectorLogin(w http.ResponseWriter, r *http.Request) {
	connID := mux.Vars(r)["connector"]
	conn, ok := s.connectors[connID]
	if !ok {
		s.notFound(w, r)
		return
	}

	// TODO(ericchiang): cache user identity.

	switch r.Method {
	case "GET":
		// TODO(ericchiang): move this to the handleLogin method.
		authReq, err := parseAuthorizationRequest(s.storage, r)
		if err != nil {
			s.renderError(w, http.StatusInternalServerError, err.Type, err.Description)
			return
		}
		if err := s.storage.CreateAuthRequest(authReq); err != nil {
			log.Printf("Failed to create authorization request: %v", err)
			s.renderError(w, http.StatusInternalServerError, errServerError, "")
			return
		}

		switch conn := conn.Connector.(type) {
		case connector.CallbackConnector:
			conn.HandleLogin(w, r, s.absURL("/callback", connID), authReq.ID)
		case connector.PasswordConnector:
			renderPasswordTmpl(w, authReq.ID, r.URL.String(), "")
		default:
			s.notFound(w, r)
		}
	case "POST":
		passwordConnector, ok := conn.Connector.(connector.PasswordConnector)
		if !ok {
			s.notFound(w, r)
			return
		}

		state := r.FormValue("state")
		username := r.FormValue("username")
		password := r.FormValue("password")

		identity, ok, err := passwordConnector.Login(username, password)
		if err != nil {
			log.Printf("Failed to login user: %v", err)
			s.renderError(w, http.StatusInternalServerError, errServerError, "")
			return
		}
		if !ok {
			renderPasswordTmpl(w, state, r.URL.String(), "Invalid credentials")
			return
		}

		s.redirectToApproval(w, r, identity, connID, state)
	default:
		s.notFound(w, r)
	}
}

func (s *Server) handleConnectorCallback(w http.ResponseWriter, r *http.Request) {
	connID := mux.Vars(r)["connector"]
	conn, ok := s.connectors[connID]
	if !ok {
		s.notFound(w, r)
		return
	}
	callbackConnector, ok := conn.Connector.(connector.CallbackConnector)
	if !ok {
		s.notFound(w, r)
		return
	}

	identity, state, err := callbackConnector.HandleCallback(r)
	if err != nil {
		log.Printf("Failed to authenticate: %v", err)
		s.renderError(w, http.StatusInternalServerError, errServerError, "")
		return
	}
	s.redirectToApproval(w, r, identity, connID, state)
}

func (s *Server) redirectToApproval(w http.ResponseWriter, r *http.Request, identity storage.Identity, connectorID, state string) {
	updater := func(a storage.AuthRequest) (storage.AuthRequest, error) {
		a.Identity = &identity
		a.ConnectorID = connectorID
		return a, nil
	}
	if err := s.storage.UpdateAuthRequest(state, updater); err != nil {
		log.Printf("Failed to updated auth request with identity: %v", err)
		s.renderError(w, http.StatusInternalServerError, errServerError, "")
		return
	}
	http.Redirect(w, r, path.Join(s.issuerURL.Path, "/approval")+"?state="+state, http.StatusSeeOther)
}

func (s *Server) handleApproval(w http.ResponseWriter, r *http.Request) {
	authReq, err := s.storage.GetAuthRequest(r.FormValue("state"))
	if err != nil {
		log.Printf("Failed to get auth request: %v", err)
		s.renderError(w, http.StatusInternalServerError, errServerError, "")
		return
	}
	if authReq.Identity == nil {
		log.Printf("Auth request does not have an identity for approval")
		s.renderError(w, http.StatusInternalServerError, errServerError, "")
		return
	}

	switch r.Method {
	case "GET":
		if s.skipApproval {
			s.sendCodeResponse(w, r, authReq, *authReq.Identity)
			return
		}
		client, err := s.storage.GetClient(authReq.ClientID)
		if err != nil {
			log.Printf("Failed to get client %q: %v", authReq.ClientID, err)
			s.renderError(w, http.StatusInternalServerError, errServerError, "")
			return
		}
		renderApprovalTmpl(w, authReq.ID, *authReq.Identity, client, authReq.Scopes)
	case "POST":

		// TODO(ericchiang): actually check approval.

		authCode := storage.AuthCode{}
		_ = authCode
	}
}

func (s *Server) sendCodeResponse(w http.ResponseWriter, r *http.Request, authReq storage.AuthRequest, identity storage.Identity) {
	if authReq.Expiry.After(s.now()) {
		s.renderError(w, http.StatusBadRequest, errInvalidRequest, "Authorization request period has expired.")
		return
	}

	if err := s.storage.DeleteAuthRequest(authReq.ID); err != nil {
		if err != storage.ErrNotFound {
			log.Printf("Failed to delete authorization request: %v", err)
			s.renderError(w, http.StatusInternalServerError, errServerError, "")
		} else {
			s.renderError(w, http.StatusBadRequest, errInvalidRequest, "Authorization request has already been completed.")
		}
		return
	}
	code := storage.AuthCode{
		ID:          storage.NewNonce(),
		ClientID:    authReq.ClientID,
		ConnectorID: authReq.ConnectorID,
		Nonce:       authReq.Nonce,
		Scopes:      authReq.Scopes,
		Identity:    *authReq.Identity,
		Expiry:      s.now().Add(time.Minute * 5),
		RedirectURI: authReq.RedirectURI,
	}
	if err := s.storage.CreateAuthCode(code); err != nil {
		log.Printf("Failed to create auth code: %v", err)
		s.renderError(w, http.StatusInternalServerError, errServerError, "")
		return
	}
	u, err := url.Parse(authReq.RedirectURI)
	if err != nil {
		s.renderError(w, http.StatusInternalServerError, errServerError, "Invalid redirect URI.")
		return
	}
	q := u.Query()
	q.Set("code", code.ID)
	q.Set("state", authReq.State)
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) renderError(w http.ResponseWriter, status int, err, description string) {
	http.Error(w, fmt.Sprintf("%s: %s", err, description), status)
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}
