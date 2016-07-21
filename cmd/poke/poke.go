package main

import (
	"fmt"

	"github.com/ericchiang/poke/connector"
	"github.com/ericchiang/poke/connector/ldap"
	"github.com/ericchiang/poke/connector/mock"
)

type config struct {
	Connectors []connectorConfig `yaml:"connectors"`
}

type connectorMetadata struct {
	Type string `yaml:"type"`
	Name string `yaml:"name"`
	ID   string `yaml:"id"`
}

// connectorConfig is a magical type that can unmarshal YAML dynamically. The
// Type field determines the connector type, which is then customized for Config.
type connectorConfig struct {
	connectorMetadata
	Config interface {
		Open() (connector.Connector, error)
	}
}

func (c *connectorConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&c.connectorMetadata); err != nil {
		return err
	}

	switch c.Type {
	case "mock":
		c.Config = &mock.Config{}
	case "ldap":
		c.Config = &ldap.Config{}
	default:
		return fmt.Errorf("unknown connector type %q", c.Type)
	}
	return unmarshal(c.Config)
}

func main() {}
