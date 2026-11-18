package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Pari1i1i/WattWise/backend/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	db *gorm.DB
}

func NewAuthService(db *gorm.DB) *AuthService {
	return &AuthService{db: db}
}

type TokenPair struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	User         domain.User `json:"user"`
}

func (s *AuthService) getJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "wattwise-default-secret-key"
	}
	return []byte(secret)
}

// GenerateSession membuat JWT Access Token & menyimpan Refresh Token ke tabel user_sessions
func (s *AuthService) GenerateSession(user domain.User, userAgent, clientIP string) (*TokenPair, error) {
	sessionID := uuid.New()

	// 1. Buat Access Token (Berlaku 24 Jam untuk kemudahan demo)
	accessClaims := jwt.MapClaims{
		"sub":        user.ID.String(),
		"session_id": sessionID.String(),
		"email":      user.Email,
		"exp":        time.Now().Add(24 * time.Hour).Unix(),
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString(s.getJWTSecret())
	if err != nil {
		return nil, err
	}

	// 2. Buat Refresh Token (Berlaku 7 Hari)
	refreshToken := uuid.New().String() + "-" + uuid.New().String()
	session := domain.UserSession{
		ID:           sessionID,
		UserID:       user.ID,
		RefreshToken: refreshToken,
		UserAgent:    userAgent,
		ClientIP:     clientIP,
		ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
		IsRevoked:    false,
	}

	if err := s.db.Create(&session).Error; err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	}, nil
}

func (s *AuthService) Register(name, email, password, userAgent, clientIP string) (*TokenPair, error) {
	var existing domain.User
	if err := s.db.Where("email = ?", email).First(&existing).Error; err == nil {
		return nil, errors.New("email sudah terdaftar")
	}

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	hashedStr := string(hashedBytes)

	user := domain.User{
		Name:         name,
		Email:        email,
		PasswordHash: &hashedStr,
		AuthProvider: "LOCAL",
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}

	return s.GenerateSession(user, userAgent, clientIP)
}

func (s *AuthService) Login(email, password, userAgent, clientIP string) (*TokenPair, error) {
	var user domain.User
	if err := s.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, errors.New("email atau password salah")
	}

	if user.PasswordHash == nil {
		return nil, errors.New("akun ini terdaftar menggunakan Google Login, silakan masuk dengan Google")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("email atau password salah")
	}

	return s.GenerateSession(user, userAgent, clientIP)
}

// LoginWithGoogle memverifikasi Google ID Token langsung ke server Google OAuth2
func (s *AuthService) LoginWithGoogle(idToken, userAgent, clientIP string) (*TokenPair, error) {
	resp, err := http.Get(fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", idToken))
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, errors.New("token Google tidak valid atau sudah kedaluwarsa")
	}
	defer resp.Body.Close()

	var googleData struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&googleData); err != nil {
		return nil, errors.New("gagal membaca profil Google")
	}

	var user domain.User
	err = s.db.Where("email = ?", googleData.Email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Buat user baru otomatis jika belum terdaftar
		user = domain.User{
			Name:         googleData.Name,
			Email:        googleData.Email,
			GoogleID:     &googleData.Sub,
			AvatarURL:    googleData.Picture,
			AuthProvider: "GOOGLE",
		}
		if err := s.db.Create(&user).Error; err != nil {
			return nil, err
		}
	} else if err == nil {
		// Update GoogleID & Avatar jika user sebelumnya daftar manual
		user.GoogleID = &googleData.Sub
		if user.AvatarURL == "" {
			user.AvatarURL = googleData.Picture
		}
		s.db.Save(&user)
	} else {
		return nil, err
	}

	return s.GenerateSession(user, userAgent, clientIP)
}

func (s *AuthService) RevokeSession(sessionID string) error {
	return s.db.Model(&domain.UserSession{}).
		Where("id = ?", sessionID).
		Update("is_revoked", true).Error
}
