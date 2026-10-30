package main

import (
	"fmt"

	pdk "github.com/extism/go-pdk"
)

type inputMessagePostRendererV1 struct {
	Manifests string   `json:"manifests"`
	ExtraArgs []string `json:"extraArgs"`
}

type outputMessagePostRendererV1 struct {
	Manifests string `json:"manifests"`
}

func runPostRendererPlugin() error {
	var input inputMessagePostRendererV1
	if err := pdk.InputJSON(&input); err != nil {
		return fmt.Errorf("failed to parse input json: %w", err)
	}

	pdk.Log(pdk.LogDebug, fmt.Sprintf("received manifests (%d bytes), extraArgs: %v", len(input.Manifests), input.ExtraArgs))

	output := outputMessagePostRendererV1{
		Manifests: "# post-rendered\n" + input.Manifests,
	}

	if err := pdk.OutputJSON(output); err != nil {
		return fmt.Errorf("failed to write output json: %w", err)
	}

	return nil
}

//go:wasmexport helm_plugin_main
func HelmPlugin() uint32 {
	if err := runPostRendererPlugin(); err != nil {
		pdk.Log(pdk.LogError, err.Error())
		pdk.SetError(err)
		return 1
	}

	return 0
}

func main() {}
