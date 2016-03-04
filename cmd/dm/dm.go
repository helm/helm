/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

	"github.com/kubernetes/deployment-manager/pkg/common"
	"github.com/kubernetes/deployment-manager/pkg/util"

	"archive/tar"
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
	deploymentName = flag.String("name", "", "Name of deployment, used for deploy and update commands (defaults to template name)")
	stdin          = flag.Bool("stdin", false, "Reads a configuration from the standard input")
	properties     = flag.String("properties", "", "Properties to use when deploying a template (e.g., --properties k1=v1,k2=v2)")
	// TODO(vaikas): CHange the default name once we figure out where the charts live.
	templateRegistry = flag.String("registry", "application-dm-templates", "Registry name")
	service          = flag.String("service", "http://localhost:8001/api/v1/proxy/namespaces/dm/services/manager-service:manager", "URL for deployment manager")
	binary           = flag.String("binary", "../expandybird/expansion/expansion.py", "Path to template expansion binary")
	timeout          = flag.Int("timeout", 20, "Time in seconds to wait for response")
	regexString      = flag.String("regex", "", "Regular expression to filter the templates listed in a template registry")
	username         = flag.String("username", "", "Github user name that overrides GITHUB_USERNAME environment variable")
	password         = flag.String("password", "", "Github password that overrides GITHUB_PASSWORD environment variable")
	apitoken         = flag.String("apitoken", "", "Github api token that overrides GITHUB_API_TOKEN environment variable")
	serviceaccount   = flag.String("serviceaccount", "", "Service account file containing JWT token")
	registryfile     = flag.String("registryfile", "", "File containing registry specification")
)

var commands = []string{
	"expand \t\t\t Expands the supplied configuration(s)",
	"deploy \t\t\t Deploys the named template or the supplied configuration(s)",
	"list \t\t\t Lists the deployments in the cluster",
	"get \t\t\t Retrieves the supplied deployment",
	"manifest \t\t\t Lists manifests for deployment or retrieves the supplied manifest in the form (deployment[/manifest])",
	"delete \t\t\t Deletes the supplied deployment",
	"update \t\t\t Updates a deployment using the supplied configuration(s)",
	"deployed-types \t\t Lists the types deployed in the cluster",
	"deployed-instances \t Lists the instances of the named type deployed in the cluster",
	"templates \t\t Lists the templates in a given template registry (specified with --registry)",
	"registries \t\t Lists the registries available",
	"describe \t\t Describes the named template in a given template registry",
	"getcredential \t\t Gets the named credential used by a registry",
	"setcredential \t\t Sets a credential used by a registry",
	"createregistry \t\t Creates a registry that holds charts",
}

