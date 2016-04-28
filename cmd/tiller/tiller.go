package main

import (
	"fmt"
	"net"
	"os"

	"github.com/deis/tiller/cmd/tiller/environment"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

// rootServer is the root gRPC server.
//
// Each gRPC service registers itself to this server during init().
var rootServer = grpc.NewServer()
var env = environment.New()

const globalUsage = `The Kubernetes Helm server.

Tiller is the server for Helm. It provides in-cluster resource management.

By default, Tiller listens for gRPC connections on port 44134.
`

var rootCommand = &cobra.Command{
	Use:   "tiller",
	Short: "The Kubernetes Helm server.",
	Long:  globalUsage,
	Run:   start,
}

func main() {
	rootCommand.Execute()
}

func start(c *cobra.Command, args []string) {
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
