package handlers

import (
	"net/http"
	"tempura-backend/config"
	"tempura-backend/models"

	"github.com/gin-gonic/gin"
)

func GetDashboardData(c *gin.Context) {
	// 1. Get latest GLOBAL sensor data (for online check)
	var globalLatest models.SensorData
	config.DB.Order("timestamp desc").First(&globalLatest)

	// 2. Get active batch
	var batch models.BatchProduksi
	if err := config.DB.Where("status_batch = ? AND is_deleted = false", "active").Order("created_at desc").First(&batch).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status": "no_active_batch",
			"message": "Tidak Ada Batch",
			"data": gin.H{
				"latest_sensor": globalLatest,
			},
		})
		return
	}

	// 3. Get latest sensor data for this batch (joining with production_histories to filter by batch_id)
	var latestData models.SensorData
	config.DB.Joins("JOIN production_histories ON production_histories.history_id = sensor_data.history_id").
		Where("production_histories.batch_id = ?", batch.BatchID).
		Order("sensor_data.timestamp desc").
		First(&latestData)

	// 4. Calculate Averages for Stats (last 24h or current run)
	var stats struct {
		AvgTemp float64 `json:"avg_temp"`
		AvgHum  float64 `json:"avg_hum"`
	}
	config.DB.Model(&models.SensorData{}).
		Joins("JOIN production_histories ON production_histories.history_id = sensor_data.history_id").
		Where("production_histories.batch_id = ?", batch.BatchID).
		Select("AVG(suhu) as avg_temp, AVG(kelembaban) as avg_hum").
		Scan(&stats)

	// 5. Calculate Fermentation Status based on Soil Moisture (percentage 0-100%)
	// Soil moisture is ONLY used for maturity display, NOT for actuator control.
	fermentationStatus := "Fase Awal"
	currentPhase := "awal"
	if latestData.SoilMoisture > 0 || latestData.SensorDataID != 0 {
		soil := latestData.SoilMoisture
		if soil >= 70 {
			fermentationStatus = "Fase Matang"
			currentPhase = "matang"
		} else if soil >= 30 {
			fermentationStatus = "Fase Aktif"
			currentPhase = "aktif"
		} else {
			fermentationStatus = "Fase Awal"
			currentPhase = "awal"
		}
	}
	if latestData.SensorDataID == 0 {
		fermentationStatus = "Menunggu Sensor..."
		currentPhase = "waiting"
	}

	// 6. Get sensor history (for chart)
	var history []models.SensorData
	config.DB.Joins("JOIN production_histories ON production_histories.history_id = sensor_data.history_id").
		Where("production_histories.batch_id = ?", batch.BatchID).
		Order("sensor_data.timestamp desc").
		Limit(20).
		Find(&history)

	// 7. Get Production History (list of runs)
	var runs []models.ProductionHistory
	config.DB.Where("batch_id = ?", batch.BatchID).Order("run_number desc").Find(&runs)

	// 8. Get system settings for UI display
	var settings models.SystemSetting
	if err := config.DB.First(&settings).Error; err != nil {
		settings = models.SystemSetting{Mode: "manual", TargetTemp: 25.0, MaxTemp: 35.0, MinHumidity: 60.0, MaxHumidity: 80.0, TargetMoisture: 30}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"batch":               batch,
			"latest_sensor":       globalLatest,
			"stats":               stats,
			"fermentation_status": fermentationStatus,
			"current_phase":       currentPhase,
			"sensor_history":      history,
			"production_runs":     runs,
			"settings":            settings,
		},
	})
}


func ControlDevice(c *gin.Context) {
	var input struct {
		Device string `json:"device"` // fan, mist, bulb
		Action string `json:"action"` // on, off
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Publish to MQTT
	topic := "tempura/device/control"
	// Sesuai dengan format di ESP32: fan_on, fan_off, mist_on, mist_off, bulb_on, bulb_off
	payload := input.Device + "_" + input.Action
	
	token := config.MQTTClient.Publish(topic, 1, false, payload)
	token.Wait()

	if token.Error() != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengirim perintah ke alat"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Perintah berhasil dikirim: " + payload,
	})
}

func GetSettings(c *gin.Context) {
	var settings models.SystemSetting
	if err := config.DB.First(&settings).Error; err != nil {
		// Create default if not exists
		settings = models.SystemSetting{
			Mode:           "manual",
			TargetTemp:     25.0,
			MaxTemp:        35.0,
			MinHumidity:    60.0,
			MaxHumidity:    80.0,
			TargetMoisture: 30,
		}
		config.DB.Create(&settings)
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": settings})
}

func UpdateSettings(c *gin.Context) {
	var input models.SystemSetting
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var settings models.SystemSetting
	config.DB.First(&settings)
	config.DB.Model(&settings).Updates(input)

	// Publish new mode to MQTT so ESP32 knows
	topic := "tempura/system/config"
	config.MQTTClient.Publish(topic, 1, true, input.Mode)

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Pengaturan diperbarui", "data": settings})
}
