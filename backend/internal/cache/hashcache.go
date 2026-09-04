package cache

import (
	"database/sql"
	"time"

	"lysis/internal/db"
)

type HashCache struct {
	DB *sql.DB
}

func NewHashCache(database *sql.DB) *HashCache {
	return &HashCache{DB: database}
}

func (c *HashCache) Get(hash string) (*db.HashCache, error) {
	return db.GetHashCache(c.DB, hash)
}

func (c *HashCache) Set(hash string, vtResult, mbResult, classification string) error {
	h := &db.HashCache{
		Hash:             hash,
		VirusTotalResult: &vtResult,
		AbuseChResult:    &mbResult,
		Classification:   &classification,
	}
	return db.SetHashCache(c.DB, h)
}

func (c *HashCache) IsFresh(cachedAt string, ttlHours int) bool {
	t, err := time.Parse(time.RFC3339, cachedAt)
	if err != nil {
		return false
	}
	return time.Since(t) < time.Duration(ttlHours)*time.Hour
}
