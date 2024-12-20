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

package netrc

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// DefaultPath returns the default path to the .netrc file
func DefaultPath() string {
	if os.Getenv("NETRC") != "" {
		return os.Getenv("NETRC")
	}
	return filepath.Join(os.Getenv("HOME"), ".netrc")
}

// Credentials represents a machine entry in .netrc file
type Credentials struct {
	Machine  string
	Login    string
	Password string
}

// GetCredentials returns the credentials for the given URL from .netrc file
func GetCredentials(urlStr string) (*Credentials, error) {
	netrcPath := DefaultPath()
	if _, err := os.Stat(netrcPath); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := os.ReadFile(netrcPath)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	host := u.Host
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}

	parser := newParser(string(data))
	machines, err := parser.parse()
	if err != nil {
		return nil, err
	}

	for _, m := range machines {
		if m.Machine == host {
			return &Credentials{
				Machine:  m.Machine,
				Login:    m.Login,
				Password: m.Password,
			}, nil
		}
	}

	return nil, nil
}

type machine struct {
	Machine  string
	Login    string
	Password string
}

type parser struct {
	input string
	pos   int
}

func newParser(input string) *parser {
	return &parser{input: input}
}

func (p *parser) parse() ([]machine, error) {
	var machines []machine
	var current machine

	for p.pos < len(p.input) {
		token := p.nextToken()
		if token == "" {
			break
		}

		switch token {
		case "machine":
			if current.Machine != "" {
				machines = append(machines, current)
				current = machine{}
			}
			current.Machine = p.nextToken()
		case "login":
			current.Login = p.nextToken()
		case "password":
			current.Password = p.nextToken()
		}
	}

	if current.Machine != "" {
		machines = append(machines, current)
	}

	return machines, nil
}

func (p *parser) nextToken() string {
	// Skip whitespace
	for p.pos < len(p.input) && (p.input[p.pos] == ' ' || p.input[p.pos] == '\t' || p.input[p.pos] == '\n' || p.input[p.pos] == '\r') {
		p.pos++
	}

	if p.pos >= len(p.input) {
		return ""
	}

	start := p.pos
	if p.input[p.pos] == '"' {
		p.pos++
		start = p.pos
		for p.pos < len(p.input) && p.input[p.pos] != '"' {
			p.pos++
		}
		if p.pos < len(p.input) {
			token := p.input[start:p.pos]
			p.pos++
			return token
		}
	} else {
		for p.pos < len(p.input) && p.input[p.pos] != ' ' && p.input[p.pos] != '\t' && p.input[p.pos] != '\n' && p.input[p.pos] != '\r' {
			p.pos++
		}
	}

	return p.input[start:p.pos]
}
