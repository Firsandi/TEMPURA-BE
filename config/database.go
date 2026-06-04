package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"tempura-backend/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDatabase() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: No .env file found, using system environment variables")
	}

	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	port := os.Getenv("DB_PORT")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=require TimeZone=Asia/Jakarta", 
		host, user, password, dbname, port)
	
	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		log.Fatalf("Gagal koneksi ke database: %v. Pastikan kredensial di .env sudah benar.", err)
	}

	DB = database
	
	// Pastikan tabel-tabel utama ada di Supabase (CREATE IF NOT EXISTS)
	rawSQL := []string{
		`CREATE TABLE IF NOT EXISTS "roles" ("role_id" bigserial PRIMARY KEY, "role_name" text)`,
		`CREATE TABLE IF NOT EXISTS "users" ("user_id" bigserial PRIMARY KEY, "email" text UNIQUE NOT NULL, "password" text NOT NULL, "fullname" text, "role_id" bigint, "is_active" boolean DEFAULT true, "is_deleted" boolean DEFAULT false, "created_at" timestamptz, "updated_at" timestamptz)`,
		`CREATE TABLE IF NOT EXISTS "batch_produksis" ("batch_id" bigserial PRIMARY KEY, "nama_batch" text, "tanggal_produksi" timestamptz, "status_batch" text DEFAULT 'draft', "jumlah_bungkus" bigint, "jumlah_kedelai" numeric, "jumlah_ragi" bigint, "created_by" bigint, "created_at" timestamptz, "is_deleted" boolean DEFAULT false)`,
		`CREATE TABLE IF NOT EXISTS "production_histories" ("history_id" bigserial PRIMARY KEY, "batch_id" bigint, "run_number" bigint, "start_time" timestamptz, "end_time" timestamptz, "status" text, "started_by" bigint, "stopped_by" bigint)`,
		`CREATE TABLE IF NOT EXISTS "sensor_data" ("sensor_data_id" bigserial PRIMARY KEY, "history_id" bigint, "suhu" numeric, "kelembaban" numeric, "soil_moisture" bigint, "relay_fan" boolean, "relay_pump" boolean, "health" text, "timestamp" timestamptz)`,
		`CREATE TABLE IF NOT EXISTS "devices" ("device_id" bigserial PRIMARY KEY, "device_name" text)`,
		`CREATE TABLE IF NOT EXISTS "device_statuses" ("status_id" bigserial PRIMARY KEY, "device_id" bigint, "status" boolean, "timestamp" timestamptz)`,
		`CREATE TABLE IF NOT EXISTS "device_control_logs" ("log_id" bigserial PRIMARY KEY, "device_id" bigint, "user_id" bigint, "action" text, "timestamp" timestamptz)`,
		`CREATE TABLE IF NOT EXISTS "system_settings" ("id" bigserial PRIMARY KEY, "mode" text DEFAULT 'manual', "target_temp" numeric DEFAULT 30, "target_moisture" bigint DEFAULT 700, "updated_at" timestamptz)`,
		`CREATE TABLE IF NOT EXISTS "password_reset_requests" ("request_id" bigserial PRIMARY KEY, "email" text, "token" text, "expires_at" timestamptz, "is_used" boolean DEFAULT false, "created_at" timestamptz, "updated_at" timestamptz)`,
	}
	for _, sql := range rawSQL {
		if err := DB.Exec(sql).Error; err != nil {
			log.Printf("Peringatan saat membuat tabel: %v", err)
		}
	}
	log.Println("Tabel-tabel dasar berhasil dipastikan ada.")

	// Auto Migrate models (akan sinkronisasi kolom yang berbeda)
	err = DB.AutoMigrate(
		&models.Role{},
		&models.User{},
		&models.BatchProduksi{},
		&models.SensorData{},
		&models.Device{},
		&models.DeviceStatus{},
		&models.DeviceControlLog{},
		&models.ProductionHistory{},
		&models.SystemSetting{},
	)
	if err != nil {
		log.Printf("Gagal migrasi database: %v", err)
	}

	// Drop the 'username' column since it's no longer used and causes NOT NULL constraint errors
	err = DB.Exec("ALTER TABLE users DROP COLUMN IF EXISTS username CASCADE").Error
	if err != nil {
		log.Printf("Gagal menghapus kolom username: %v", err)
	} else {
		log.Println("Pastikan kolom username sudah terhapus jika ada.")
	}

	// Hapus admin lama jika masih tersisa (berdasarkan email lama)
	DB.Exec("DELETE FROM users WHERE email = 'admin@tempura.com'")

	log.Println("Database connected and migrated successfully")

	// Seed Roles if empty
	var roleCount int64
	DB.Model(&models.Role{}).Count(&roleCount)
	if roleCount == 0 {
		roles := []models.Role{
			{RoleID: 1, RoleName: "Admin"},
			{RoleID: 2, RoleName: "Pegawai"},
		}
		DB.Create(&roles)
		log.Println("Default roles created")
	}

	// Seed Admin User if admin doesn't exist or force update
	var admin models.User
	result := DB.Where("email = ?", "andrawf7@gmail.com").First(&admin)
	if result.Error != nil {
		// Create new
		admin = models.User{
			Email:    "andrawf7@gmail.com",
			Fullname: "Pemilik Tempura",
			RoleID:   1,
		}
		admin.SetPassword("admin123")
		DB.Create(&admin)
		log.Println("Default admin user created: andrawf7@gmail.com / admin123")
	} else {
		// Ensure it's an admin and has the right password (hashed)
		admin.RoleID = 1
		admin.SetPassword("admin123")
		DB.Save(&admin)
		log.Println("Admin account synchronized and password hashed: andrawf7@gmail.com")
	}
}
