// Package ldap implements strategies for authenticating using the LDAP protocol.
package ldap

import (
	"fmt"

	"gopkg.in/ldap.v2"

	"github.com/ericchiang/poke/connector"
	"github.com/ericchiang/poke/storage"
)

var knownFields = map[string]bool{
	"host":        true,
	"bind_dn":     true,
	"username":    true,
	"password":    true,
	"group_query": true,
}

func init() {
	connector.Register("ldap", new(factory))
}

type factory struct{}

func (d *factory) New(config map[string]string) (connector.Connector, error) {
	for field := range config {
		if !knownFields[field] {
			return nil, fmt.Errorf("ldap: unrecognized field %q", field)
		}
	}
	return &ldapConnector{
		host:       config["host"],
		bindDN:     config["bind_dn"],
		username:   config["username"],
		password:   config["password"],
		groupQuery: config["group_query"],
	}, nil
}

type ldapConnector struct {
	host   string
	bindDN string

	username string
	password string

	groupQuery string

	// TODO(ericchiang): TLS Config
}

func (c *ldapConnector) do(f func(c *ldap.Conn) error) error {
	// TODO(ericchiang): Connection pooling.
	conn, err := ldap.Dial("tcp", c.host)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	return f(conn)
}

func (c *ldapConnector) Login(username, password string) (storage.Identity, error) {
	err := c.do(func(conn *ldap.Conn) error {
		return conn.Bind(fmt.Sprintf("uid=%s,%s", username, c.bindDN), password)
	})
	if err != nil {
		return storage.Identity{}, err
	}

	return storage.Identity{Username: username}, nil
}

func (c *ldapConnector) Close() error {
	return nil
}
