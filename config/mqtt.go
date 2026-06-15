package config

import (
	"fmt"
	"log"
	"time"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var MQTTClient mqtt.Client
var OnConnectCallback func(mqtt.Client)

func InitMQTT() {
	opts := mqtt.NewClientOptions()
	opts.AddBroker("tcp://broker.emqx.io:1883")
	// Menggunakan ClientID unik berbasis timestamp agar backend lokal dan produksi tidak saling menendang
	opts.SetClientID(fmt.Sprintf("Tempura_Backend_Subscriber_%d", time.Now().UnixNano()))
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)

	opts.SetOnConnectHandler(func(c mqtt.Client) {
		fmt.Println("Koneksi MQTT terhubung/dipulihkan!")
		if OnConnectCallback != nil {
			OnConnectCallback(c)
		}
	})

	MQTTClient = mqtt.NewClient(opts)
	if token := MQTTClient.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("Error connecting to MQTT: %v", token.Error())
	}
}
