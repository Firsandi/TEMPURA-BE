package main

import (
	"log"
	"net/http"
	"os"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"tempura-backend/handlers"
	"tempura-backend/config"
	"tempura-backend/models"
	"tempura-backend/services"
	"time"
)

func monitorDeviceStatus() {
	var lastOfflineAlertTime time.Time
	alertCooldown := 30 * time.Minute

	for {
		var latestSensor models.SensorData
		if err := config.DB.Order("timestamp desc").First(&latestSensor).Error; err == nil {
			now := time.Now()
			// If latest sensor data is older than 5 minutes
			if now.Sub(latestSensor.Timestamp) > 5*time.Minute {
				if now.Sub(lastOfflineAlertTime) > alertCooldown {
					services.SendAlertNotification(
						"Perangkat Offline!",
						"Tidak ada data masuk dari perangkat selama lebih dari 5 menit.",
						"device_offline",
					)
					lastOfflineAlertTime = now
				}
			}
		}
		
		// Tidur 1 menit sebelum mengecek lagi
		time.Sleep(1 * time.Minute)
	}
}

func main() {
	// 1. Initialize Database (Supabase)
	config.ConnectDatabase()

	// 2. Initialize MQTT
	config.InitMQTT()

	// 3. Initialize Firebase Admin SDK (for FCM push notifications)
	if err := services.InitFirebase(); err != nil {
		log.Printf("Warning: Firebase init failed, FCM notifications disabled: %v", err)
	}

	services.StartMQTTSubscription()
	go monitorDeviceStatus()

	// 3. Setup Router
	r := gin.Default()

	// 4. Allow CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// 5. Routes
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	auth := r.Group("/auth")
	{
		auth.POST("/login", handlers.Login)
		auth.POST("/forgot-password", handlers.RequestPasswordReset)
		auth.POST("/reset-password", handlers.ResetPassword)
		auth.POST("/change-password", handlers.ChangePassword)
		auth.PUT("/update-profile", handlers.UpdateProfile)
	}

	batchGroup := r.Group("/batch")
	{
		batchGroup.GET("", handlers.GetBatches)
		batchGroup.POST("", handlers.CreateBatch)
		batchGroup.GET("/:id", handlers.GetBatchDetail)
		batchGroup.PUT("/:id", handlers.UpdateBatch)
		batchGroup.DELETE("/:id", handlers.DeleteBatch)
		batchGroup.PUT("/:id/start", handlers.StartBatch)
		batchGroup.PUT("/:id/stop", handlers.StopBatch)
	}

	dashboard := r.Group("/dashboard")
	{
		dashboard.GET("/latest", handlers.GetDashboardData)
		dashboard.POST("/control", handlers.ControlDevice)
		dashboard.GET("/settings", handlers.GetSettings)
		dashboard.PUT("/settings", handlers.UpdateSettings)
	}

	userGroup := r.Group("/users")
	{
		userGroup.GET("", handlers.GetEmployees)
		userGroup.POST("", handlers.CreateEmployee)
		userGroup.PUT("/:id", handlers.UpdateEmployee)
		userGroup.DELETE("/:id", handlers.DeleteEmployee)
	}

	// 6. Run Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server running on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Gagal menjalankan server: %v", err)
	}
}
