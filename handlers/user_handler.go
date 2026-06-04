package handlers

import (
	"crypto/rand"
	"math/big"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"tempura-backend/config"
	"tempura-backend/models"
	"tempura-backend/services"
)

// generateRandomPassword creates an 8-character random password
func generateRandomPassword() (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 8)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}

func GetEmployees(c *gin.Context) {
	var users []models.User
	// RoleID = 2 usually means Employee (Pegawai). 
	// We return only active employees for the management page.
	if err := config.DB.Where("role_id = ? AND is_deleted = false", 2).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data pegawai"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   users,
	})
}

func CreateEmployee(c *gin.Context) {
	var input struct {
		Fullname string `json:"full_name" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if email already exists
	var existingUser models.User
	if err := config.DB.Where("email = ?", input.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email sudah digunakan"})
		return
	}

	// Auto-generate password
	password, err := generateRandomPassword()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat password akun"})
		return
	}

	user := models.User{
		Email:    input.Email,
		Fullname: input.Fullname,
		RoleID:   2,        // Pegawai Role
		IsActive: true,
	}
	user.SetPassword(password) // Hash the auto-generated password

	if err := config.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan akun pegawai"})
		return
	}

	// Send email with credentials
	go func() {
		// Run in background so it doesn't block the API response
		_ = services.SendAccountEmail(user.Email, password)
	}()

	c.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"message": "Akun pegawai berhasil dibuat, password dikirim via email",
		"data":    user,
	})
}

func UpdateEmployee(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		Fullname string `json:"full_name"`
		Email    string `json:"email"`
		IsActive *bool  `json:"is_active"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := config.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pegawai tidak ditemukan"})
		return
	}

	// Check conflict

	if input.Email != "" && input.Email != user.Email {
		var checkUser models.User
		if err := config.DB.Where("email = ?", input.Email).First(&checkUser).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Email sudah digunakan"})
			return
		}
	}

	updates := map[string]interface{}{}
	if input.Fullname != "" { updates["fullname"] = input.Fullname }
	if input.Email != "" { updates["email"] = input.Email }
	if input.IsActive != nil { updates["is_active"] = *input.IsActive }
	updates["updated_at"] = time.Now()

	if err := config.DB.Model(&user).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengupdate akun pegawai"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data pegawai berhasil diperbarui",
	})
}

func DeleteEmployee(c *gin.Context) {
	id := c.Param("id")
	
	if err := config.DB.Model(&models.User{}).Where("user_id = ?", id).Update("is_deleted", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus pegawai"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Akun pegawai berhasil dihapus",
	})
}