var usage = func() {
	message := "Usage: %s [<flags>] <command> [(<template-name> | <deployment-name> | (<configuration> [<import1>...<importN>]))]\n"
	fmt.Fprintf(os.Stderr, message, os.Args[0])
	fmt.Fprintln(os.Stderr, "Commands:")
	for _, command := range commands {
		fmt.Fprintln(os.Stderr, command)
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Flags:")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "--stdin requires a file name and either the file contents or a tar archive containing the named file.")
	fmt.Fprintln(os.Stderr, "        a tar archive may include any additional files referenced directly or indirectly by the named file.")
	os.Exit(0)
}

func getCredential() *common.RegistryCredential {
	*apitoken = strings.TrimSpace(*apitoken)
	if *apitoken == "" {
		*apitoken = strings.TrimSpace(os.Getenv("GITHUB_API_TOKEN"))
	}

	if *apitoken != "" {
		return &common.RegistryCredential{
			APIToken: common.APITokenCredential(*apitoken),
		}
	}

	*username = strings.TrimSpace(*username)
	if *username == "" {
		*username = strings.TrimSpace(os.Getenv("GITHUB_USERNAME"))
	}

	if *username != "" {
		*password = strings.TrimSpace(*password)
		if *password == "" {
			*password = strings.TrimSpace(os.Getenv("GITHUB_PASSWORD"))
		}

		return &common.RegistryCredential{
			BasicAuth: common.BasicAuthCredential{
				Username: *username,
				Password: *password,
			},
		}
	}

	if *serviceaccount != "" {
		b, err := ioutil.ReadFile(*serviceaccount)
		if err != nil {
			log.Fatalf("Unable to read service account file: %v", err)
		}
		return &common.RegistryCredential{
			ServiceAccount: common.JWTTokenCredential(string(b)),
		}
	}
	return nil
}

func init() {
	flag.Usage = usage
}

func getRegistry() ([]byte, error) {
	if *registryfile == "" {
		log.Fatalf("No registryfile specified (-registryfile)")
	}
	return ioutil.ReadFile(*registryfile)
}

func main() {
	defer func() {
		result := recover()
		if result != nil {
			log.Fatalln(result)
		}
	}()

	execute()
	os.Exit(0)
}

func execute() {
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		usage()
	}

	switch args[0] {
	case "templates":
		path := fmt.Sprintf("registries/%s/types", *templateRegistry)
		if *regexString != "" {
			path += fmt.Sprintf("?%s", url.QueryEscape(*regexString))
		}

		callService(path, "GET", "list templates", nil)
	case "describe":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "No type name or URL supplied")
			os.Exit(1)
		}

		path := fmt.Sprintf("types/%s/metadata", url.QueryEscape(args[1]))
		callService(path, "GET", "get metadata for type", nil)
	case "expand":
		template := loadTemplate(args)
		callService("expand", "POST", "expand configuration", marshalTemplate(template))
	case "deploy":
		template := loadTemplate(args)
		action := fmt.Sprintf("deploy configuration named %s", template.Name)
		callService("deployments", "POST", action, marshalTemplate(template))
	case "list":
		callService("deployments", "GET", "list deployments", nil)
	case "getcredential":
		path := fmt.Sprintf("credentials/%s", args[1])
		callService(path, "GET", "get credential", nil)
	case "setcredential":
		c := getCredential()
		if c == nil {
			panic(fmt.Errorf("Failed to create a credential from flags/arguments"))
		}
		y, err := yaml.Marshal(c)
		if err != nil {
			panic(fmt.Errorf("Failed to serialize credential: %#v : %s", c, err))
		}

		path := fmt.Sprintf("credentials/%s", args[1])
		callService(path, "POST", "set credential", ioutil.NopCloser(bytes.NewReader(y)))
	case "createregistry":
		reg, err := getRegistry()
		if err != nil {
			panic(fmt.Errorf("Failed to create a registry from arguments: %#v", err))
		}
		path := fmt.Sprintf("registries/%s", *templateRegistry)
		callService(path, "POST", "create registry", ioutil.NopCloser(bytes.NewReader(reg)))
	case "get":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "No deployment name supplied")
			os.Exit(1)
		}

		path := fmt.Sprintf("deployments/%s", url.QueryEscape(args[1]))
		action := fmt.Sprintf("get deployment named %s", args[1])
		callService(path, "GET", action, nil)
	case "manifest":
		msg := "Must specify manifest in the form <deployment> <manifest> or just <deployment> to list."
		if len(args) < 2 || len(args) > 3 {
			fmt.Fprintln(os.Stderr, msg)
			os.Exit(1)
		}

		path := fmt.Sprintf("deployments/%s/manifests", url.QueryEscape(args[1]))
		if len(args) > 2 {
			path = path + fmt.Sprintf("/%s", url.QueryEscape(args[2]))
		}

		action := fmt.Sprintf("get manifest %s", args[1])
		callService(path, "GET", action, nil)
	case "delete":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "No deployment name supplied")
			os.Exit(1)
		}

		path := fmt.Sprintf("deployments/%s", url.QueryEscape(args[1]))
		action := fmt.Sprintf("delete deployment named %s", args[1])
		callService(path, "DELETE", action, nil)
	case "update":
		template := loadTemplate(args)
		path := fmt.Sprintf("deployments/%s", url.QueryEscape(template.Name))
		action := fmt.Sprintf("delete deployment named %s", template.Name)
		callService(path, "PUT", action, marshalTemplate(template))
	case "deployed-types":
		callService("types", "GET", "list deployed types", nil)
	case "deployed-instances":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "No type name supplied")
			os.Exit(1)
		}

		tURL := args[1]
		path := fmt.Sprintf("types/%s/instances", url.QueryEscape(tURL))
		action := fmt.Sprintf("list deployed instances of type %s", tURL)
		callService(path, "GET", action, nil)
	case "registries":
		callService("registries", "GET", "list registries", nil)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command '%s' for 'dm'\n", args[0])
		fmt.Fprintln(os.Stderr, "Run 'dm --help' for usage")
		os.Exit(1)
	}
}

