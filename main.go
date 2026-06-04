package main

import (
	"log"
	"net/http"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"tempura-backend/handlers"
	"tempura-backend/config"
	"tempura-backend/services"
)

func main() {
	// 1. Initialize Database (Supabase)
	config.ConnectDatabase()

	// 2. Initialize MQTT
	config.InitMQTT()
	services.StartMQTTSubscription()

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
	log.Println("Server running on port 8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Gagal menjalankan server: %v", err)
	}
}
