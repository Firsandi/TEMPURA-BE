package main

import (
	"fmt"
	"tempura-backend/config"
	"tempura-backend/models"
	"time"
)

func main() {
	config.ConnectDatabase()
	var latest models.SensorData
	config.DB.Order("timestamp desc").First(&latest)
	fmt.Printf("Latest sensor: ID=%d, Temp=%f, Soil=%d, Timestamp=%s, Time since=%s\n", 
		latest.SensorDataID, latest.Suhu, latest.SoilMoisture, latest.Timestamp, time.Since(latest.Timestamp))
}
