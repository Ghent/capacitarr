package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"capacitarr/internal/config"
	"capacitarr/internal/db"
	"capacitarr/internal/events"
)

// Sentinel errors for authentication operations.
var (
	ErrWrongPassword = errors.New("password is incorrect")
	ErrUserNotFound  = errors.New("user not found")
)

// BcryptCost is the cost factor for bcrypt password hashing across all auth
// operations. The Go default is 10; we use 12 for stronger brute-force
// resistance while keeping hashing under ~250ms on typical hardware.
const BcryptCost = 12

// AuthService manages authentication, password changes, username changes, and API keys.
type AuthService struct {
	db  *gorm.DB
	bus *events.EventBus
	cfg *config.Config
}

// NewAuthService creates a new AuthService.
func NewAuthService(database *gorm.DB, bus *events.EventBus, cfg *config.Config) *AuthService {
	return &AuthService{db: database, bus: bus, cfg: cfg}
}

// Login verifies credentials and returns a JWT token on success.
func (s *AuthService) Login(username, password string) (string, error) {
	var auth db.AuthConfig
	if err := s.db.Where("username = ?", username).First(&auth).Error; err != nil {
		return "", fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(auth.Password), []byte(password)); err != nil {
		return "", fmt.Errorf("invalid credentials")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": username,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	s.bus.Publish(events.LoginEvent{Username: username})
	return tokenString, nil
}

// ChangePassword verifies the current password and sets a new one.
func (s *AuthService) ChangePassword(username, currentPwd, newPwd string) error {
	var auth db.AuthConfig
	if err := s.db.Where("username = ?", username).First(&auth).Error; err != nil {
		return fmt.Errorf("%w: %v", ErrUserNotFound, err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(auth.Password), []byte(currentPwd)); err != nil {
		return ErrWrongPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPwd), BcryptCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	if err := s.db.Model(&auth).Update("password", string(hash)).Error; err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	s.bus.Publish(events.PasswordChangedEvent{Username: username})
	return nil
}

// ChangeUsername verifies the password and updates the username.
func (s *AuthService) ChangeUsername(currentUser, newUsername, password string) error {
	var auth db.AuthConfig
	if err := s.db.Where("username = ?", currentUser).First(&auth).Error; err != nil {
		return fmt.Errorf("%w: %v", ErrUserNotFound, err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(auth.Password), []byte(password)); err != nil {
		return ErrWrongPassword
	}

	if err := s.db.Model(&auth).Update("username", newUsername).Error; err != nil {
		return fmt.Errorf("failed to update username: %w", err)
	}

	s.bus.Publish(events.UsernameChangedEvent{
		OldUsername: currentUser,
		NewUsername: newUsername,
	})
	return nil
}

// IsInitialized returns whether at least one user exists in the database.
func (s *AuthService) IsInitialized() (bool, error) {
	var count int64
	if err := s.db.Model(&db.AuthConfig{}).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check auth initialization: %w", err)
	}
	return count > 0, nil
}

// Bootstrap creates the first user in a transaction. Returns the created user
// if this call performed the creation, or nil if another user already exists.
func (s *AuthService) Bootstrap(username, password string) (*db.AuthConfig, error) {
	var user *db.AuthConfig
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&db.AuthConfig{}).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil // Another request already created the first user
		}
		hashed, hashErr := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
		if hashErr != nil {
			return hashErr
		}
		auth := db.AuthConfig{Username: username, Password: string(hashed)}
		if err := tx.Create(&auth).Error; err != nil {
			return err
		}
		user = &auth
		slog.Info("First user bootstrapped", "component", "auth", "username", username)
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("bootstrap failed: %w", txErr)
	}
	return user, nil
}

// GetByUsername looks up a user by username.
func (s *AuthService) GetByUsername(username string) (*db.AuthConfig, error) {
	var auth db.AuthConfig
	if err := s.db.Where("username = ?", username).First(&auth).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &auth, nil
}

