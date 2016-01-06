package dm

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/ghodss/yaml"
)

var DefaultHTTPTimeout time.Duration

type Client struct {
	// Timeout on HTTP connections.
	HTTPTimeout time.Duration
	// The remote host
	Host string
	// The protocol. Currently only http and https are supported.
	Protocol string
}

func NewClient(host string) *Client {
	return &Client{
		HTTPTimeout: DefaultHTTPTimeout,
		Protocol:    "https",
		Host:        host,
	}
}

func (c *Client) url(path string) string {
	// TODO: Switch to net.URL
	return c.Protocol + "://" + c.Host + "/" + path
}

// CallService is a low-level function for making an API call.
func (c *Client) CallService(path, method, action string, reader io.ReadCloser) {
	u := c.url(path)

	resp := c.callHttp(u, method, action, reader)
	var j interface{}
	if err := json.Unmarshal([]byte(resp), &j); err != nil {
		panic(fmt.Errorf("Failed to parse JSON response from service: %s", resp))
	}

	y, err := yaml.Marshal(j)
	if err != nil {
		panic(fmt.Errorf("Failed to serialize JSON response from service: %s", resp))
	}

	fmt.Println(string(y))
}

func (c *Client) callHttp(path, method, action string, reader io.ReadCloser) string {
	request, err := http.NewRequest(method, path, reader)
	request.Header.Add("Content-Type", "application/json")

	client := http.Client{
		Timeout: time.Duration(time.Duration(DefaultHTTPTimeout) * time.Second),
	}

	response, err := client.Do(request)
	if err != nil {
		panic(fmt.Errorf("cannot %s: %s\n", action, err))
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		panic(fmt.Errorf("cannot %s: %s\n", action, err))
	}

	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices {
		message := fmt.Sprintf("status code: %d status: %s : %s", response.StatusCode, response.Status, body)
		panic(fmt.Errorf("cannot %s: %s\n", action, message))
	}

	return string(body)
}
