package bot

import (
	"log"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"bot-gateway/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// authUser — foydalanuvchini cache dan yoki user-service dan oladi.
func (h *Handler) authUser(tgID int64) *models.User {
	if u, ok := h.userCache.get(tgID); ok {
		return u
	}
	user, err := h.userSvc.GetByTelegramID(tgID)
	if err != nil {
		return nil
	}
	if !user.IsActive {
		return nil
	}
	h.userCache.set(tgID, user)
	return user
}

// requireRole — foydalanuvchi kerakli rolda emasligini tekshiradi.
// false qaytarsa access yo'q deb xabar yuboradi.
func (h *Handler) requireRole(bot *tgbotapi.BotAPI, chatID int64, user *models.User, roles ...string) bool {
	if user == nil {
		send(bot, chatID, "⛔ Siz ro'yxatdan o'tmagansiz. /start bosing.")
		return false
	}
	for _, r := range roles {
		if user.Role == r {
			return true
		}
	}
	send(bot, chatID, "⛔ Sizda bu amalni bajarish huquqi yo'q.")
	return false
}

// requireStaff — superadmin/admin/operator bo'lishi shart
func (h *Handler) requireStaff(bot *tgbotapi.BotAPI, chatID int64, user *models.User) bool {
	return h.requireRole(bot, chatID, user,
		models.RoleSuperadmin, models.RoleAdmin, models.RoleOperator)
}

// logAction — audit log: kim, nima qildi
func (h *Handler) logAction(user *models.User, action, details string) {
	log.Printf("audit: [%s] @%s (tg:%d) → %s | %s",
		user.Role, user.Username, user.TelegramID, action, details)
}

// rateLimiter — sliding window: 10 soniyada 15 ta so'rovdan oshsa bloklaydi
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[int64][]time.Time
}

func newRateLimiter() *rateLimiter {
	rl := &rateLimiter{buckets: make(map[int64][]time.Time)}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) isBlocked(tgID int64) bool {
	const window = 10 * time.Second
	const limit = 15

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-window)

	ts := rl.buckets[tgID]
	filtered := ts[:0]
	for _, t := range ts {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	filtered = append(filtered, now)
	rl.buckets[tgID] = filtered

	if len(filtered) > limit {
		log.Printf("⚠️  Rate limit: tg:%d (%d req/10s)", tgID, len(filtered))
		return true
	}
	return false
}

func (rl *rateLimiter) cleanup() {
	for range time.Tick(5 * time.Minute) {
		rl.mu.Lock()
		cutoff := time.Now().Add(-10 * time.Second)
		for id, ts := range rl.buckets {
			alive := ts[:0]
			for _, t := range ts {
				if t.After(cutoff) {
					alive = append(alive, t)
				}
			}
			if len(alive) == 0 {
				delete(rl.buckets, id)
			} else {
				rl.buckets[id] = alive
			}
		}
		rl.mu.Unlock()
	}
}

// safeText — matn uzunligini cheklaydi (Unicode rune bo'yicha)
func safeText(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes])
}

// getUserFromUpdate — message yoki callback'dan telegram ID oladi
func getUserFromUpdate(update tgbotapi.Update) (int64, string, string, string) {
	if update.Message != nil {
		f := update.Message.From
		return f.ID, f.UserName, f.FirstName, f.LastName
	}
	if update.CallbackQuery != nil {
		f := update.CallbackQuery.From
		return f.ID, f.UserName, f.FirstName, f.LastName
	}
	return 0, "", "", ""
}

// send — xabar yuborish helper
func send(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	if _, err := bot.Send(msg); err != nil {
		log.Printf("send error: %v", err)
	}
}

// sendWithKeyboard — klaviatura bilan xabar
func sendWithKeyboard(bot *tgbotapi.BotAPI, chatID int64, text string, kb interface{}) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	switch k := kb.(type) {
	case tgbotapi.InlineKeyboardMarkup:
		msg.ReplyMarkup = k
	case tgbotapi.ReplyKeyboardMarkup:
		msg.ReplyMarkup = k
	case tgbotapi.ReplyKeyboardRemove:
		msg.ReplyMarkup = k
	}
	if _, err := bot.Send(msg); err != nil {
		log.Printf("sendWithKeyboard error: %v", err)
	}
}

// answerCallback — callback query'ga javob beradi
func answerCallback(bot *tgbotapi.BotAPI, callbackID, text string) {
	cb := tgbotapi.NewCallback(callbackID, text)
	if _, err := bot.Request(cb); err != nil {
		log.Printf("callback answer error: %v", err)
	}
}

// editMessage — inline xabarni tahrirlaydi
func editMessage(bot *tgbotapi.BotAPI, chatID int64, msgID int, text string, kb *tgbotapi.InlineKeyboardMarkup) {
	edit := tgbotapi.NewEditMessageText(chatID, msgID, text)
	edit.ParseMode = "HTML"
	if kb != nil {
		edit.ReplyMarkup = kb
	}
	if _, err := bot.Send(edit); err != nil {
		log.Printf("editMessage error: %v", err)
	}
}

func (h *Handler) getOrRegister(tgID int64, username, firstName, lastName string) (*models.User, error) {
	if u, ok := h.userCache.get(tgID); ok {
		return u, nil
	}
	user, err := h.userSvc.RegisterOrGet(tgID, username, firstName, lastName)
	if err != nil {
		return nil, err
	}
	h.userCache.set(tgID, user)
	return user, nil
}

// statusText — klip holat kodini Uzbekcha icon+matn sifatida qaytaradi
func statusText(status string) string {
	switch status {
	case models.ClipStatusPending:
		return "⏳ Kutilmoqda"
	case models.ClipStatusPaid:
		return "💰 To'langan"
	case models.ClipStatusProcessing:
		return "⚙️ Jarayonda"
	case models.ClipStatusDone:
		return "✅ Tayyor"
	case models.ClipStatusFailed:
		return "❌ Rad etildi"
	case models.ClipStatusRefunded:
		return "↩️ Qaytarildi"
	default:
		return status
	}
}

// branchAccessOK — operator/admin o'z filialiga kirish huquqini tekshiradi
func branchAccessOK(user *models.User, branchID int64) bool {
	if user.IsSuperadmin() {
		return true
	}
	if user.BranchID == nil {
		return false
	}
	return *user.BranchID == branchID
}
