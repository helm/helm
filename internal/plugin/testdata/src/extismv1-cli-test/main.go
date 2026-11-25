package main

import (
	"fmt"
	"strings"

	pdk "github.com/extism/go-pdk"
)

type InputMessageCLIV1 struct {
	ExtraArgs []string `json:"extraArgs"`
}

type OutputMessageCLIV1 struct {
	Data string `json:"data"`
}

type ConfigCLIV1 struct{}

func RunCLIPlugin() error {
	var input InputMessageCLIV1
	if err := pdk.InputJSON(&input); err != nil {
		return fmt.Errorf("failed to parse input json: %w", err)
	}

	pdk.Log(pdk.LogDebug, fmt.Sprintf("Received args: %v", input.ExtraArgs))

	return pdk.OutputJSON(OutputMessageCLIV1{
		Data: "cli plugin output: " + strings.Join(input.ExtraArgs, " ") + "\n",
	})
}

//go:wasmexport helm_plugin_main
func HelmPlugin() uint32 {
	if err := RunCLIPlugin(); err != nil {
		pdk.Log(pdk.LogError, err.Error())
		pdk.SetError(err)
		return 1
	}

	return 0
}

func main() {}
