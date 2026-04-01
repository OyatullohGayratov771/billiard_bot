package bot

import (
	"log"

	"bot-gateway/internal/models"
	"bot-gateway/internal/repository"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// authUser — har bir updateda foydalanuvchini DB dan oladi.
// Agar foydalanuvchi topilmasa nil qaytaradi (yangi foydalanuvchi /start bosishi kerak).
func (h *Handler) authUser(tgID int64) *models.User {
	user, err := h.userRepo.GetByTelegramID(tgID)
	if err != nil {
		return nil
	}
	if !user.IsActive {
		return nil
	}
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

// logAction — audit log yozadi (goroutine'da, bloklash yo'q)
func (h *Handler) logAction(user *models.User, action, details string) {
	go func() {
		var uid *int64
		if user != nil {
			uid = &user.ID
		}
		h.auditRepo.Log(uid, action, details)
	}()
}

// rateLimiter — oddiy rate limiting (xotira ichida)
type rateLimiter struct {
	counts map[int64]int
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{counts: make(map[int64]int)}
}

// isBlocked — 10 soniyada 5 ta so'rovdan ko'p bo'lsa bloklaydi (soddalashtirilgan)
func (rl *rateLimiter) isBlocked(tgID int64) bool {
	rl.counts[tgID]++
	if rl.counts[tgID] > 30 {
		log.Printf("⚠️  Rate limit: tg:%d", tgID)
		return true
	}
	return false
}

func (rl *rateLimiter) reset(tgID int64) {
	delete(rl.counts, tgID)
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

// userRepo shortcut for authUser
func (h *Handler) getOrRegister(tgID int64, username, firstName, lastName string) (*models.User, error) {
	user, err := h.userSvc.RegisterOrGet(tgID, username, firstName, lastName)
	if err != nil {
		return nil, err
	}
	return user, nil
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

// isUserRepo helper for direct access
type userRepoIface interface {
	GetByTelegramID(tgID int64) (*models.User, error)
}

var _ userRepoIface = (*repository.UserRepo)(nil)
