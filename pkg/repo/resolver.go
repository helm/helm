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

package repo // import "helm.sh/helm/pkg/repo"

import (
	"context"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context/ctxhttp"
)

var _ remotes.Resolver = &Resolver{}

// Resolver provides remotes based on a locator
type Resolver struct {
	auth           docker.Authorizer
	host           func(string) (string, error)
	plainHTTP      bool
	client         *http.Client
	dockerResolver remotes.Resolver
}

// newResolver returns a new resolver to a Docker registry.
//
// We are simply wrapping its functionality to provide a way to fetch tag lists (see tagFetcher)
func newResolver(options docker.ResolverOptions) *Resolver {
	if options.Tracker == nil {
		options.Tracker = docker.NewInMemoryTracker()
	}
	if options.Host == nil {
		options.Host = docker.DefaultHost
	}
	if options.Authorizer == nil {
		options.Authorizer = docker.NewAuthorizer(options.Client, options.Credentials)
	}
	return &Resolver{
		auth:           options.Authorizer,
		host:           options.Host,
		plainHTTP:      options.PlainHTTP,
		client:         options.Client,
		dockerResolver: docker.NewResolver(options),
	}
}

func (r *Resolver) Resolve(ctx context.Context, ref string) (string, ocispec.Descriptor, error) {
	return r.dockerResolver.Resolve(ctx, ref)
}

func (r *Resolver) Fetcher(ctx context.Context, ref string) (remotes.Fetcher, error) {
	return r.dockerResolver.Fetcher(ctx, ref)
}

func (r *Resolver) Pusher(ctx context.Context, ref string) (remotes.Pusher, error) {
	return r.dockerResolver.Pusher(ctx, ref)
}

func (r *Resolver) base(refspec reference.Spec) (*baseResolver, error) {
	var (
		err  error
		base url.URL
	)

	host := refspec.Hostname()
	base.Host = host
	if r.host != nil {
		base.Host, err = r.host(host)
		if err != nil {
			return nil, err
		}
	}

	base.Scheme = "https"
	if r.plainHTTP || strings.HasPrefix(base.Host, "localhost:") {
		base.Scheme = "http"
	}

	prefix := strings.TrimPrefix(refspec.Locator, host+"/")
	base.Path = path.Join("/v2", prefix)

	return &baseResolver{
		refspec: refspec,
		base:    base,
		client:  r.client,
		auth:    r.auth,
	}, nil
}

type baseResolver struct {
	refspec reference.Spec
	base    url.URL

	client *http.Client
	auth   docker.Authorizer
}

func (b *baseResolver) url(ps ...string) string {
	url := b.base
	url.Path = path.Join(url.Path, path.Join(ps...))
	return url.String()
}

func (b *baseResolver) authorize(ctx context.Context, req *http.Request) error {
	// Check if has header for host
	if b.auth != nil {
		if err := b.auth.Authorize(ctx, req); err != nil {
			return err
		}
	}

	return nil
}

func (b *baseResolver) doRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("url", req.URL.String()))
	log.G(ctx).WithField("request.headers", req.Header).WithField("request.method", req.Method).Debug("do request")
	if err := b.authorize(ctx, req); err != nil {
		return nil, errors.Wrap(err, "failed to authorize")
	}
	resp, err := ctxhttp.Do(ctx, b.client, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to do request")
	}
	log.G(ctx).WithFields(logrus.Fields{
		"status":           resp.Status,
		"response.headers": resp.Header,
	}).Debug("fetch response received")
	return resp, nil
}

func (b *baseResolver) doRequestWithRetries(ctx context.Context, req *http.Request, responses []*http.Response) (*http.Response, error) {
	resp, err := b.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	responses = append(responses, resp)
	req, err = b.retryRequest(ctx, req, responses)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}
	if req != nil {
		resp.Body.Close()
		return b.doRequestWithRetries(ctx, req, responses)
	}
	return resp, err
}

func (b *baseResolver) retryRequest(ctx context.Context, req *http.Request, responses []*http.Response) (*http.Request, error) {
	if len(responses) > 5 {
		return nil, nil
	}
	last := responses[len(responses)-1]
	if last.StatusCode == http.StatusUnauthorized {
		log.G(ctx).WithField("header", last.Header.Get("WWW-Authenticate")).Debug("Unauthorized")
		if b.auth != nil {
			if err := b.auth.AddResponses(ctx, responses); err == nil {
				return copyRequest(req)
			} else if !errdefs.IsNotImplemented(err) {
				return nil, err
			}
		}

		return nil, nil
	} else if last.StatusCode == http.StatusMethodNotAllowed && req.Method == http.MethodHead {
		// Support registries which have not properly implemented the HEAD method for
		// manifests endpoint
		if strings.Contains(req.URL.Path, "/manifests/") {
			// TODO: copy request?
			req.Method = http.MethodGet
			return copyRequest(req)
		}
	}

	// TODO: Handle 50x errors accounting for attempt history
	return nil, nil
}

func copyRequest(req *http.Request) (*http.Request, error) {
	ireq := *req
	if ireq.GetBody != nil {
		var err error
		ireq.Body, err = ireq.GetBody()
		if err != nil {
			return nil, err
		}
	}
	return &ireq, nil
}
