package ping

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func Handler() http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()

		var request Req
		decoder := json.NewDecoder(req.Body)
		decoder.UseNumber()

		if err := decoder.Decode(&request); err != nil {
			fmt.Println("error in request")
			return
		}

		request.RequestID = req.Header.Get("Request-Id")

		response := Response{Status: true, Data: "pong"}
		payload, err := json.Marshal(response)
		if err != nil {
			fmt.Println("error parsing response")
			return
		}

		res.Write(payload)
	})
}
