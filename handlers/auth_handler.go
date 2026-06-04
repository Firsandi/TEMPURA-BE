package handlers

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"tempura-backend/config"
	"tempura-backend/models"
	"tempura-backend/services"
)

func Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	// Cari user berdasarkan email
	result := config.DB.Where("email = ? AND is_deleted = false", req.Email).First(&user)
	
	if result.Error != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "User tidak ditemukan",
		})
		return
	}

	// Cek password menggunakan bcrypt
	if !user.CheckPassword(req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "Kata sandi salah",
		})
		return
	}

	c.JSON(http.StatusOK, models.LoginResponse{
		Status:  "success",
		Message: "Login berhasil",
		Data:    &user,
	})
}

func RequestPasswordReset(c *gin.Context) {
	var input struct {
		Email string `json:"email"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find user by email
	var user models.User
	if err := config.DB.Where("email = ? AND is_deleted = false", input.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email tidak terdaftar"})
		return
	}

	// Generate 6-digit OTP token
	token, _ := generateOTP(6)
	expiresAt := time.Now().Add(1 * time.Hour) // Token valid for 1 hour

	// Create request
	request := models.PasswordResetRequest{
		Email:     input.Email,
		Token:     token,
		ExpiresAt: expiresAt,
		IsUsed:    false,
	}

	if err := config.DB.Create(&request).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat permintaan reset"})
		return
	}

	// Send OTP via SMTP with HTML Template (falls back to console print if SMTP not configured)
	if err := services.SendOTPEmail(user.Email, token); err != nil {
		fmt.Printf("Warning: Gagal mengirim email OTP: %v\n", err)
		// Still proceed - token is saved in DB
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Kode verifikasi telah dikirim ke email Anda.",
	})
}

func ResetPassword(c *gin.Context) {
	var input struct {
		Email       string `json:"email" binding:"required"`
		Token       string `json:"token" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format input tidak valid"})
		return
	}

	var request models.PasswordResetRequest
	// Find the most recent unused token for this email
	err := config.DB.Where("email = ? AND token = ? AND is_used = ?", input.Email, input.Token, false).
		Order("created_at desc").First(&request).Error

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Kode verifikasi salah atau sudah digunakan"})
		return
	}

	// Check expiry
	if time.Now().After(request.ExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Kode verifikasi telah kedaluwarsa"})
		return
	}

	// Update User Password with Hashing
	var user models.User
	if err := config.DB.Where("email = ?", request.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User tidak ditemukan"})
		return
	}
	
	user.SetPassword(input.NewPassword)
	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui kata sandi"})
		return
	}

	// Mark token as used
	config.DB.Model(&request).Update("is_used", true)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Kata sandi berhasil diperbarui. Silakan login kembali.",
	})
}

// Helper function to generate OTP
func generateOTP(n int) (string, error) {
	const digits = "0123456789"
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		ret[i] = digits[num.Int64()]
	}
	return string(ret), nil
}

func ChangePassword(c *gin.Context) {
	var input struct {
		UserID      uint   `json:"user_id" binding:"required"`
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := config.DB.First(&user, input.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User tidak ditemukan"})
		return
	}

	if !user.CheckPassword(input.OldPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Kata sandi lama salah"})
		return
	}

	user.SetPassword(input.NewPassword)
	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui kata sandi"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Kata sandi berhasil diubah",
	})
}

func UpdateProfile(c *gin.Context) {
	var input struct {
		UserID   uint   `json:"user_id" binding:"required"`
		Fullname string `json:"full_name"`
		Email    string `json:"email"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := config.DB.First(&user, input.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User tidak ditemukan"})
		return
	}

	// Check email conflict
	if input.Email != "" && input.Email != user.Email {
		var checkUser models.User
		if err := config.DB.Where("email = ? AND user_id != ?", input.Email, input.UserID).First(&checkUser).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Email sudah digunakan oleh user lain"})
			return
		}
	}

	updates := map[string]interface{}{}
	if input.Fullname != "" {
		updates["fullname"] = input.Fullname
	}
	if input.Email != "" {
		updates["email"] = input.Email
	}
	updates["updated_at"] = time.Now()

	if err := config.DB.Model(&user).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui profil"})
		return
	}

	// Reload user for response
	config.DB.First(&user, input.UserID)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Profil berhasil diperbarui",
		"data":    &user,
	})
}
