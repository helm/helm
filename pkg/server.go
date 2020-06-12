package main

import (
	"fmt"
	"net/http"
)

func main() {
	startServer()
}

func startServer() {
	router := http.NewServeMux()
	router.HandleFunc("/ping", pingFunc)

	err := http.ListenAndServe(fmt.Sprintf(":%d", 8080), router)
	if err != nil {
		fmt.Println("error starting server", err)
	}
}

func pingFunc(w http.ResponseWriter, r *http.Request) {
	fmt.Println("pong")
}
