package config

import (
	"fmt"
	"log"
	"os"
	"time"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var MQTTClient mqtt.Client
var OnConnectCallback func(mqtt.Client)

func InitMQTT() {
	opts := mqtt.NewClientOptions()
	
	broker := os.Getenv("MQTT_BROKER")
	if broker == "" {
		broker = "tcp://broker.emqx.io:1883" // Fallback
	}
	opts.AddBroker(broker)

	username := os.Getenv("MQTT_USERNAME")
	password := os.Getenv("MQTT_PASSWORD")
	if username != "" && password != "" {
		opts.SetUsername(username)
		opts.SetPassword(password)
	}

	// Menggunakan ClientID unik berbasis timestamp agar backend lokal dan produksi tidak saling menendang
	opts.SetClientID(fmt.Sprintf("Tempura_Backend_Subscriber_%d", time.Now().UnixNano()))
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	opts.SetKeepAlive(30 * time.Second)
	opts.SetPingTimeout(10 * time.Second)
	opts.SetMaxReconnectInterval(10 * time.Second)

	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		fmt.Printf("Koneksi MQTT terputus (ConnectionLostHandler): %v\n", err)
	})

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
