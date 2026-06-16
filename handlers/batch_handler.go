package handlers

import (
	"fmt"
	"net/http"
	"time"
	"strconv"
	"tempura-backend/config"
	"tempura-backend/models"
	"tempura-backend/services"

	"github.com/gin-gonic/gin"
)

// GetBatches returns all batches, including drafts
func GetBatches(c *gin.Context) {
	var batches []models.BatchProduksi
	if err := config.DB.Where("is_deleted = false").Order("status_batch desc, created_at desc").Find(&batches).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data batch"})
		return
	}

	// Build response with active_start_time for active batches
	type BatchResponse struct {
		models.BatchProduksi
		ActiveStartTime *time.Time `json:"active_start_time"`
	}

	var response []BatchResponse
	for _, batch := range batches {
		br := BatchResponse{BatchProduksi: batch}
		if batch.StatusBatch == "active" {
			var history models.ProductionHistory
			if err := config.DB.Where("batch_id = ? AND end_time IS NULL", batch.BatchID).First(&history).Error; err == nil {
				br.ActiveStartTime = &history.StartTime
			}
		}
		response = append(response, br)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   response,
	})
}

// StartBatch moves a batch from draft/completed to active
func StartBatch(c *gin.Context) {
	id := c.Param("id")

	// 1. Check if sensors are detected (latest sensor < 30s)
	var latest models.SensorData
	config.DB.Order("timestamp desc").First(&latest)
	if time.Since(latest.Timestamp) > 30*time.Second {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Sensor tidak terdeteksi. Pastikan alat IoT aktif sebelum memulai batch.",
		})
		return
	}

	// 2. Check if another batch is already active
	var activeCount int64
	config.DB.Model(&models.BatchProduksi{}).Where("status_batch = ? AND is_deleted = false", "active").Count(&activeCount)
	if activeCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Masih ada batch yang sedang berjalan. Hentikan batch tersebut terlebih dahulu.",
		})
		return
	}

	// 3. Get Batch
	var batch models.BatchProduksi
	if err := config.DB.First(&batch, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Batch tidak ditemukan"})
		return
	}

	// 4. Create Production History record
	var lastRun int
	config.DB.Model(&models.ProductionHistory{}).Where("batch_id = ?", batch.BatchID).Select("MAX(run_number)").Row().Scan(&lastRun)
	
	userIDStr := c.Query("user_id")
	var startedBy *uint
	if userIDStr != "" {
		if idInt, err := strconv.Atoi(userIDStr); err == nil {
			uid := uint(idInt)
			startedBy = &uid
		}
	}

	history := models.ProductionHistory{
		BatchID:   batch.BatchID,
		RunNumber: lastRun + 1,
		StartTime: time.Now(),
		Status:    "Berjalan", // Temporary status
		StartedBy: startedBy,
	}
	config.DB.Create(&history)

	// 5. Update batch status to active and reset end_timestamp
	config.DB.Model(&batch).Updates(map[string]interface{}{
		"status_batch":  "active",
		"end_timestamp": nil,
	})

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Batch berhasil dijalankan",
		"data":    history,
	})
}

