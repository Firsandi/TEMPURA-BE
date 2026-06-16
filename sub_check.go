package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	opts := mqtt.NewClientOptions()
	opts.AddBroker("tcp://broker.emqx.io:1883")
	opts.SetClientID("Tempura_Test_Subscriber")

	opts.SetOnConnectHandler(func(c mqtt.Client) {
		fmt.Println("Connected to broker")
		c.Subscribe("tempura/sensor/data", 0, func(client mqtt.Client, msg mqtt.Message) {
			fmt.Printf("Received: %s\n", msg.Payload())
		})
	})

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		fmt.Printf("Error: %v\n", token.Error())
		return
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	<-sigc
}
