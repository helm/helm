/*
Copyright 2015 The Kubernetes Authors All rights reserved.
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

package main

import (
	"expandybird/expander"

	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	action  = flag.String("action", "deploy", "expand | deploy | list | get | delete | update")
	name    = flag.String("name", "", "Name of template or deployment")
	service = flag.String("service", "http://localhost:8080", "URL for deployment manager")
	binary  = flag.String("binary", "../expandybird/expansion/expansion.py",
		"Path to template expansion binary")
)

var usage = func() {
	message := "usage: %s [<flags>] (name | (<template> [<import1>...<importN>]))\n"
	fmt.Fprintf(os.Stderr, message, os.Args[0])
	flag.PrintDefaults()
}

func main() {
	flag.Parse()
	name := getNameArgument()
	switch *action {
	case "expand":
		backend := expander.NewExpander(*binary)
		template := loadTemplate(name)
		output, err := backend.ExpandTemplate(template)
		if err != nil {
			log.Fatalf("cannot expand %s: %s\n", name, err)
		}

		fmt.Println(output)
	case "deploy":
		callService("deployments", "POST", name, readTemplate(name))
	case "list":
		callService("deployments", "GET", name, nil)
	case "get":
		path := fmt.Sprintf("deployments/%s", name)
		callService(path, "GET", name, nil)
	case "delete":
		path := fmt.Sprintf("deployments/%s", name)
		callService(path, "DELETE", name, nil)
	case "update":
		path := fmt.Sprintf("deployments/%s", name)
		callService(path, "PUT", name, readTemplate(name))
	}
}

func callService(path, method, name string, reader io.ReadCloser) {
	action := strings.ToLower(method)
	if action == "post" {
		action = "deploy"
	}

	url := fmt.Sprintf("%s/%s", *service, path)
	request, err := http.NewRequest(method, url, reader)
	request.Header.Add("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Fatalf("cannot %s template named %s: %s\n", action, name, err)
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatalf("cannot %s template named %s: %s\n", action, name, err)
	}

	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices {
		message := fmt.Sprintf("status code: %d status: %s", response.StatusCode, response.Status)
		log.Fatalf("cannot %s template named %s: %s\n", action, name, message)
	}

	fmt.Println(string(body))
}

func readTemplate(name string) io.ReadCloser {
	return marshalTemplate(loadTemplate(name))
}

func loadTemplate(name string) *expander.Template {
	args := flag.Args()
	if len(args) < 1 {
		usage()
		os.Exit(1)
	}

	var template *expander.Template
	var err error
	if len(args) == 1 {
		template, err = expander.NewTemplateFromRootTemplate(args[0])
	} else {
		template, err = expander.NewTemplateFromFileNames(args[0], args[1:])
	}
	if err != nil {
		log.Fatalf("cannot create template from supplied file names: %s\n", err)
	}

	if name != "" {
		template.Name = name
	}

	return template
}

func marshalTemplate(template *expander.Template) io.ReadCloser {
	j, err := json.Marshal(template)
	if err != nil {
		log.Fatalf("cannot deploy template %s: %s\n", template.Name, err)
	}

	return ioutil.NopCloser(bytes.NewReader(j))
}

func getNameArgument() string {
	if *name == "" {
		*name = fmt.Sprintf("manifest-%d", time.Now().UTC().UnixNano())
	}

	return *name
}
