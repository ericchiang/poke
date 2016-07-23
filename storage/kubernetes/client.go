package kubernetes

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gtank/cryptopasta"
	homedir "github.com/mitchellh/go-homedir"
	yaml "gopkg.in/yaml.v2"

	"github.com/ericchiang/poke/storage"
	"github.com/ericchiang/poke/storage/kubernetes/types/client/v1"
)

type client struct {
	client    *http.Client
	baseURL   string
	namespace string
	apiGroup  string
	version   string
}

func (c *client) urlFor(apiGroup, version, namespace, resource, name string) string {
	basePath := "apis/"
	if apiGroup == "" {
		basePath = "api/"
	}
	var p string
	if namespace != "" {
		p = path.Join(basePath, apiGroup, version, "namespaces", namespace, resource, name)
	} else {
		p = path.Join(basePath, apiGroup, version, resource, name)
	}
	if strings.HasSuffix(c.baseURL, "/") {
		return c.baseURL + p
	}
	return c.baseURL + "/" + p
}

type httpErr struct {
	status string
	body   []byte
}

func (e *httpErr) Error() string {
	return fmt.Sprintf("%s: response from server \"%s\"", e.status, e.body)
}

func checkHTTPErr(r *http.Response, validStatusCodes ...int) error {
	for _, status := range validStatusCodes {
		if r.StatusCode == status {
			return nil
		}
	}
	if r.StatusCode == http.StatusNotFound {
		return storage.ErrNotFound
	}
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 2<<15)) // 64 KiB
	if err != nil {
		return fmt.Errorf("read response body: %v", err)
	}
	return &httpErr{r.Status, body}
}

// Close the response body. The initial request is drained so the connection can
// be reused.
func closeResp(r *http.Response) {
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()
}

func (c *client) get(apiGroup, version, namespace, resource, name string, v interface{}) error {
	url := c.urlFor(apiGroup, version, namespace, resource, name)
	log.Println(url)
	resp, err := c.client.Get(url)
	if err != nil {
		return err
	}
	defer closeResp(resp)
	if err := checkHTTPErr(resp, http.StatusOK); err != nil {
		return err
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func (c *client) list(apiGroup, version, namespace, resource string, v interface{}) error {
	return c.get(apiGroup, version, namespace, resource, "", v)
}

func (c *client) post(apiGroup, version, namespace, resource, name string, v interface{}) error {
	body, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal object: %v", err)
	}

	url := c.urlFor(apiGroup, version, namespace, resource, name)
	resp, err := c.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer closeResp(resp)
	return checkHTTPErr(resp, http.StatusOK)
}

func (c *client) delete(apiGroup, version, namespace, resource, name string) error {
	url := c.urlFor(apiGroup, version, namespace, resource, name)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("create delete request: %v", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("delete request: %v", err)
	}
	defer closeResp(resp)
	return checkHTTPErr(resp, http.StatusOK)
}

func (c *client) patch(apiGroup, version, namespace, resource, name string, v interface{}) error {
	body, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal object: %v", err)
	}

	url := c.urlFor(apiGroup, version, namespace, resource, name)
	req, err := http.NewRequest("PATCH", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create patch request: %v", err)
	}

	// This Content-Type tells Kubernetes to do an atomic update using the
	// resourceVersion tag in the object metadata.
	req.Header.Set("Content-Type", "application/json-patch+json")
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("patch request: %v", err)
	}
	defer closeResp(resp)

	return checkHTTPErr(resp, http.StatusOK)
}

func loadKubeConfig() (*v1.Config, error) {
	kubeConfigPath := os.Getenv("KUBECONFIG")
	if kubeConfigPath == "" {
		p, err := homedir.Dir()
		if err != nil {
			return nil, fmt.Errorf("finding homedir: %v", err)
		}
		kubeConfigPath = filepath.Join(p, ".kube", "config")
	}
	data, err := ioutil.ReadFile(kubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %v", kubeConfigPath, err)
	}
	var c v1.Config
	return &c, yaml.Unmarshal(data, &c)
}

func newClient(config *v1.Config) (*client, error) {
	cluster, user, namespace, err := currentContext(config)
	if err != nil {
		return nil, err
	}
	if namespace == "" {
		namespace = "default"
	}

	tlsConfig := cryptopasta.DefaultTLSConfig()
	data := func(b []byte, file string) ([]byte, error) {
		if b != nil {
			return b, nil
		}
		if file == "" {
			return nil, nil
		}
		return ioutil.ReadFile(file)
	}

	if caData, err := data(cluster.CertificateAuthorityData, cluster.CertificateAuthority); err != nil {
		return nil, err
	} else if caData != nil {
		tlsConfig.RootCAs = x509.NewCertPool()
		if !tlsConfig.RootCAs.AppendCertsFromPEM(caData) {
			return nil, fmt.Errorf("no certificate data found: %v", err)
		}
	}

	clientCert, err := data(user.ClientCertificateData, user.ClientCertificate)
	if err != nil {
		return nil, err
	}
	clientKey, err := data(user.ClientKeyData, user.ClientKey)
	if err != nil {
		return nil, err
	}
	if clientCert != nil && clientKey != nil {
		cert, err := tls.X509KeyPair(clientCert, clientKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load client cert: %v", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	var t http.RoundTripper = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSClientConfig:       tlsConfig,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if user.Token != "" {
		t = transport{
			updateReq: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+user.Token)
			},
			base: t,
		}
	}

	if user.Username != "" && user.Password != "" {
		t = transport{
			updateReq: func(r *http.Request) {
				r.SetBasicAuth(user.Username, user.Password)
			},
			base: t,
		}
	}

	// TODO(ericchiang): make API Group and version configurable.
	return &client{&http.Client{Transport: t}, cluster.Server, namespace, "oidc.coreos.com", "v1"}, nil
}

type transport struct {
	updateReq func(r *http.Request)
	base      http.RoundTripper
}

func (t transport) RoundTrip(r *http.Request) (*http.Response, error) {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	t.updateReq(r2)
	return t.base.RoundTrip(r2)
}

func currentContext(config *v1.Config) (cluster v1.Cluster, user v1.AuthInfo, ns string, err error) {
	if config.CurrentContext == "" {
		return cluster, user, "", errors.New("kubeconfig has no current context")
	}
	context, ok := func() (v1.Context, bool) {
		for _, namedContext := range config.Contexts {
			if namedContext.Name == config.CurrentContext {
				return namedContext.Context, true
			}
		}
		return v1.Context{}, false
	}()
	if !ok {
		return cluster, user, "", fmt.Errorf("no context named %q found", config.CurrentContext)
	}

	cluster, ok = func() (v1.Cluster, bool) {
		for _, namedCluster := range config.Clusters {
			if namedCluster.Name == context.Cluster {
				return namedCluster.Cluster, true
			}
		}
		return v1.Cluster{}, false
	}()
	if !ok {
		return cluster, user, "", fmt.Errorf("no cluster named %q found", context.Cluster)
	}

	user, ok = func() (v1.AuthInfo, bool) {
		for _, namedAuthInfo := range config.AuthInfos {
			if namedAuthInfo.Name == context.AuthInfo {
				return namedAuthInfo.AuthInfo, true
			}
		}
		return v1.AuthInfo{}, false
	}()
	if !ok {
		return cluster, user, "", fmt.Errorf("no user named %q found", context.AuthInfo)
	}
	return cluster, user, context.Namespace, nil
}

func newInClusterClient() (*client, error) {
	return nil, nil
}
