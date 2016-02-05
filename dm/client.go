package dm

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ghodss/yaml"
)

// The default HTTP timeout
var DefaultHTTPTimeout time.Duration = time.Second * 10

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
}

// NewClient creates a new DM client. Host name is required.
func NewClient(host string) *Client {
	return &Client{
		HTTPTimeout: DefaultHTTPTimeout,
		Protocol:    "https",
		Host:        host,
		Transport:   NewDebugTransport(nil),
	}
}

// url constructs the URL.
func (c *Client) url(path string) string {
	// TODO: Switch to net.URL
	return c.Protocol + "://" + c.Host + "/" + path
}

// CallService is a low-level function for making an API call.
//
// This calls the service and then unmarshals the returned data into dest.
func (c *Client) CallService(path, method, action string, dest interface{}, reader io.ReadCloser) error {
	u := c.url(path)

	resp, err := c.callHttp(u, method, action, reader)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(resp), dest); err != nil {
		return fmt.Errorf("Failed to parse JSON response from service: %s", resp)
	}

	// From here down is just printing the data.

	y, err := yaml.Marshal(dest)
	if err != nil {
		return fmt.Errorf("Failed to serialize JSON response from service: %s", resp)
	}

	fmt.Println(string(y))
	return nil
}

// callHttp is  a low-level primative for executing HTTP operations.
func (c *Client) callHttp(path, method, action string, reader io.ReadCloser) (string, error) {
	request, err := http.NewRequest(method, path, reader)
	request.Header.Add("Content-Type", "application/json")

	client := http.Client{
		Timeout:   time.Duration(time.Duration(DefaultHTTPTimeout) * time.Second),
		Transport: c.Transport,
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

func (c *Client) ListDeployments() error {
	var d interface{}
	if err := c.CallService("deployments", "GET", "foo", &d, nil); err != nil {
		return err
	}

	fmt.Printf("%#v\n", d)
	return nil
}

func (c *Client) DeployChart(filename, deployname string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	request, err := http.NewRequest("POST", "/v2/deployments/", f)

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
		Transport: c.Transport,
	}

	response, err := client.Do(request)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		return err
	}

	// FIXME: We only want 200 OK or 204(?) CREATED
	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices {
		message := fmt.Sprintf("status code: %d status: %s : %s", response.StatusCode, response.Status, body)
		return fmt.Errorf("Failed to post: %s", message)
	}

	return nil
}
