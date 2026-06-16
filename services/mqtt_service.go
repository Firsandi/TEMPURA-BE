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

var (
	lastTempAlertTime time.Time
	lastHumAlertTime  time.Time
	alertCooldown     = 30 * time.Minute
)

type SensorPayload struct {
	Temp      float64 `json:"temp"`
	Hum       float64 `json:"hum"`
	Soil      int     `json:"soil"`
	RawSoil   int     `json:"raw_soil"`
	RelayFan  bool    `json:"relay_fan"`
	RelayPump bool    `json:"relay_pump"`
	RelayBulb bool    `json:"relay_bulb"`
	Health    string  `json:"health"`
}

func RegisterMQTTCallback() {
	config.OnConnectCallback = func(c mqtt.Client) {
		topic := "tempura/sensor/data"
		token := c.Subscribe(topic, 1, handleSensorData)
		token.Wait()
		if token.Error() != nil {
			log.Printf("Gagal subscribe ke topik %s: %v", topic, token.Error())
		} else {
			fmt.Printf("Berhasil subscribe ke topik: %s\n", topic)
		}
	}
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
		} else {
			fmt.Printf("Saved global heartbeat data\n")
		}
	}


	// 2.5 Check Alerts
	var settings models.SystemSetting
	if err := config.DB.First(&settings).Error; err == nil {
		now := time.Now()
		// Temp Alert
		if payload.Temp < settings.TargetTemp || payload.Temp > settings.MaxTemp {
			if now.Sub(lastTempAlertTime) > alertCooldown {
				title := "Peringatan Suhu Abnormal!"
				body := fmt.Sprintf("Suhu saat ini %.1f C (Batas: %.1f C - %.1f C)", payload.Temp, settings.TargetTemp, settings.MaxTemp)
				go SendAlertNotification(title, body, "temperature_alert")
				lastTempAlertTime = now
			}
		}
		// Hum Alert
		if payload.Hum < settings.MinHumidity || payload.Hum > settings.MaxHumidity {
			if now.Sub(lastHumAlertTime) > alertCooldown {
				title := "Peringatan Kelembaban Abnormal!"
				body := fmt.Sprintf("Kelembaban saat ini %.1f%% (Batas: %.1f%% - %.1f%%)", payload.Hum, settings.MinHumidity, settings.MaxHumidity)
				go SendAlertNotification(title, body, "humidity_alert")
				lastHumAlertTime = now
			}
		}
	}

	// 3. Auto Control Logic (only if batch is active)
	if batchID != nil {
		runAutoControl(client, payload, batch)
	}
}

// runAutoControl executes automatic actuator control based on temperature and humidity.
// Logika kontrol fungsi ganda:
//   - Lampu: pemanas (suhu rendah) + pengusir lembab (kelembaban tinggi)
//   - Kipas: pendingin (suhu tinggi) + pengusir lembab (kelembaban tinggi)
//   - Mist Maker: penambah kelembaban (kelembaban rendah)
// Soil moisture hanya untuk deteksi kematangan (fail-safe auto-harvest).
func runAutoControl(client mqtt.Client, payload SensorPayload, batch models.BatchProduksi) {
	// Check if system is in auto mode
	var settings models.SystemSetting
	if err := config.DB.First(&settings).Error; err != nil {
		log.Printf("Auto-control: Could not fetch settings, skipping: %v", err)
		return
	}

	if settings.Mode != "auto" {
		return
	}

	log.Printf("AUTO-CONTROL: Suhu=%.1f°C Kelembaban=%.1f%% Soil=%d%%", payload.Temp, payload.Hum, payload.Soil)

	topic := "tempura/device/control"

	// --- FAIL-SAFE: Tempe matang (soil < 30%) → matikan semua ---
	if payload.Soil < settings.TargetMoisture {
		publishControl(client, topic, "fan_off")
		publishControl(client, topic, "mist_off")
		publishControl(client, topic, "bulb_off")
		log.Printf("AUTO-CONTROL FAIL-SAFE: Soil %d%% < %d%% → Semua alat OFF (tempe matang)", payload.Soil, settings.TargetMoisture)
		// Cek auto-harvest
		checkAutoHarvest(client, payload, batch, topic)
		return
	}

	// --- KONTROL LAMPU (Fungsi Ganda: nyala jika dingin ATAU terlalu lembab) ---
	if payload.Temp < settings.TargetTemp || payload.Hum > settings.MaxHumidity {
		publishControl(client, topic, "bulb_on")
		log.Printf("AUTO-CONTROL: Lampu ON (Suhu %.1f°C < %.1f°C atau Kelembaban %.1f%% > %.1f%%)", payload.Temp, settings.TargetTemp, payload.Hum, settings.MaxHumidity)
	} else {
		publishControl(client, topic, "bulb_off")
		log.Printf("AUTO-CONTROL: Lampu OFF (kondisi aman)")
	}

	// --- KONTROL KIPAS (Fungsi Ganda: nyala jika kepanasan ATAU terlalu lembab) ---
	if payload.Temp > settings.MaxTemp || payload.Hum > settings.MaxHumidity {
		publishControl(client, topic, "fan_on")
		log.Printf("AUTO-CONTROL: Kipas ON (Suhu %.1f°C > %.1f°C atau Kelembaban %.1f%% > %.1f%%)", payload.Temp, settings.MaxTemp, payload.Hum, settings.MaxHumidity)
	} else {
		publishControl(client, topic, "fan_off")
		log.Printf("AUTO-CONTROL: Kipas OFF (kondisi aman)")
	}

	// --- KONTROL MIST MAKER (Hanya nyala jika ruangan terlalu kering) ---
	if payload.Hum < settings.MinHumidity {
		publishControl(client, topic, "mist_on")
		log.Printf("AUTO-CONTROL: Mist Maker ON (Kelembaban %.1f%% < %.1f%%)", payload.Hum, settings.MinHumidity)
	} else {
		publishControl(client, topic, "mist_off")
		log.Printf("AUTO-CONTROL: Mist Maker OFF (kelembaban cukup)")
	}

	// --- AUTO-HARVEST: Soil < threshold konstan selama 30 menit ---
	checkAutoHarvest(client, payload, batch, topic)
}

