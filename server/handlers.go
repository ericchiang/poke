package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	jose "gopkg.in/square/go-jose.v2"

	"github.com/ericchiang/poke/connector"
	"github.com/ericchiang/poke/storage"
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
	state := authReq.ID

	if len(s.connectors) == 1 {
		for id := range s.connectors {
			http.Redirect(w, r, s.absPath("/auth", id)+"?state="+state, http.StatusFound)
			return
		}
	}

	connectorInfos := make([]connectorInfo, len(s.connectors))
	i := 0
	for id := range s.connectors {
		connectorInfos[i] = connectorInfo{
			DisplayName: id,
			URL:         s.absPath("/auth", id) + "?state=" + state,
		}
		i++
	}

	renderLoginOptions(w, connectorInfos, state)
}

func (s *Server) handleConnectorLogin(w http.ResponseWriter, r *http.Request) {
	connID := mux.Vars(r)["connector"]
	conn, ok := s.connectors[connID]
	if !ok {
		s.notFound(w, r)
		return
	}

	// TODO(ericchiang): cache user identity.

	state := r.FormValue("state")
	switch r.Method {
	case "GET":
		switch conn := conn.Connector.(type) {
		case connector.CallbackConnector:
			callbackURL, err := conn.LoginURL(s.absURL("/callback", connID), state)
			if err != nil {
				log.Printf("Connector %q returned error when creating callback: %v", connID, err)
				s.renderError(w, http.StatusInternalServerError, errServerError, "")
				return
			}
			http.Redirect(w, r, callbackURL, http.StatusFound)
		case connector.PasswordConnector:
			renderPasswordTmpl(w, state, r.URL.String(), "")
		default:
			s.notFound(w, r)
		}
	case "POST":
		passwordConnector, ok := conn.Connector.(connector.PasswordConnector)
		if !ok {
			s.notFound(w, r)
			return
		}

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

		groups, ok, err := s.groups(identity, state, conn.Connector)
		if err != nil {
			s.renderError(w, http.StatusInternalServerError, errServerError, "")
			return
		}
		if ok {
			identity.Groups = groups
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
	groups, ok, err := s.groups(identity, state, conn.Connector)
	if err != nil {
		s.renderError(w, http.StatusInternalServerError, errServerError, "")
		return
	}
	if ok {
		identity.Groups = groups
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

func (s *Server) groups(identity storage.Identity, authReqID string, conn connector.Connector) ([]string, bool, error) {
	groupsConn, ok := conn.(connector.GroupsConnector)
	if !ok {
		return nil, false, nil
	}
	authReq, err := s.storage.GetAuthRequest(authReqID)
	if err != nil {
		log.Printf("get auth request: %v", err)
		return nil, false, err
	}
	reqGroups := func() bool {
		for _, scope := range authReq.Scopes {
			if scope == scopeGroups {
				return true
			}
		}
		return false
	}()
	if !reqGroups {
		return nil, false, nil
	}
	groups, err := groupsConn.Groups(identity)
	return groups, true, err
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
		if r.FormValue("approval") != "approve" {
			s.renderError(w, http.StatusInternalServerError, "approval rejected", "")
			return
		}
		s.sendCodeResponse(w, r, authReq, *authReq.Identity)
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

	if authReq.RedirectURI == "urn:ietf:wg:oauth:2.0:oob" {
		// TODO(ericchiang): Add a proper template.
		fmt.Fprintf(w, "Code: %s", code.ID)
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
	clientID, clientSecret, ok := r.BasicAuth()
	if ok {
		var err error
		if clientID, err = url.QueryUnescape(clientID); err != nil {
			tokenErr(w, errInvalidRequest, "client_id improperly encoded", http.StatusBadRequest)
			return
		}
		if clientSecret, err = url.QueryUnescape(clientSecret); err != nil {
			tokenErr(w, errInvalidRequest, "client_secret improperly encoded", http.StatusBadRequest)
			return
		}
	} else {
		clientID = r.PostFormValue("client_id")
		clientSecret = r.PostFormValue("client_secret")
	}

	client, err := s.storage.GetClient(clientID)
	if err != nil {
		if err != storage.ErrNotFound {
			log.Printf("failed to get client: %v", err)
			tokenErr(w, errServerError, "", http.StatusInternalServerError)
		} else {
			tokenErr(w, errInvalidClient, "Invalid client credentials.", http.StatusUnauthorized)
		}
		return
	}
	if client.Secret != clientSecret {
		tokenErr(w, errInvalidClient, "Invalid client credentials.", http.StatusUnauthorized)
		return
	}

	grantType := r.PostFormValue("grant_type")
	switch grantType {
	case "authorization_code":
		s.handleAuthCode(w, r, client)
	case "refresh_token":
		s.handleRefreshToken(w, r, client)
	default:
		tokenErr(w, errInvalidGrant, "", http.StatusBadRequest)
	}
}

// handle an access token request https://tools.ietf.org/html/rfc6749#section-4.1.3
func (s *Server) handleAuthCode(w http.ResponseWriter, r *http.Request, client storage.Client) {
	code := r.PostFormValue("code")
	redirectURI := r.PostFormValue("redirect_uri")

	authCode, err := s.storage.GetAuthCode(code)
	if err != nil || s.now().After(authCode.Expiry) || authCode.ClientID != client.ID {
		if err != storage.ErrNotFound {
			log.Printf("failed to get auth code: %v", err)
			tokenErr(w, errServerError, "", http.StatusInternalServerError)
		} else {
			tokenErr(w, errInvalidRequest, "Invalid or expired code parameter.", http.StatusBadRequest)
		}
		return
	}

	if authCode.RedirectURI != redirectURI {
		tokenErr(w, errInvalidRequest, "redirect_uri did not match URI from initial request.", http.StatusBadRequest)
		return
	}

	idToken, expiry, err := s.newIDToken(client.ID, authCode.Identity, authCode.Scopes, authCode.Nonce)
	if err != nil {
		log.Printf("failed to create ID token: %v", err)
		tokenErr(w, errServerError, "", http.StatusInternalServerError)
		return
	}

	if err := s.storage.DeleteAuthCode(code); err != nil {
		log.Printf("failed to delete auth code: %v", err)
		tokenErr(w, errServerError, "", http.StatusInternalServerError)
		return
	}

	reqRefresh := func() bool {
		for _, scope := range authCode.Scopes {
			if scope == scopeOfflineAccess {
				return true
			}
		}
		return false
	}()
	var refreshToken string
	if reqRefresh {
		refresh := storage.Refresh{
			RefreshToken: storage.NewNonce(),
			ClientID:     authCode.ClientID,
			ConnectorID:  authCode.ConnectorID,
			Scopes:       authCode.Scopes,
			Identity:     authCode.Identity,
			Nonce:        authCode.Nonce,
		}
		if err := s.storage.CreateRefresh(refresh); err != nil {
			log.Printf("failed to create refresh token: %v", err)
			tokenErr(w, errServerError, "", http.StatusInternalServerError)
			return
		}
		refreshToken = refresh.RefreshToken
	}
	s.writeAccessToken(w, idToken, refreshToken, expiry)
}

// handle a refresh token request https://tools.ietf.org/html/rfc6749#section-6
func (s *Server) handleRefreshToken(w http.ResponseWriter, r *http.Request, client storage.Client) {
	code := r.PostFormValue("refresh_token")
	scope := r.PostFormValue("scope")
	if code == "" {
		tokenErr(w, errInvalidRequest, "No refresh token in request.", http.StatusBadRequest)
		return
	}

	refresh, err := s.storage.GetRefresh(code)
	if err != nil || refresh.ClientID != client.ID {
		if err != storage.ErrNotFound {
			log.Printf("failed to get auth code: %v", err)
			tokenErr(w, errServerError, "", http.StatusInternalServerError)
		} else {
			tokenErr(w, errInvalidRequest, "Refresh token is invalid or has already been claimed by another client.", http.StatusBadRequest)
		}
		return
	}

	scopes := refresh.Scopes
	if scope != "" {
		requestedScopes := strings.Split(scope, " ")
		contains := func() bool {
		Loop:
			for _, s := range requestedScopes {
				for _, scope := range refresh.Scopes {
					if s == scope {
						continue Loop
					}
				}
				return false
			}
			return true
		}()
		if !contains {
			tokenErr(w, errInvalidRequest, "Requested scopes did not contain authorized scopes.", http.StatusBadRequest)
			return
		}
		scopes = requestedScopes
	}

	// TODO(ericchiang): re-auth with backends

	idToken, expiry, err := s.newIDToken(client.ID, refresh.Identity, scopes, refresh.Nonce)
	if err != nil {
		log.Printf("failed to create ID token: %v", err)
		tokenErr(w, errServerError, "", http.StatusInternalServerError)
		return
	}

	if err := s.storage.DeleteRefresh(code); err != nil {
		log.Printf("failed to delete auth code: %v", err)
		tokenErr(w, errServerError, "", http.StatusInternalServerError)
		return
	}
	refresh.RefreshToken = storage.NewNonce()
	if err := s.storage.CreateRefresh(refresh); err != nil {
		log.Printf("failed to create refresh token: %v", err)
		tokenErr(w, errServerError, "", http.StatusInternalServerError)
		return
	}
	s.writeAccessToken(w, idToken, refresh.RefreshToken, expiry)
}

func (s *Server) writeAccessToken(w http.ResponseWriter, idToken, refreshToken string, expiry time.Time) {
	// TODO(ericchiang): figure out an access token story and support the user info
	// endpoint. For now use a random value so no one depends on the access_token
	// holding a specific structure.
	resp := struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token,omitempty"`
		IDToken      string `json:"id_token"`
	}{
		storage.NewNonce(),
		"bearer",
		int(expiry.Sub(s.now())),
		refreshToken,
		idToken,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("failed to marshal access token response: %v", err)
		tokenErr(w, errServerError, "", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

func (s *Server) renderError(w http.ResponseWriter, status int, err, description string) {
	http.Error(w, fmt.Sprintf("%s: %s", err, description), status)
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}
