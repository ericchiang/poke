// Package connector defines interfaces for federated identity strategies.
package connector

import (
	"fmt"
	"net/http"

	"github.com/ericchiang/poke/storage"
)

var factories = make(map[string]Factory)

// Factory is an implementation which can initialize a connector.
type Factory interface {
	// New initializes a Connector. The callback URL is the URL which the the HandleCallback
	// URL will listen at.
	New(config map[string]string) (Connector, error)
}

// Register makes a factory available by the provided name. If register is
// called twice with the same name or if the factory is nil, it panics.
//
// knownFields holds the list of configuration fields known to the factory.
func Register(name string, factory Factory) {
	if factory == nil {
		panic("factory cannot be nil")
	}
	if _, ok := factories[name]; ok {
		panic("factory " + name + " is already registered")
	}
	factories[name] = factory
}

// New initializes a connector from the provided configuration.
func New(name string, config map[string]string) (Connector, error) {
	factory, ok := factories[name]
	if !ok {
		return nil, fmt.Errorf("no factory of type %s found", name)
	}
	return factory.New(config)
}

// Connector is a mechanism for federating login to a remote identity service.
//
// Implementations are expected to implement either the PasswordConnector or
// CallbackConnector interface.
type Connector interface {
	Close() error
}

// PasswordConnector is an optional interface for password based connectors.
type PasswordConnector interface {
	Login(username, password string) (identity storage.Identity, validPassword bool, err error)
}

// CallbackConnector is an optional interface for callback based connectors.
type CallbackConnector interface {
	HandleLogin(w http.ResponseWriter, r *http.Request, callbackURL, state string)
	HandleCallback(r *http.Request) (identity storage.Identity, state string, err error)
}

// GroupsConnector is an optional interface for connectors which can map a user to groups.
type GroupsConnector interface {
	Groups(identity storage.Identity) ([]string, error)
}
