/*
Copyright The Helm Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package getter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

type TestFileHandler struct{}

func (h *TestFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	HandleClient(w, r)
}

func TestHTTPGetter(t *testing.T) {
	g, err := newHTTPGetter("http://example.com", "", "", "")
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := g.(*HttpGetter); !ok {
		t.Fatal("Expected newHTTPGetter to produce an HttpGetter")
	}

	// Test with SSL:
	cd := "../../testdata"
	join := filepath.Join
	ca, pub, priv := join(cd, "ca.pem"), join(cd, "crt.pem"), join(cd, "key.pem")
	g, err = newHTTPGetter("https://example.com/", pub, priv, ca)
	if err != nil {
		t.Fatal(err)
	}
	if hg, ok := g.(*HttpGetter); !ok {
		t.Fatal("Expected newHTTPGetter to produce an HttpGetter")
	} else if hg.client == http.DefaultClient {
		t.Fatal("Expected newHTTPGetter to return a non-default HTTP client")
	}

	// Test with SSL, caFile only
	g, err = newHTTPGetter("https://example.com/", "", "", ca)
	if err != nil {
		t.Fatal(err)
	}
	if hg, ok := g.(*HttpGetter); !ok {
		t.Fatal("Expected newHTTPGetter to produce an HttpGetter")
	} else if hg.client == http.DefaultClient {
		t.Fatal("Expected newHTTPGetter to return a non-default HTTP client")
	}
}

func HandleClient(writer http.ResponseWriter, request *http.Request) {
	f, _ := os.Open("testdata/sssd-0.1.0.tgz")
	defer f.Close()

	b := make([]byte, 512)
	f.Read(b)
	//Get the file size
	FileStat, _ := f.Stat()
	FileSize := strconv.FormatInt(FileStat.Size(), 10)

	//Simulating improper header values from bitbucket
	writer.Header().Set("Content-Type", "application/x-tar")
	writer.Header().Set("Content-Encoding", "gzip")
	writer.Header().Set("Content-Length", FileSize)

	f.Seek(0, 0)
	io.Copy(writer, f)
	return
}

func TestHTTPGetterTarDownload(t *testing.T) {
	h := &TestFileHandler{}
	server := httptest.NewServer(h)
	defer server.Close()

	g, err := newHTTPGetter(server.URL, "", "", "")
	if err != nil {
		t.Fatal(err)
	}

	data, _ := g.Get(server.URL)
	mimeType := http.DetectContentType(data.Bytes())

	expectedMimeType := "application/x-gzip"
	if mimeType != expectedMimeType {
		t.Fatalf("Expected response with MIME type %s, but got %s", expectedMimeType, mimeType)
	}
}
