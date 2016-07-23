package kubernetes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"

	"github.com/ericchiang/poke/storage/kubernetes/types/client/v1"
)

type client struct {
	client    *http.Client
	host      string
	apiGroup  string
	namespace string
}

func kubeconfigPath() (string, error) {
	if p := os.Getenv("KUBECONFIG"); p != "" {
		return p, nil
	}
	p, err := homedir.Dir()
	if err != nil {
		return "", fmt.Errorf("finding homedir: %v", err)
	}
	return filepath.Join(p, ".kube", "config"), nil
}

func loadKubeconfig() (*v1.Config, error) {
	path, err := kubeconfigPath()
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open %s: %v", path, err)
	}
	defer f.Close()
	var c v1.Config
	return &c, json.NewDecoder(f).Decode(&c)
}

func newClient() (*client, error) {
	return nil, nil
}

func newInClusterClient() (*client, error) {
	return nil, nil
}
