// Package connectortest implements a mock connector which requires no user interaction.
package connectortest

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/ericchiang/poke/connector"
	"github.com/ericchiang/poke/storage"
)

// New returns a mock connector which requires no user interaction. It always returns
// the same (fake) identity.
func New() connector.Connector {
	return mockConnector{}
}

func init() {
	connector.Register("mock", new(factory))
}

type factory struct{}

func (f *factory) New(config map[string]string) (connector.Connector, error) {
	if len(config) != 0 {
		return nil, errors.New("connectortest: mock connector does not take any config")
	}
	return New(), nil
}

type mockConnector struct{}

func (m mockConnector) Close() error { return nil }

func (m mockConnector) HandleLogin(w http.ResponseWriter, r *http.Request, callbackURL, state string) {
	u, err := url.Parse(callbackURL)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse callbackURL %q: %v", callbackURL, err), http.StatusBadRequest)
		return
	}
	v := u.Query()
	v.Set("state", state)
	u.RawQuery = v.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (m mockConnector) HandleCallback(r *http.Request) (storage.Identity, string, error) {
	return storage.Identity{
		Username:      "Kilgore Trout",
		Email:         "kilgore@kilgore.trout",
		EmailVerified: true,
	}, r.URL.Query().Get("state"), nil
}

func (m mockConnector) Groups(identity storage.Identity) ([]string, error) {
	return []string{"authors"}, nil
}