// UpdateBatch updates draft batch details
func UpdateBatch(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		NamaBatch     string  `json:"nama_batch"`
		JumlahBungkus int     `json:"jumlah_bungkus"`
		JumlahKedelai float64 `json:"jumlah_kedelai"`
		JumlahRagi    int     `json:"jumlah_ragi"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Only allow update if status is draft
	var batch models.BatchProduksi
	if err := config.DB.Where("batch_id = ?", id).First(&batch).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Batch tidak ditemukan"})
		return
	}

	if batch.StatusBatch != "draft" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya batch berstatus draft yang dapat diubah"})
		return
	}

	if err := config.DB.Model(&batch).Updates(input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui batch"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Batch berhasil diperbarui"})
}

// DeleteBatch deletes a draft batch
func DeleteBatch(c *gin.Context) {
	id := c.Param("id")
	
	var batch models.BatchProduksi
	if err := config.DB.Where("batch_id = ?", id).First(&batch).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Batch tidak ditemukan"})
		return
	}

	// Check if it has history (already active or completed)
	if batch.StatusBatch != "draft" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Batch yang sudah berjalan tidak dapat dihapus"})
		return
	}

	if err := config.DB.Model(&batch).Update("is_deleted", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus batch"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Batch berhasil dihapus"})
}



// StopBatch marks a batch as completed (Manual)
func StopBatch(c *gin.Context) {
	id := c.Param("id")
	
	userIDStr := c.Query("user_id")
	var stoppedBy *uint
	if userIDStr != "" {
		if idInt, err := strconv.Atoi(userIDStr); err == nil {
			uid := uint(idInt)
			stoppedBy = &uid
		}
	}

	if err := CompleteBatch(id, "Fermentasi Dihentikan (dihentikan paksa)", stoppedBy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Batch berhasil dihentikan paksa",
	})
}

// CompleteBatch is a helper to finalize a batch session
func CompleteBatch(batchID interface{}, status string, stoppedBy *uint) error {
	now := time.Now()
	
	// Get batch name before completing to send notification
	var batch models.BatchProduksi
	if err := config.DB.First(&batch, batchID).Error; err == nil {
		go services.SendHarvestNotification(batch.NamaBatch)
	}

	// 1. Update Batch status and end_timestamp
	if err := config.DB.Model(&models.BatchProduksi{}).Where("batch_id = ?", batchID).Updates(map[string]interface{}{
		"status_batch":  "completed",
		"end_timestamp": &now,
	}).Error; err != nil {
		return fmt.Errorf("gagal update status batch: %v", err)
	}

	// 2. Update Production History
	updates := map[string]interface{}{
		"end_time": &now,
		"status":   status,
	}
	if stoppedBy != nil {
		updates["stopped_by"] = stoppedBy
	}

	if err := config.DB.Model(&models.ProductionHistory{}).
		Where("batch_id = ? AND end_time IS NULL", batchID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("gagal update history: %v", err)
	}

	return nil
}

func GetBatchDetail(c *gin.Context) {
	id := c.Param("id")

	var batch models.BatchProduksi
	if err := config.DB.First(&batch, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Batch tidak ditemukan"})
		return
	}

	var runs []models.ProductionHistory
	config.DB.Where("batch_id = ?", batch.BatchID).Order("run_number desc").Find(&runs)

	type RunWithUser struct {
		models.ProductionHistory
		StartedByName string `json:"started_by_name"`
		StoppedByName string `json:"stopped_by_name"`
	}

	var runsWithUser []RunWithUser
	for _, run := range runs {
		var rwu RunWithUser
		rwu.ProductionHistory = run

		if run.StartedBy != nil {
			var u models.User
			if config.DB.First(&u, *run.StartedBy).Error == nil {
				rwu.StartedByName = u.Fullname
			}
		}
		if run.StoppedBy != nil {
			var u models.User
			if config.DB.First(&u, *run.StoppedBy).Error == nil {
				rwu.StoppedByName = u.Fullname
			}
		}
		runsWithUser = append(runsWithUser, rwu)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"batch":           batch,
			"production_runs": runsWithUser,
		},
	})
}

func CreateBatch(c *gin.Context) {
	var input struct {
		NamaBatch     string  `json:"nama_batch"`
		JumlahBungkus int     `json:"jumlah_bungkus"`
		JumlahKedelai float64 `json:"jumlah_kedelai"`
		JumlahRagi    int     `json:"jumlah_ragi"`
		CreatedBy     uint    `json:"created_by"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if batch name already exists
	var existing models.BatchProduksi
	if err := config.DB.Where("nama_batch = ? AND is_deleted = false", input.NamaBatch).First(&existing).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nama batch sudah ada. Gunakan nama lain."})
		return
	}

	batch := models.BatchProduksi{
		NamaBatch:       input.NamaBatch,
		JumlahBungkus:   input.JumlahBungkus,
		JumlahKedelai:   input.JumlahKedelai,
		JumlahRagi:      input.JumlahRagi,
		TanggalProduksi: time.Now(),
		StatusBatch:     "draft", // Initial status is draft
		CreatedBy:       input.CreatedBy,
		IsDeleted:       false,
	}

	if err := config.DB.Create(&batch).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat batch baru"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Batch draft berhasil dibuat",
		"data":    batch,
	})
}

