package main

import (
	"fmt"
	"log"
	"tempura-backend/config"
	"tempura-backend/models"
)

func main() {
	config.ConnectDatabase()
	var data []models.SensorData
	err := config.DB.Order("timestamp desc").Limit(5).Find(&data).Error
	if err != nil {
		log.Fatalf("Error querying DB: %v", err)
	}

	fmt.Println("Latest 5 sensor data in DB:")
	for _, d := range data {
		fmt.Printf("ID: %d, Temp: %.2f, Hum: %.2f, Soil: %d, RelayFan: %t, RelayPump: %t, RelayBulb: %t, Health: %s, Timestamp: %s\n",
			d.SensorDataID, d.Suhu, d.Kelembaban, d.SoilMoisture, d.RelayFan, d.RelayPump, d.RelayBulb, d.Health, d.Timestamp)
	}
}
