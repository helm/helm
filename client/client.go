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
	"github.com/ghodss/yaml"

	"github.com/kubernetes/deployment-manager/expandybird/expander"
	"github.com/kubernetes/deployment-manager/manager/manager"

	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	action  = flag.String("action", "deploy", "expand | deploy | list | get | delete | update | listtypes | listtypeinstances")
	name    = flag.String("name", "", "Name of template or deployment")
	service = flag.String("service", "http://localhost:8080", "URL for deployment manager")
	binary  = flag.String("binary", "../expandybird/expansion/expansion.py",
		"Path to template expansion binary")

	properties = flag.String("properties", "", "Properties to use when deploying a type")
	repository = flag.String("repository",
		"https://raw.githubusercontent.com/kubernetes/deployment-manager/master/examples",
		"Root of type repository")
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
	case "listtypes":
		callService("types", "GET", name, nil)
	case "listtypeinstances":
		path := fmt.Sprintf("types/%s/instances", url.QueryEscape(name))
		callService(path, "GET", name, nil)
	}
}

func callService(path, method, name string, reader io.ReadCloser) {
	action := strings.ToLower(method)
	if action == "post" {
		action = "deploy"
	}

	u := fmt.Sprintf("%s/%s", *service, path)
	request, err := http.NewRequest(method, u, reader)
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
		message := fmt.Sprintf("status code: %d status: %s : %s", response.StatusCode, response.Status, body)
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
		if strings.HasSuffix(args[0], ".py") || strings.HasSuffix(args[0], ".jinja") {
			template = buildTemplateFromType(name, args[0])
		} else {
			template, err = expander.NewTemplateFromRootTemplate(args[0])
		}
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

func buildTemplateFromType(name string, typeName string) *expander.Template {
	// TODO: Fill this in with an actual URL fetched from github.
	fullType := *repository + "/" + typeName

	props := make(map[string]interface{})
	if *properties != "" {
		plist := strings.Split(*properties, ",")
		for _, p := range plist {
			ppair := strings.Split(p, "=")
			if len(ppair) != 2 {
				log.Fatalf("--properties must be in the form \"p1=v1,p2=v2,...\": %s", p)
			}

			// support ints
			// TODO: needs to done to support other types.
			i, err := strconv.Atoi(ppair[1])
			if err != nil {
				props[ppair[0]] = ppair[1]
			} else {
				props[ppair[0]] = i
			}
		}
	}

	config := manager.Configuration{Resources: []*manager.Resource{&manager.Resource{
		Name:       name,
		Type:       fullType,
		Properties: props,
	}}}

	y, err := yaml.Marshal(config)
	if err != nil {
		log.Fatalf("cannot create configuration for deployment: %v\n", config)
	}

	return &expander.Template{
		// Name will be set later.
		Content: string(y),
		// No imports, as this is a single type from repository.
	}
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
