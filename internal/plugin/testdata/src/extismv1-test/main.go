package main

import (
	_ "embed"
	"fmt"

	pdk "github.com/extism/go-pdk"
)

type InputMessageTestV1 struct {
	Name string
}

type OutputMessageTestV1 struct {
	Greeting string
}

type ConfigTestV1 struct{}

func runGetterPluginImpl(input InputMessageTestV1) (*OutputMessageTestV1, error) {
	name := input.Name
	return &OutputMessageTestV1{
		Greeting: fmt.Sprintf("Hello, %s! (%d)", name, len(name)),
	}, nil
}

func RunGetterPlugin() error {
	var input InputMessageTestV1
	if err := pdk.InputJSON(&input); err != nil {
		return fmt.Errorf("failed to parse input json: %w", err)
	}

	pdk.Log(pdk.LogDebug, fmt.Sprintf("Received input: %+v", input))
	output, err := runGetterPluginImpl(input)
	if err != nil {
		pdk.Log(pdk.LogError, fmt.Sprintf("failed: %s", err.Error()))
		return err
	}

	pdk.Log(pdk.LogDebug, fmt.Sprintf("Sending output: %+v", output))
	if err := pdk.OutputJSON(output); err != nil {
		return fmt.Errorf("failed to write output json: %w", err)
	}

	return nil
}

//go:wasmexport helm_plugin_main
func HelmPlugin() uint32 {
	pdk.Log(pdk.LogDebug, "running example-extism-getter plugin")

	if err := RunGetterPlugin(); err != nil {
		pdk.Log(pdk.LogError, err.Error())
		pdk.SetError(err)
		return 1
	}

	return 0
}

func main() {}
