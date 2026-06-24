package main

import (
	"fmt"
	"log"
	"os"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type BatchProduksi struct {
	BatchID       uint    `gorm:"primaryKey;column:batch_id"`
	NamaBatch     string  `gorm:"column:nama_batch"`
	StatusBatch   string  `gorm:"column:status_batch"`
	IsDeleted     bool    `gorm:"column:is_deleted"`
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Printf("Warning: error loading .env: %v", err)
	}

	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	port := os.Getenv("DB_PORT")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=require TimeZone=Asia/Jakarta", 
		host, user, password, dbname, port)
	
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Gagal koneksi: %v", err)
	}

	var batches []BatchProduksi
	if err := db.Where("is_deleted = false").Order("batch_id desc").Find(&batches).Error; err != nil {
		log.Fatalf("Gagal select: %v", err)
	}

	fmt.Println("--- Batches (Not Deleted) ---")
	for _, b := range batches {
		fmt.Printf("ID: %d, Name: %s, Status: %s\n", b.BatchID, b.NamaBatch, b.StatusBatch)
	}
}
