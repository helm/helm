package kube

import (
	"flag"
	"fmt"
	"os"
)

func init() {
	if level := os.Getenv("KUBE_LOG_LEVEL"); level != "" {
		flag.Set("vmodule", fmt.Sprintf("loader=%s,round_trippers=%s,request=%s", level, level, level))
		flag.Set("logtostderr", "true")
	}
}
