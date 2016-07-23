package kubernetes

import "testing"

func TestLoadClient(t *testing.T) {
	loadClient(t)
}

func loadClient(t *testing.T) *client {
	config, err := loadKubeConfig()
	if err != nil {
		t.Skip()
	}
	cli, err := newClient(config)
	if err != nil {
		t.Fatal(err)
	}
	return cli
}

func TestURLFor(t *testing.T) {
	tests := []struct {
		apiGroup, version, namespace, resource, name string

		baseURL string
		want    string
	}{
		{
			"", "v1", "default", "pods", "a",
			"https://k8s.example.com",
			"https://k8s.example.com/api/v1/namespaces/default/pods/a",
		},
		{
			"foo", "v1", "default", "bar", "a",
			"https://k8s.example.com",
			"https://k8s.example.com/apis/foo/v1/namespaces/default/bar/a",
		},
		{
			"foo", "v1", "default", "bar", "a",
			"https://k8s.example.com/",
			"https://k8s.example.com/apis/foo/v1/namespaces/default/bar/a",
		},
		{
			"foo", "v1", "default", "bar", "a",
			"https://k8s.example.com/",
			"https://k8s.example.com/apis/foo/v1/namespaces/default/bar/a",
		},
		{
			// no namespace
			"foo", "v1", "", "bar", "a",
			"https://k8s.example.com",
			"https://k8s.example.com/apis/foo/v1/bar/a",
		},
	}

	for _, test := range tests {
		c := &client{baseURL: test.baseURL}
		got := c.urlFor(test.apiGroup, test.version, test.namespace, test.resource, test.name)
		if got != test.want {
			t.Errorf("(&client{baseURL:%q}).urlFor(%q, %q, %q, %q, %q): expected %q got %q",
				test.baseURL,
				test.apiGroup, test.version, test.namespace, test.resource, test.name,
				test.want, got,
			)
		}
	}
}
