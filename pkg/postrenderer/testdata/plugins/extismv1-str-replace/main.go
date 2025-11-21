package main

import (
	"fmt"
	"strings"

	pdk "github.com/extism/go-pdk"

	"helm.sh/helm/v4/internal/plugin/schema"
)

func RunPlugin() error {
	var input schema.InputMessagePostRendererV1

	if err := pdk.InputJSON(&input); err != nil {
		return fmt.Errorf("failed to parse input json: %w", err)
	}

	replacement := "BARTEST"

	if len(input.ExtraArgs) > 0 {
		replacement = strings.Join(input.ExtraArgs, " ")
	}

	updatedManifests := strings.ReplaceAll(input.Manifests, "FOOTEST", replacement)

	result := schema.OutputMessagePostRendererV1{
		Manifests: updatedManifests,
	}

	if err := pdk.OutputJSON(&result); err != nil {
		return fmt.Errorf("failed to write output json: %w", err)
	}

	return nil
}

//go:wasmexport helm_plugin_main
func HelmChartRenderer() uint64 {
	pdk.Log(pdk.LogDebug, "running postrenderer-v1-extism plugin")

	if err := RunPlugin(); err != nil {
		pdk.Log(pdk.LogError, err.Error())
		pdk.SetError(err)
		return 1
	}

	return 0
}

func main() {}