func callService(path, method, action string, reader io.ReadCloser) {
	var URL *url.URL
	URL, err := url.Parse(*service)
	if err != nil {
		panic(fmt.Errorf("cannot parse url (%s): %s\n", *service, err))
	}

	URL.Path = strings.TrimRight(URL.Path, "/") + "/" + strings.TrimLeft(path, "/")
	resp := callHTTP(URL.String(), method, action, reader)
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

func callHTTP(path, method, action string, reader io.ReadCloser) string {
	request, err := http.NewRequest(method, path, reader)
	request.Header.Add("Content-Type", "application/json")

	client := http.Client{
		Timeout: time.Duration(time.Duration(*timeout) * time.Second),
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

func loadTemplate(args []string) *common.Template {
	var template *common.Template
	var err error
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "No template name or configuration(s) supplied")
		os.Exit(1)
	}

	if *stdin {
		if len(args) < 2 {
			os.Exit(1)
		}

		input, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			panic(err)
		}

		r := bytes.NewReader(input)
		template, err = util.NewTemplateFromArchive(args[1], r, args[2:])
		if err != nil {
			if err != tar.ErrHeader {
				panic(err)
			}

			r := bytes.NewReader(input)
			template, err = util.NewTemplateFromReader(args[1], r, args[2:])
			if err != nil {
				panic(fmt.Errorf("cannot create configuration from supplied arguments: %s\n", err))
			}
		}
	} else {
		// See if the first argument is a local file. It could either be a type, or it could be a configuration. If
		// it's a local file, it's configuration.
		if _, err := os.Stat(args[1]); err == nil {
			if len(args) > 2 {
				template, err = util.NewTemplateFromFileNames(args[1], args[2:])
			} else {
				template, err = util.NewTemplateFromRootTemplate(args[1])
			}
		} else {
			template = buildTemplateFromType(args[1])
		}

		if err != nil {
			panic(fmt.Errorf("cannot create configuration from supplied arguments: %s\n", err))
		}
	}

	// Override name if set from flags.
	if *deploymentName != "" {
		template.Name = *deploymentName
	}

	return template
}

func buildTemplateFromType(t string) *common.Template {
	props := make(map[string]interface{})
	if *properties != "" {
		plist := strings.Split(*properties, ",")
		for _, p := range plist {
			ppair := strings.Split(p, "=")
			if len(ppair) != 2 {
				panic(fmt.Errorf("--properties must be in the form \"p1=v1,p2=v2,...\": %s\n", p))
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

	// Name the deployment after the type name.
	template, err := util.NewTemplateFromType(t, t, props)
	if err != nil {
		panic(fmt.Errorf("cannot create configuration from type (%s): %s\n", t, err))
	}

	return template
}

func marshalTemplate(template *common.Template) io.ReadCloser {
	j, err := json.Marshal(template)
	if err != nil {
		panic(fmt.Errorf("cannot deploy configuration %s: %s\n", template.Name, err))
	}

	return ioutil.NopCloser(bytes.NewReader(j))
}

func getRandomName() string {
	return fmt.Sprintf("manifest-%d", time.Now().UTC().UnixNano())
}
