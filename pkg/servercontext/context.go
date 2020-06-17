package servercontext

import (
	"fmt"
	"log"
	"os"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
)

var app Application

type Application struct {
	Config       *cli.EnvSettings
	ActionConfig *action.Configuration
}

func App() *Application {
	return &app
}

func NewApp() *Application {
	app.Config = envSettings()
	app.ActionConfig = boostrapActionConfig()
	return &app
}

func envSettings() *cli.EnvSettings {
	envSettings := cli.New()
	for k, v := range envSettings.EnvVars() {
		fmt.Println(k, v)
	}
	return envSettings
}

func debug(format string, v ...interface{}) {
	format = fmt.Sprintf("[debug] %s\n", format)
	log.Output(2, fmt.Sprintf(format, v...))
}

func boostrapActionConfig() *action.Configuration {
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(app.Config.RESTClientGetter(), app.Config.Namespace(), os.Getenv("HELM_DRIVER"), debug); err != nil {
		log.Fatalf("error getting configuration: %v", err)
		return nil
	}
	return actionConfig
}
