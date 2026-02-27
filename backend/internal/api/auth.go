package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/capacitarr/capacitarr/backend/internal/config"
	"github.com/capacitarr/capacitarr/backend/internal/db"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func LoginHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		var user db.AuthConfig
		if err := db.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
			// If no user exists in DB at all, bootstrap the first user
			var count int64
			db.DB.Model(&db.AuthConfig{}).Count(&count)
			if count == 0 {
				hashed, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
				user = db.AuthConfig{Username: req.Username, Password: string(hashed)}
				db.DB.Create(&user)
			} else {
				http.Error(w, "Invalid credentials", http.StatusUnauthorized)
				return
			}
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": user.Username,
			"exp": time.Now().Add(24 * time.Hour).Unix(),
		})

		tokenString, err := token.SignedString([]byte(cfg.JWTSecret))
		if err != nil {
			http.Error(w, "Error generating token", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "jwt",
			Value:    tokenString,
			Expires:  time.Now().Add(24 * time.Hour),
			HttpOnly: true,
			Secure:   false, // Set to true in production
			Path:     "/",
			SameSite: http.SameSiteLaxMode,
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "success"})
	}
}

func GenerateAPIKeyHandler(cfg *config.Config) http.HandlerFunc {
	// Handled by AuthMiddleware ensures user is logged in
	return func(w http.ResponseWriter, r *http.Request) {
		username := r.Context().Value(UserContextKey).(string)

		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err != nil {
			http.Error(w, "Error generating API key", http.StatusInternalServerError)
			return
		}
		apiKey := hex.EncodeToString(bytes)

		if err := db.DB.Model(&db.AuthConfig{}).Where("username = ?", username).Update("api_key", apiKey).Error; err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"api_key": apiKey})
	}
}
