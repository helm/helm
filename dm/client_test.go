package dm

import (
	"testing"
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
