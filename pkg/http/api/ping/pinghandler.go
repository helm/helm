package ping

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func Handler() http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {

		res.Header().Set("Content-Type", "application/json")
		defer req.Body.Close()

		var request PingReq
		decoder := json.NewDecoder(req.Body)
		decoder.UseNumber()

		if err := decoder.Decode(&request); err != nil {
			fmt.Println("error in request")
			return
		}

		request.RequestID = req.Header.Get("Request-Id")

		response := PingResponse{Status: true, Data: "pong"}
		payload, err := json.Marshal(response)
		if err != nil {
			fmt.Println("error parsing response")
			return
		}

		res.Write(payload)
	})
}
