/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main // import "k8s.io/helm/cmd/tiller"

import (
	"net/http"

	_ "net/http/pprof"

	log "github.com/Sirupsen/logrus"
	"google.golang.org/grpc"
)

func startTracing(addr string) {
	grpc.EnableTracing = true

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(traceIndexHTML))
	})

	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			logger.WithFields(log.Fields{
				"_module":  "trace",
				"_context": "startTracing",
				"error":    err,
			}).Error("Tracing error")
		}
	}()
	logger.WithFields(log.Fields{
		"_module":  "trace",
		"_context": "startTracing",
		"address":  addr,
	}).Info("Tracing server listening")
}

const traceIndexHTML = `<!DOCTYPE html>
<html>
  <body>
    <ul>
      <li><a href="/debug/requests">requests</a></li>
      <li><a href="/debug/events">events</a></li>
      <li><a href="/debug/pprof">pprof</a></li>
      <li><a href="/debug/vars">vars</a></li>
    </ul>
  </body>
</html>
`
