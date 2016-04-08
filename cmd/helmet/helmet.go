package main

import (
	"fmt"
	"os"

	"github.com/codegangsta/cli"
	"github.com/deis/tiller/pkg/hapi"
	ctx "golang.org/x/net/context"
	"google.golang.org/grpc"
)

func main() {
	app := cli.NewApp()
	app.Name = "helmet"
	app.Usage = "The Helm Easy Tester (HelmET)"
	app.Action = run

	app.Run(os.Args)
}

func run(c *cli.Context) {
	conn, err := grpc.Dial("localhost:44134", grpc.WithInsecure())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not connect to server: %s\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	pc := hapi.NewProbeClient(conn)

	req := &hapi.PingRequest{Name: "helmet"}
	res, err := pc.Ready(ctx.Background(), req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error pinging server: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Server is %s\n", res.Status)
}
