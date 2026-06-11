package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	opts := mqtt.NewClientOptions()
	opts.AddBroker("tcp://broker.emqx.io:1883")
	opts.SetClientID("Tempura_Debugger_Unique")

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("Error connecting to MQTT: %v", token.Error())
	}
	defer client.Disconnect(250)

	fmt.Println("Connected to MQTT Broker. Subscribing to tempura/#...")

	client.Subscribe("tempura/#", 1, func(client mqtt.Client, msg mqtt.Message) {
		fmt.Printf("[%s] %s\n", msg.Topic(), string(msg.Payload()))
	})

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	fmt.Println("Exiting...")
}
