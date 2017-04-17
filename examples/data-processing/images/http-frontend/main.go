package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/streadway/amqp"
)

var channel *amqp.Channel
var queue amqp.Queue

func SendMessageHandler(rw http.ResponseWriter, req *http.Request) {
	params := []string{"param1", "param2", "param3"}
	data := map[string]string{}
	get_variables := req.URL.Query()
	for _, param := range params {
		data[param] = get_variables.Get(param)
	}
	json_data, err := json.Marshal(data)
	HandleError(err, "Error while converting query data to JSON")
	err = channel.Publish("", queue.Name, false, false, amqp.Publishing{DeliveryMode: amqp.Persistent, ContentType: "application/json", Body: []byte(json_data)})
	HandleError(err, "Failed to publish a message")
	fmt.Fprintln(rw, "Success!")
}

func HandleError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}

func main() {
	broker := os.Getenv("RABBITMQ_SERVER")
	username := os.Getenv("RABBITMQ_USERNAME")
	password := os.Getenv("RABBITMQ_PASSWORD")
	conn, err := amqp.Dial("amqp://" + username + ":" + password + "@" + broker + ":5672/")
	HandleError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()
	channel, err = conn.Channel()
	HandleError(err, "Failed to open a channel")
	defer channel.Close()
	queue, err = channel.QueueDeclare("messages", true, false, false, false, nil)
	HandleError(err, "Failed to declare a queue")

	r := mux.NewRouter()
	r.Path("/send_data/").Methods("GET").HandlerFunc(SendMessageHandler)

	n := negroni.Classic()
	n.UseHandler(r)
	n.Run(":80")
}
