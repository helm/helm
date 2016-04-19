package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	RootCommand.AddCommand(fetchCmd)
}

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Download a chart from a repository and unpack it in local directory.",
	Long:  "",
	RunE:  Fetch,
}

func Fetch(cmd *cobra.Command, args []string) error {
	// parse args
	// get download url
	// call download url
	out, err := os.Create("nginx-2.0.0.tgz")
	if err != nil {
		return err
	}
	defer out.Close()
	resp, err := http.Get("http://localhost:8879/charts/nginx-2.0.0.tgz")
	fmt.Println("after req")
	// unpack file
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	return nil
}
