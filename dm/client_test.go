package dm

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kubernetes/deployment-manager/common"
)

func TestDefaultServerURL(t *testing.T) {
	tt := []struct {
		host string
		url  string
	}{
		{"127.0.0.1", "http://127.0.0.1"},
		{"127.0.0.1:8080", "http://127.0.0.1:8080"},
		{"foo.bar.com", "http://foo.bar.com"},
		{"foo.bar.com/prefix", "http://foo.bar.com/prefix/"},
		{"http://host/prefix", "http://host/prefix/"},
		{"https://host/prefix", "https://host/prefix/"},
		{"http://host", "http://host"},
		{"http://host/other", "http://host/other/"},
	}

	for _, tc := range tt {
		u, err := DefaultServerURL(tc.host)
		if err != nil {
			t.Fatal(err)
		}

		if tc.url != u.String() {
			t.Errorf("%s, expected host %s, got %s", tc.host, tc.url, u.String())
		}
	}
}

func TestURL(t *testing.T) {
	tt := []struct {
		host string
		path string
		url  string
	}{
		{"127.0.0.1", "foo", "http://127.0.0.1/foo"},
		{"127.0.0.1:8080", "foo", "http://127.0.0.1:8080/foo"},
		{"foo.bar.com", "foo", "http://foo.bar.com/foo"},
		{"foo.bar.com/prefix", "foo", "http://foo.bar.com/prefix/foo"},
		{"http://host/prefix", "foo", "http://host/prefix/foo"},
		{"http://host", "foo", "http://host/foo"},
		{"http://host/other", "/foo", "http://host/foo"},
	}

	for _, tc := range tt {
		c := NewClient(tc.host)
		p, err := c.url(tc.path)
		if err != nil {
			t.Fatal(err)
		}

		if tc.url != p {
			t.Errorf("expected %s, got %s", tc.url, p)
		}
	}
}

type fakeClient struct {
	*Client
	server  *httptest.Server
	handler http.HandlerFunc
}

func (c *fakeClient) setup() *fakeClient {
	c.server = httptest.NewServer(c.handler)
	c.Client = NewClient(c.server.URL)
	return c
}

func (c *fakeClient) teardown() {
	c.server.Close()
}

func TestUserAgent(t *testing.T) {
	fc := &fakeClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.UserAgent(), "helm") {
				t.Error("user agent is not set")
			}
		}),
	}
	fc.setup().ListDeployments()
}

func TestListDeployments(t *testing.T) {
	fc := &fakeClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`["guestbook.yaml"]`))
		}),
	}
	defer fc.teardown()

	l, err := fc.setup().ListDeployments()
	if err != nil {
		t.Fatal(err)
	}

	if len(l) != 1 {
		t.Fatal("expected a single deployment")
	}
}

func TestGetDeployment(t *testing.T) {
	fc := &fakeClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"name":"guestbook.yaml","id":0,"createdAt":"2016-02-08T12:17:49.251658308-08:00","deployedAt":"2016-02-08T12:17:49.251658589-08:00","modifiedAt":"2016-02-08T12:17:51.177518098-08:00","deletedAt":"0001-01-01T00:00:00Z","state":{"status":"Deployed"},"latestManifest":"manifest-1454962670728402229"}`))
		}),
	}
	defer fc.teardown()

	d, err := fc.setup().GetDeployment("guestbook.yaml")
	if err != nil {
		t.Fatal(err)
	}

	if d.Name != "guestbook.yaml" {
		t.Fatalf("expected deployment name 'guestbook.yaml', got '%s'", d.Name)
	}

	if d.State.Status != common.DeployedStatus {
		t.Fatalf("expected deployment status 'Deployed', got '%s'", d.State.Status)
	}
}

func TestPostDeployment(t *testing.T) {
	cfg := &common.Configuration{
		[]*common.Resource{
			{
				Name: "foo",
				Type: "helm:example.com/foo/bar",
				Properties: map[string]interface{}{
					"port": ":8080",
				},
			},
		},
	}

	fc := &fakeClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintln(w, "{}")
		}),
	}
	defer fc.teardown()

	if err := fc.setup().PostDeployment(cfg); err != nil {
		t.Fatalf("failed to post deployment: %s", err)
	}
}
