package main

import (
	"testing"

	"github.com/ericchiang/poke/connector/mock"

	yaml "gopkg.in/yaml.v2"
)

func TestConnectorConfig(t *testing.T) {
	data := `type: mock
name: mock1
id: mock2`
	var config connectorConfig
	if err := yaml.Unmarshal([]byte(data), &config); err != nil {
		t.Fatalf("failed to unmarshal yaml: %v", err)
	}
	if _, ok := config.Config.(*mock.Config); !ok {
		t.Fatalf("expected a mock config, got %T", config.Config)
	}
}
