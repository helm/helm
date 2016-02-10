package dm

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	fancypath "path"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubernetes/deployment-manager/common"
)

// The default HTTP timeout
var DefaultHTTPTimeout = time.Second * 10

// The default HTTP Protocol
var DefaultHTTPProtocol = "http"

// Client is a DM client.
type Client struct {
	// Timeout on HTTP connections.
	HTTPTimeout time.Duration
	// The remote host
	Host string
	// The protocol. Currently only http and https are supported.
	Protocol string
	// Transport
	Transport http.RoundTripper
	// Debug enables http logging
	Debug bool

	// Base URL for remote service
	baseURL *url.URL
}

// NewClient creates a new DM client. Host name is required.
func NewClient(host string) *Client {
	url, _ := DefaultServerURL(host)

	return &Client{
		HTTPTimeout: DefaultHTTPTimeout,
		baseURL:     url,
		Transport:   http.DefaultTransport,
	}
}

// SetDebug enables debug mode which logs http
func (c *Client) SetDebug(enable bool) *Client {
	c.Debug = enable
	return c
}

// transport wraps client transport if debug is enabled
func (c *Client) transport() http.RoundTripper {
	if c.Debug {
		return NewDebugTransport(c.Transport)
	}
	return c.Transport
}

// SetTransport sets a custom Transport. Defaults to http.DefaultTransport
func (c *Client) SetTransport(tr http.RoundTripper) *Client {
	c.Transport = tr
	return c
}

// SetTimeout sets a timeout for http connections
func (c *Client) SetTimeout(seconds int) *Client {
	c.HTTPTimeout = time.Duration(time.Duration(seconds) * time.Second)
	return c
}

// url constructs the URL.
func (c *Client) url(rawurl string) (string, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return "", err
	}
	return c.baseURL.ResolveReference(u).String(), nil
}

// CallService is a low-level function for making an API call.
//
// This calls the service and then unmarshals the returned data into dest.
func (c *Client) CallService(path, method, action string, dest interface{}, reader io.ReadCloser) error {
	u, err := c.url(path)
	if err != nil {
		return err
	}

	resp, err := c.callHTTP(u, method, action, reader)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(resp), dest); err != nil {
		return fmt.Errorf("Failed to parse JSON response from service: %s", resp)
	}
	return nil
}

// callHTTP is  a low-level primative for executing HTTP operations.
func (c *Client) callHTTP(path, method, action string, reader io.ReadCloser) (string, error) {
	request, err := http.NewRequest(method, path, reader)

	// TODO: dynamically set version
	request.Header.Set("User-Agent", "helm/0.0.1")
	request.Header.Add("Content-Type", "application/json")

	client := http.Client{
		Timeout:   time.Duration(time.Duration(DefaultHTTPTimeout) * time.Second),
		Transport: c.transport(),
	}

	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("cannot %s: %s\n", action, err)
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("cannot %s: %s\n", action, err)
	}

	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices {
		message := fmt.Sprintf("status code: %d status: %s : %s", response.StatusCode, response.Status, body)
		return "", fmt.Errorf("cannot %s: %s\n", action, message)
	}

	return string(body), nil
}

// DefaultServerURL converts a host, host:port, or URL string to the default base server API path
// to use with a Client
func DefaultServerURL(host string) (*url.URL, error) {
	if host == "" {
		return nil, fmt.Errorf("host must be a URL or a host:port pair")
	}
	base := host
	hostURL, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	if hostURL.Scheme == "" {
		hostURL, err = url.Parse(DefaultHTTPProtocol + "://" + base)
		if err != nil {
			return nil, err
		}
	}
	if len(hostURL.Path) > 0 && !strings.HasSuffix(hostURL.Path, "/") {
		hostURL.Path = hostURL.Path + "/"
	}

	return hostURL, nil
}

// ListDeployments lists the deployments in DM.
func (c *Client) ListDeployments() ([]string, error) {
	var l []string
	if err := c.CallService("deployments", "GET", "list deployments", &l, nil); err != nil {
		return nil, err
	}

	return l, nil
}

// DeployChart sends a chart to DM for deploying.
func (c *Client) DeployChart(filename, deployname string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}

	u, err := c.url("/v2/deployments")
	request, err := http.NewRequest("POST", u, f)
	if err != nil {
		f.Close()
		return err
	}

	// There is an argument to be made for using the legacy x-octet-stream for
	// this. But since we control both sides, we should use the standard one.
	// Also, gzip (x-compress) is usually treated as a content encoding. In this
	// case it probably is not, but it makes more sense to follow the standard,
	// even though we don't assume the remote server will strip it off.
	request.Header.Add("Content-Type", "application/x-tar")
	request.Header.Add("Content-Encoding", "gzip")
	request.Header.Add("X-Deployment-Name", deployname)
	request.Header.Add("X-Chart-Name", filepath.Base(filename))

	client := http.Client{
		Timeout:   time.Duration(time.Duration(DefaultHTTPTimeout) * time.Second),
		Transport: c.transport(),
	}

	response, err := client.Do(request)
	if err != nil {
		return err
	}

	// We only want 201 CREATED. Admittedly, we could accept 200 and 202.
	if response.StatusCode < http.StatusCreated {
		body, err := ioutil.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			return err
		}
		return fmt.Errorf("failed to post: %d %s - %s", response.StatusCode, response.Status, body)
	}

	return nil
}

// GetDeployment retrieves the supplied deployment
func (c *Client) GetDeployment(name string) (*common.Deployment, error) {
	var deployment *common.Deployment
	if err := c.CallService(fancypath.Join("deployments", name), "GET", "get deployment", &deployment, nil); err != nil {
		return nil, err
	}
	return deployment, nil
}
