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
	"github.com/kubernetes/deployment-manager/registry"

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
	// TODO(jackgr): Implement reading a template from stdin
	//stdin       = flag.Bool("stdin", false, "Reads a template from the standard input")
	properties    = flag.String("properties", "", "Properties to use when deploying a type (e.g., --properties k1=v1,k2=v2)")
	type_registry = flag.String("registry", "kubernetes/deployment-manager", "Github based type registry [owner/repo]")
	service       = flag.String("service", "http://localhost:8001/api/v1/proxy/namespaces/default/services/manager-service:manager", "URL for deployment manager")
	binary        = flag.String("binary", "../expandybird/expansion/expansion.py", "Path to template expansion binary")
)

var commands = []string{
	"expand \t\t\t Expands the supplied template(s)",
	"deploy \t\t\t Deploys the supplied type or template(s)",
	"list \t\t\t Lists the deployments in the cluster",
	"get \t\t\t Retrieves the supplied deployment",
	"delete \t\t\t Deletes the supplied deployment",
	"update \t\t\t Updates a deployment using the supplied template(s)",
	"deployed-types \t\t Lists the types deployed in the cluster",
	"deployed-instances \t Lists the instances of the supplied type deployed in the cluster",
	"types \t\t\t Lists the types in the current registry",
	"describe \t\t Describes the supplied type in the current registry",
}

var usage = func() {
	message := "Usage: %s [<flags>] <command> (<type-name> | <deployment-name> | (<template> [<import1>...<importN>]))\n"
	fmt.Fprintf(os.Stderr, message, os.Args[0])
	fmt.Fprintln(os.Stderr, "Commands:")
	for _, command := range commands {
		fmt.Fprintln(os.Stderr, command)
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Flags:")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr)
	os.Exit(1)
}

func getGitRegistry() *registry.GithubRegistry {
	s := strings.Split(*type_registry, "/")
	if len(s) != 2 {
		log.Fatalf("invalid type registry: %s", type_registry)
	}

	return registry.NewGithubRegistry(s[0], s[1])
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "No command supplied")
		usage()
	}

	command := args[0]
	switch command {
	case "types":
		git := getGitRegistry()
		types, err := git.List()
		if err != nil {
			log.Fatalf("Cannot list %v err")
		}

		fmt.Printf("Types:")
		for _, t := range types {
			fmt.Printf("%s:%s", t.Name, t.Version)
			downloadURL, err := git.GetURL(t)
			if err != nil {
				log.Printf("Failed to get download URL for type %s:%s", t.Name, t.Version)
			}

			fmt.Printf("\tdownload URL: %s", downloadURL)
		}
	case "describe":
		fmt.Printf("this feature is not yet implemented")
	case "expand":
		backend := expander.NewExpander(*binary)
		template := loadTemplate(args)
		output, err := backend.ExpandTemplate(template)
		if err != nil {
			log.Fatalf("cannot expand %s: %s\n", template.Name, err)
		}

		fmt.Println(output)
	case "deploy":
		template := loadTemplate(args)
		action := fmt.Sprintf("deploy template named %s", template.Name)
		callService("deployments", "POST", action, marshalTemplate(template))
	case "list":
		callService("deployments", "GET", "list deployments", nil)
	case "get":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "No deployment name supplied")
			usage()
		}

		path := fmt.Sprintf("deployments/%s", args[1])
		action := fmt.Sprintf("get deployment named %s", args[1])
		callService(path, "GET", action, nil)
	case "delete":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "No deployment name supplied")
			usage()
		}

		path := fmt.Sprintf("deployments/%s", args[1])
		action := fmt.Sprintf("delete deployment named %s", args[1])
		callService(path, "DELETE", action, nil)
	case "update":
		template := loadTemplate(args)
		path := fmt.Sprintf("deployments/%s", template.Name)
		action := fmt.Sprintf("delete deployment named %s", template.Name)
		callService(path, "PUT", action, marshalTemplate(template))
	case "deployed-types":
		action := fmt.Sprintf("list types in registry %s", *type_registry)
		callService("types", "GET", action, nil)
	case "deployed-instances":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "No type name supplied")
			usage()
		}

		path := fmt.Sprintf("types/%s/instances", url.QueryEscape(args[1]))
		action := fmt.Sprintf("list instances of type %s in registry %s", args[1], *type_registry)
		callService(path, "GET", action, nil)
	default:
		usage()
	}
}

func callService(path, method, action string, reader io.ReadCloser) {
	u := fmt.Sprintf("%s/%s", *service, path)
	request, err := http.NewRequest(method, u, reader)
	request.Header.Add("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Fatalf("cannot %s: %s\n", action, err)
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatalf("cannot %s: %s\n", action, err)
	}

	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices {
		message := fmt.Sprintf("status code: %d status: %s : %s", response.StatusCode, response.Status, body)
		log.Fatalf("cannot %s: %s\n", action, message)
	}

	fmt.Println(string(body))
}

func loadTemplate(args []string) *expander.Template {
	var template *expander.Template
	var err error
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "No type name or template file(s) supplied")
		usage()
	}

	if len(args) < 3 {
		if t := getRegistryType(args[1]); t != nil {
			template = buildTemplateFromType(args[1], *t)
		} else {
			template, err = expander.NewTemplateFromRootTemplate(args[1])
		}
	} else {
		template, err = expander.NewTemplateFromFileNames(args[1], args[2:])
	}

	if err != nil {
		log.Fatalf("cannot create template from supplied arguments: %s\n", err)
	}

	return template
}

// TODO: needs better validation that this is actually a registry type.
func getRegistryType(fullType string) *registry.Type {
	tList := strings.Split(fullType, ":")
	if len(tList) != 2 {
		return nil
	}

	return &registry.Type{
		Name:    tList[0],
		Version: tList[1],
	}
}

func buildTemplateFromType(name string, t registry.Type) *expander.Template {
	git := getGitRegistry()
	downloadURL, err := git.GetURL(t)
	if err != nil {
		log.Fatalf("Failed to get download URL for type %s:%s\n%s\n", t.Name, t.Version, err)
	}

	props := make(map[string]interface{})
	if *properties != "" {
		plist := strings.Split(*properties, ",")
		for _, p := range plist {
			ppair := strings.Split(p, "=")
			if len(ppair) != 2 {
				log.Fatalf("--properties must be in the form \"p1=v1,p2=v2,...\": %s\n", p)
			}

			// support ints
			// TODO: needs to support other types.
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
		Type:       downloadURL,
		Properties: props,
	}}}

	y, err := yaml.Marshal(config)
	if err != nil {
		log.Fatalf("error: %s\ncannot create configuration for deployment: %v\n", err, config)
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

func getRandomName() string {
	return fmt.Sprintf("manifest-%d", time.Now().UTC().UnixNano())
}
