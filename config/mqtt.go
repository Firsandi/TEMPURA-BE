package config

import (
	"fmt"
	"log"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var MQTTClient mqtt.Client

func InitMQTT() {
	opts := mqtt.NewClientOptions()
	opts.AddBroker("tcp://broker.emqx.io:1883")
	opts.SetClientID("Tempura_Backend_Subscriber")
	opts.SetCleanSession(true)

	MQTTClient = mqtt.NewClient(opts)
	if token := MQTTClient.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("Error connecting to MQTT: %v", token.Error())
	}

	fmt.Println("Connected to MQTT Broker (broker.emqx.io)")
}
