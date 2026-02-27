package db

import (
	"time"
)

// AuthConfig stores credentials for web UI sessions
type AuthConfig struct {
	ID        uint      `gorm:"primarykey"`
	Username  string    `gorm:"uniqueIndex;not null"`
	Password  string    `gorm:"not null"` // Hashed password
	APIKey    string    `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// LibraryHistory stores historical capacity logs
type LibraryHistory struct {
	ID            uint      `gorm:"primarykey"`
	Timestamp     time.Time `gorm:"index;not null"`
	TotalCapacity int64     `gorm:"not null"`
	UsedCapacity  int64     `gorm:"not null"`
	Resolution    string    `gorm:"index;not null"` // "raw", "hourly", "daily", "weekly"
	CreatedAt     time.Time
}
