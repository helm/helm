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

package repo

import (
	"context"
	"io"
	"net/http"
	"path"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type tagsResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

type tagFetcher struct {
	*baseResolver
}

func (f tagFetcher) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(
		logrus.Fields{
			"base":   f.base.String(),
			"digest": desc.Digest,
		},
	))

	url := f.url(path.Join("tags", "list"))

	return f.open(ctx, url)
}

func (f tagFetcher) open(ctx context.Context, u string) (io.ReadCloser, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := f.doRequestWithRetries(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode > 299 {
		// TODO(stevvooe): When doing a offset specific request, we should
		// really distinguish between a 206 and a 200. In the case of 200, we
		// can discard the bytes, hiding the seek behavior from the
		// implementation.

		resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return nil, errors.Wrapf(errdefs.ErrNotFound, "content at %v not found", u)
		}
		return nil, errors.Errorf("unexpected status code %v: %v", u, resp.Status)
	}

	return resp.Body, nil
}
