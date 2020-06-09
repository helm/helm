package main

import (
	"fmt"
	"log"
	"os"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/gates"
	"helm.sh/helm/v3/pkg/http"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	settings = cli.New()
)

func debug(format string, v ...interface{}) {
	format = fmt.Sprintf("[debug] %s\n", format)
	log.Output(2, fmt.Sprintf(format, v...))
}

const FeatureGateOCI = gates.Gate("HELM_EXPERIMENTAL_OCI")

// Input: repositories.yaml, repositories cache (optional), chart location

func main() {
	actionConfig := new(action.Configuration)
	for k, v := range settings.EnvVars() {
		fmt.Println(k, v)
	}
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), debug); err != nil {
		log.Fatalf("error getting configuration: %v", err)
		return
	}
	if _, err := actionConfig.KubernetesClientSet(); err != nil {
		log.Fatalf("error initilizing kubernetes client configuration: %v", err)
		return
	}

	listReleases(actionConfig)
	helmRepoUpdate()
	// this has to be added in repositories: https://charts.bitnami.com/bitnami
	installRelease(actionConfig, "bitnami/redis", "bitnami-redis-2")
}

func listReleases(cfg *action.Configuration) {
	releases, err := http.List(cfg)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(releases))
}

func helmRepoUpdate() {
	err := http.HelmRepoUpdate()
	if err != nil {
		panic(err)
	}
}

func installRelease(cfg *action.Configuration, chartPath string, releaseName string) {
	fmt.Printf("installing chart: %s. release name: %s ", chartPath, releaseName)
	_, err := http.Install(cfg, chartPath, releaseName)
	if err != nil {
		fmt.Println("error installing chart", err)
		return
	}
	fmt.Printf("installed chart: %s release: %s successfully", chartPath, releaseName)
}
