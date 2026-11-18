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

package driver

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"slices"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/pager"

	rspb "helm.sh/helm/v4/pkg/release/v1"
)

var b64 = base64.StdEncoding

var magicGzip = []byte{0x1f, 0x8b, 0x08}

// listPageSize is the maximum number of objects requested from the API server
// per List call by the Secrets and ConfigMaps drivers. Fetching every release
// record in one unpaginated request can run into the API server's request
// timeout when a namespace holds many (or large) release records.
var listPageSize int64 = 500

var systemLabels = []string{"name", "owner", "status", "version", "createdAt", "modifiedAt"}

// encodeRelease encodes a release returning a base64 encoded
// gzipped string representation, or error.
func encodeRelease(rls *rspb.Release) (string, error) {
	b, err := json.Marshal(rls)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return "", err
	}
	if _, err = w.Write(b); err != nil {
		w.Close()
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	return b64.EncodeToString(buf.Bytes()), nil
}

// decodeRelease decodes the bytes of data into a release
// type. Data must contain a base64 encoded gzipped string of a
// valid release, otherwise an error is returned.
func decodeRelease(data string) (*rspb.Release, error) {
	// base64 decode string
	b, err := b64.DecodeString(data)
	if err != nil {
		return nil, err
	}

	// For backwards compatibility with releases that were stored before
	// compression was introduced we skip decompression if the
	// gzip magic header is not found
	if len(b) > 3 && bytes.Equal(b[0:3], magicGzip) {
		r, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		b2, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		b = b2
	}

	var rls rspb.Release
	// unmarshal release object bytes
	if err := json.Unmarshal(b, &rls); err != nil {
		return nil, err
	}
	return &rls, nil
}

// Checks if label is system
func isSystemLabel(key string) bool {
	return slices.Contains(GetSystemLabels(), key)
}

// Removes system labels from labels map
func filterSystemLabels(lbs map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range lbs {
		if !isSystemLabel(k) {
			result[k] = v
		}
	}
	return result
}

// Checks if labels array contains system labels
func ContainsSystemLabels(lbs map[string]string) bool {
	for k := range lbs {
		if isSystemLabel(k) {
			return true
		}
	}
	return false
}

func GetSystemLabels() []string {
	return systemLabels
}

// listAllPaged lists every object matching opts using paginated requests and
// calls fn for each item. If the continue token expires between pages, it
// falls back to a single full list, like client-go's pager does.
func listAllPaged[T runtime.Object](ctx context.Context, opts metav1.ListOptions, pageFn pager.ListPageFunc, fn func(T)) (int, error) {
	p := pager.New(pageFn)
	p.PageSize = listPageSize

	obj, _, err := p.List(ctx, opts)
	if err != nil {
		return 0, err
	}

	count := 0
	err = meta.EachListItem(obj, func(o runtime.Object) error {
		item, ok := o.(T)
		if !ok {
			return fmt.Errorf("unexpected list item type %T", o)
		}
		count++
		fn(item)
		return nil
	})
	return count, err
}
