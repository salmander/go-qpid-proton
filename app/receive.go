package main

import (
	"fmt"
	"log"
	"sync"

	"qpid.apache.org/amqp"
	"qpid.apache.org/electron"
)

const QUEUE_ORDER_CREATED = "/test1"
const NUMBER_OF_MESSAGES = 100

var wait sync.WaitGroup
var container electron.Container
var connections chan electron.Connection
var sentChan chan electron.Outcome
var messages chan amqp.Message

func Debugf(format string, data ...interface{}) {
	log.Printf(format, data...)
}
func main() {
	messages = make(chan amqp.Message) // Channel for messages from goroutines to main()
	defer close(messages)
	urlStr := "amqp://queue" + QUEUE_ORDER_CREATED

	var wait sync.WaitGroup // Used by main() to wait for all goroutines to end.
	wait.Add(1)             // Wait for one goroutine per URL.

	container = electron.NewContainer("ThisIsTheClientID")
	connections = make(chan electron.Connection, 1) // Connections to close on exit

	// Start a goroutine to for each URL to receive messages and send them to the messages channel.
	// main() receives and prints them.
	Debugf("Connecting to %s\n", urlStr)
	go Consume(urlStr) // Start the goroutine

	// All goroutines are started, we are receiving messages.
	fmt.Printf("Listening on %d connections\n", 1)

	// print each message until the count is exceeded.
	for i := uint64(0); i < NUMBER_OF_MESSAGES; i++ {
		m := <-messages
		Debugf("%v\n", m.Body())
	}
	fmt.Printf("Received %d messages\n", NUMBER_OF_MESSAGES)

	// Close all connections, this will interrupt goroutines blocked in Receiver.Receive()
	// with electron.Closed.
	for i := 0; i < NUMBER_OF_MESSAGES; i++ {
		c := <-connections
		Debugf("close %s", c)
		c.Close(nil)
	}
	wait.Wait() // Wait for all goroutines to finish.
}

func Consume(urlStr string) {
	defer wait.Done() // Notify main() when this goroutine is done.
	var err error
	if url, err := amqp.ParseURL(urlStr); err == nil {
		if c, err := container.Dial("tcp", url.Host); err == nil {
			connections <- c // Save connection so we can Close() when main() ends
			if r, err := c.Receiver(electron.Source("topic://" + url.Path)); err == nil {
				// Loop receiving messages and sending them to the main() goroutine
				for {
					if rm, err := r.Receive(); err == nil {
						rm.Accept()
						messages <- rm.Message
					} else if err == electron.Closed {
						return
					} else {
						log.Fatal("receive error %v: %v", urlStr, err)
					}
				}
			}
		}
	}
	if err != nil {
		log.Fatal(err)
	}
}
