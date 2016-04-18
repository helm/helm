package main

import (
	"fmt"
	"net"
	"os"

	"github.com/codegangsta/cli"
	"github.com/deis/tiller/cmd/tiller/environment"
	"google.golang.org/grpc"
)

// rootServer is the root gRPC server.
//
// Each gRPC service registers itself to this server during init().
var rootServer *grpc.Server = grpc.NewServer()
var env = environment.New()

func main() {
	app := cli.NewApp()
	app.Name = "tiller"
	app.Usage = `The Helm server.`
	app.Action = start

	app.Run(os.Args)
}

func start(c *cli.Context) {
	addr := ":44134"
	lstn, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Server died: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Tiller is running on %s\n", addr)

	if err := rootServer.Serve(lstn); err != nil {
		fmt.Fprintf(os.Stderr, "Server died: %s\n", err)
		os.Exit(1)
	}
}
