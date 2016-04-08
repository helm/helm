package main

import (
	"fmt"
	"os"

	"github.com/codegangsta/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "tiller"
	app.Usage = `The Helm server.`
	app.Action = start

	app.Run(os.Args)
}

func start(c *cli.Context) {
	if err := startServer(":44134"); err != nil {
		fmt.Fprintf(os.Stderr, "Server died: %s\n", err)
		os.Exit(1)
	}
}
