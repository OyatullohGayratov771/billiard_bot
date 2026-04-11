package bot

import (
	"sync"
	"time"

	"bot-gateway/internal/models"
)

const userCacheTTL = 60 * time.Second

type cachedUser struct {
	user      *models.User
	expiresAt time.Time
}

// UserCache — xotiraga foydalanuvchilarni saqlaydi (TTL bilan)
type UserCache struct {
	mu    sync.RWMutex
	items map[int64]cachedUser
}

func newUserCache() *UserCache {
	c := &UserCache{items: make(map[int64]cachedUser)}
	go c.gc() // expired yozuvlarni tozalab turadi
	return c
}

func (c *UserCache) get(tgID int64) (*models.User, bool) {
	c.mu.RLock()
	entry, ok := c.items[tgID]
	c.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.user, true
}

func (c *UserCache) set(tgID int64, user *models.User) {
	c.mu.Lock()
	c.items[tgID] = cachedUser{user: user, expiresAt: time.Now().Add(userCacheTTL)}
	c.mu.Unlock()
}

// Invalidate — rol o'zgarganda yoki deactivate bo'lganda chaqiriladi
func (c *UserCache) Invalidate(tgID int64) {
	c.mu.Lock()
	delete(c.items, tgID)
	c.mu.Unlock()
}

// gc — har 5 daqiqada expired yozuvlarni o'chiradi
func (c *UserCache) gc() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		now := time.Now()
		c.mu.Lock()
		for id, entry := range c.items {
			if now.After(entry.expiresAt) {
				delete(c.items, id)
			}
		}
		c.mu.Unlock()
	}
}
