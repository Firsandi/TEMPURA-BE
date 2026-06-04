package services

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
	"tempura-backend/config"
	"tempura-backend/models"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type SensorPayload struct {
	Temp      float64 `json:"temp"`
	Hum       float64 `json:"hum"`
	Soil      int     `json:"soil"`
	RelayFan  bool    `json:"relay_fan"`
	RelayPump bool    `json:"relay_pump"`
	RelayBulb bool    `json:"relay_bulb"`
	Health    string  `json:"health"`
}

func StartMQTTSubscription() {
	topic := "tempura/sensor/data"
	token := config.MQTTClient.Subscribe(topic, 1, handleSensorData)
	token.Wait()
	fmt.Printf("Subscribed to topic: %s\n", topic)
}

func handleSensorData(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())

	var payload SensorPayload
	err := json.Unmarshal(msg.Payload(), &payload)
	if err != nil {
		log.Printf("Error unmarshaling MQTT payload: %v", err)
		return
	}

	// 1. Get active batch (optional)
	var batch models.BatchProduksi
	var batchID *uint
	var historyID *uint
	result := config.DB.Where("status_batch = ? AND is_deleted = false", "active").Order("created_at desc").First(&batch)

	if result.Error == nil {
		batchID = &batch.BatchID
		
		// Get current history record
		var history models.ProductionHistory
		if err := config.DB.Where("batch_id = ? AND end_time IS NULL", batch.BatchID).First(&history).Error; err == nil {
			historyID = &history.HistoryID
		}
	}

	// 2. Save to database (Always save, even without batch)
	sensorData := models.SensorData{
		HistoryID:    historyID,
		Suhu:         payload.Temp,
		Kelembaban:   payload.Hum,
		SoilMoisture: payload.Soil,
		RelayFan:     payload.RelayFan,
		RelayPump:    payload.RelayPump,
		RelayBulb:    payload.RelayBulb,
		Health:       payload.Health,
	}

	dbResult := config.DB.Create(&sensorData)
	if dbResult.Error != nil {
		log.Printf("Error saving sensor data to DB: %v", dbResult.Error)
	} else {
		if batchID != nil {
			fmt.Printf("Saved sensor data for Batch #%d Run #%v\n", *batchID, historyID)
			
			// 3. Auto-Harvest Logic
			// If Soil Moisture < 200 (Siap Panen)
			if payload.Soil > 0 && payload.Soil < 200 {
				log.Printf("AUTO-HARVEST: Batch #%d reached harvest threshold!", *batchID)
				now := time.Now()
				config.DB.Model(&models.BatchProduksi{}).Where("batch_id = ?", *batchID).Update("status_batch", "completed")
				config.DB.Model(&models.ProductionHistory{}).Where("batch_id = ? AND end_time IS NULL", *batchID).Updates(map[string]interface{}{
					"end_time": &now,
					"status":   "Berhasil Fermentasi (normal)",
				})
			}
		} else {
			fmt.Printf("Saved global heartbeat data\n")
		}
	}
}