// IsUsernameTaken checks whether a username is already in use.
func (s *AuthService) IsUsernameTaken(username string) (bool, error) {
	var count int64
	if err := s.db.Model(&db.AuthConfig{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check username: %w", err)
	}
	return count > 0, nil
}

// ValidateAPIKey checks the given plaintext API key against stored (hashed or
// legacy plaintext) keys. If a legacy plaintext key matches, it is transparently
// upgraded to a SHA-256 hash. Returns the matching AuthConfig on success.
func (s *AuthService) ValidateAPIKey(plaintextKey string) (*db.AuthConfig, error) {
	hashedKey := HashAPIKey(plaintextKey)

	// Fast path: look up by the hashed value (new-style keys)
	var auth db.AuthConfig
	if err := s.db.Where("api_key = ?", hashedKey).First(&auth).Error; err == nil {
		return &auth, nil
	}

	// Slow path: legacy plaintext key — look up directly and upgrade in-place
	if err := s.db.Where("api_key = ?", plaintextKey).First(&auth).Error; err == nil {
		if !IsHashedAPIKey(auth.APIKey) {
			s.db.Model(&auth).Update("api_key", hashedKey)
			slog.Info("Upgraded legacy plaintext API key to SHA-256 hash", "component", "auth", "username", auth.Username)
		}
		return &auth, nil
	}

	return nil, fmt.Errorf("invalid API key")
}

// EnsureProxyUser creates a user record for a proxy-auth header user if one
// doesn't already exist. The created user has an unusable password hash.
func (s *AuthService) EnsureProxyUser(username string) error {
	var auth db.AuthConfig
	if err := s.db.Where("username = ?", username).First(&auth).Error; err == nil {
		return nil // user already exists
	}

	placeholder, _ := bcrypt.GenerateFromPassword([]byte("proxy-auth-placeholder"), BcryptCost)
	auth = db.AuthConfig{
		Username: username,
		Password: string(placeholder),
	}
	if err := s.db.Create(&auth).Error; err != nil {
		return fmt.Errorf("failed to create proxy user: %w", err)
	}
	slog.Info("Auto-created user from proxy auth header", "component", "auth", "username", username) //nolint:gosec // username is from a trusted reverse proxy header
	return nil
}

// apiKeyHashPrefix marks a stored API key as already hashed with SHA-256.
// Legacy plaintext keys (without this prefix) are transparently upgraded
// on first use.
const apiKeyHashPrefix = "sha256:"

// HashAPIKey produces a deterministic SHA-256 hash of the given plaintext
// API key, prefixed with "sha256:" so we can distinguish hashed from legacy
// plaintext keys in the database. SHA-256 (without salt) is appropriate here
// because API keys are 256-bit random values — effectively unguessable and
// immune to rainbow table attacks.
func HashAPIKey(plaintext string) string {
	h := sha256.Sum256([]byte(plaintext))
	return apiKeyHashPrefix + hex.EncodeToString(h[:])
}

// IsHashedAPIKey returns true if the stored key already uses the hashed format.
func IsHashedAPIKey(stored string) bool {
	return strings.HasPrefix(stored, apiKeyHashPrefix)
}

// GenerateAPIKey creates a new API key, stores its SHA-256 hash and hint,
// and returns the plaintext key (shown only once).
func (s *AuthService) GenerateAPIKey(username string) (string, error) {
	var auth db.AuthConfig
	if err := s.db.Where("username = ?", username).First(&auth).Error; err != nil {
		return "", fmt.Errorf("user not found")
	}

	// Generate 32 random bytes → 64 hex characters
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", fmt.Errorf("failed to generate random key: %w", err)
	}
	plaintext := hex.EncodeToString(keyBytes)

	// Store SHA-256 hash and hint (last 4 chars)
	hashHex := HashAPIKey(plaintext)
	hint := plaintext[len(plaintext)-4:]

	if err := s.db.Model(&auth).Updates(map[string]any{
		"api_key":      hashHex,
		"api_key_hint": hint,
	}).Error; err != nil {
		return "", fmt.Errorf("failed to store API key: %w", err)
	}

	s.bus.Publish(events.APIKeyGeneratedEvent{
		Username: username,
		Hint:     hint,
	})

	return plaintext, nil
}
