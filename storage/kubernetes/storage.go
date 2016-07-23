package kubernetes

import "github.com/ericchiang/poke/storage"

// Config values for the Kubernetes storage type.
type Config struct {
	UniqueID  string
	Namespace string

	OutsideCluster bool
}

// New returns a storage using Kubernetes third party resource.
func (c *Config) New() (storage.Storage, error) {
	return &client{}, nil
}

func (cli *client) Close() error {
	return nil
}

func (cli *client) CreateAuthRequest(a storage.AuthRequest) error {
	return nil
}

func (cli *client) CreateClient(c storage.Client) error {
	return nil
}

func (cli *client) CreateAuthCode(c storage.AuthCode) error {
	return nil
}

func (cli *client) CreateRefresh(r storage.Refresh) error {
	return nil
}

func (cli *client) GetAuthRequest(id string) (storage.AuthRequest, error) {
	return storage.AuthRequest{}, nil
}

func (cli *client) GetAuthCode(id string) (storage.AuthCode, error) {
	return storage.AuthCode{}, nil
}

func (cli *client) GetClient(id string) (storage.Client, error) {
	return storage.Client{}, nil
}

func (cli *client) GetKeys() (storage.Keys, error) {
	return storage.Keys{}, nil
}

func (cli *client) GetRefresh(id string) (storage.Refresh, error) {
	return storage.Refresh{}, nil
}

func (cli *client) ListClients() ([]storage.Client, error) {
	return nil, nil
}

func (cli *client) ListRefreshTokens() ([]storage.Refresh, error) {
	return nil, nil
}

func (cli *client) DeleteAuthRequest(id string) error {
	return nil
}

func (cli *client) DeleteAuthCode(code string) error {
	return nil
}

func (cli *client) DeleteClient(id string) error {
	return nil
}

func (cli *client) DeleteRefresh(id string) error {
	return nil
}

func (cli *client) UpdateClient(id string, updater func(old storage.Client) (storage.Client, error)) error {
	return nil
}

func (cli *client) UpdateKeys(updater func(old storage.Keys) (storage.Keys, error)) error {
	return nil
}

func (cli *client) UpdateAuthRequest(id string, updater func(a storage.AuthRequest) (storage.AuthRequest, error)) error {
	return nil
}
