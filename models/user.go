package models

import (
	"time"
	"golang.org/x/crypto/bcrypt"
)

type Role struct {
	RoleID   uint   `gorm:"primaryKey;column:role_id" json:"role_id"`
	RoleName string `gorm:"unique;not null" json:"role_name"` // Admin, Pegawai
}

type User struct {
	UserID       uint      `gorm:"primaryKey;column:user_id" json:"id"`
	Email        string    `gorm:"unique;not null" json:"email"`
	Password     string    `gorm:"not null" json:"-"` // Hashed
	Fullname     string    `json:"full_name"`
	RoleID       uint      `json:"role_id"`
	Role         Role      `gorm:"foreignKey:RoleID" json:"role"`
	IsActive     bool      `gorm:"default:true" json:"is_active"`
	IsDeleted    bool      `gorm:"default:false" json:"is_deleted"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (u *User) SetPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

type PasswordResetRequest struct {
	RequestID uint      `gorm:"primaryKey;column:request_id" json:"request_id"`
	Email     string    `json:"email"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	IsUsed    bool      `gorm:"default:false" json:"is_used"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    *User  `json:"data,omitempty"`
}