// checkAutoHarvest checks if soil moisture has been below 30% consistently for 30 minutes.
func checkAutoHarvest(client mqtt.Client, payload SensorPayload, batch models.BatchProduksi, topic string) {
	if payload.Soil >= 30 {
		return // Belum fase matang
	}

	// Query data sensor terakhir dalam 30 menit, cek apakah semua soil < 30%
	thirtyMinAgo := time.Now().Add(-30 * time.Minute)
	var count int64
	config.DB.Model(&models.SensorData{}).
		Where("timestamp >= ? AND soil_moisture < 30", thirtyMinAgo).
		Count(&count)

	// Minimal 6 data points dalam 30 menit (interval 5 detik = ~360 data, tapi kita toleran minimal 6)
	if count < 6 {
		log.Printf("AUTO-HARVEST: Soil %d%% < 30%%, tapi baru %d data point (butuh min 6). Menunggu...", payload.Soil, count)
		return
	}

	// Cek apakah ada data yang >= 30% dalam 30 menit terakhir
	var aboveCount int64
	config.DB.Model(&models.SensorData{}).
		Where("timestamp >= ? AND soil_moisture >= 30", thirtyMinAgo).
		Count(&aboveCount)

	if aboveCount > 0 {
		log.Printf("AUTO-HARVEST: Masih ada %d data dengan soil >= 30%% dalam 30 menit terakhir. Belum stabil.", aboveCount)
		return
	}

	// Semua data dalam 30 menit terakhir konsisten < 30%!
	log.Printf("AUTO-HARVEST: Batch #%d - Soil < 30%% konstan selama 30 menit! Memulai panen otomatis.", batch.BatchID)

	// Matikan semua aktuator
	publishControl(client, topic, "fan_off")
	publishControl(client, topic, "mist_off")
	publishControl(client, topic, "bulb_off")

	// Complete the batch
	now := time.Now()
	config.DB.Model(&models.BatchProduksi{}).Where("batch_id = ?", batch.BatchID).Updates(map[string]interface{}{
		"status_batch":  "completed",
		"end_timestamp": &now,
	})
	config.DB.Model(&models.ProductionHistory{}).Where("batch_id = ? AND end_time IS NULL", batch.BatchID).Updates(map[string]interface{}{
		"end_time": &now,
		"status":   "Matang Sempurna (otomatis)",
	})

	log.Printf("AUTO-HARVEST: Batch #%d '%s' berhasil di-harvest otomatis!", batch.BatchID, batch.NamaBatch)

	// Send FCM notification
	go SendHarvestNotification(batch.NamaBatch)
}

// publishControl publishes a control command to MQTT.
func publishControl(client mqtt.Client, topic, command string) {
	token := client.Publish(topic, 1, false, command)
	token.Wait()
	if token.Error() != nil {
		log.Printf("Error publishing control command '%s': %v", command, token.Error())
	}
}
