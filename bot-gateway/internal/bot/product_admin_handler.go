package bot

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"bot-gateway/internal/client"
	"bot-gateway/internal/config"
	"bot-gateway/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ===================== DO'KON — MAHSULOT BOSHQARUVI (admin) =====================

func (h *Handler) showProductMenu(bot *tgbotapi.BotAPI, chatID int64, user *models.User) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	if h.productSvc == nil {
		send(bot, chatID, "❌ Do'kon xizmati ulanmagan.")
		return
	}
	list, err := h.productSvc.List()
	if err != nil {
		send(bot, chatID, "❌ Mahsulotlarni yuklab bo'lmadi.")
		return
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("🛒 <b>Do'kon</b> — %d ta mahsulot\n", len(list)))
	if len(list) == 0 {
		b.WriteString("\nHali mahsulot yo'q. <b>➕ Yangi mahsulot</b> bilan qo'shing.")
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, p := range list {
		icon := p.Emoji
		if p.ImageURL != "" {
			icon = "🖼"
		}
		stock := "✅"
		if !p.InStock {
			stock = "🚫"
		}
		label := fmt.Sprintf("%s %s · %s so'm  %s", icon, trim(p.Name, 22), fmtSum(p.Price), stock)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("prod_del:%d", p.ID)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("➕ Yangi mahsulot", "prod_add"),
	))
	if len(list) > 0 {
		b.WriteString("\n<i>O'chirish uchun mahsulot ustiga bosing.</i>")
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendWithKeyboard(bot, chatID, b.String(), kb)
}

// prod_add — yangi mahsulot qo'shishni boshlaydi
func (h *Handler) cbProductAdd(bot *tgbotapi.BotAPI, chatID, tgID int64, user *models.User) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	h.states.Clear(tgID)
	h.states.Set(tgID, StateProdName)
	send(bot, chatID, "🛒 <b>Yangi mahsulot</b>\n\n1️⃣ Mahsulot <b>nomini</b> kiriting:\n\n<i>(Bekor qilish — /cancel)</i>")
}

// prod_del — o'chirishni so'raydi (tasdiq)
func (h *Handler) cbProductDelAsk(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, idStr string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	id := mustParseInt64(idStr)
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗑 Ha, o'chirish", fmt.Sprintf("prod_delok:%d", id)),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Yo'q", "prod_menu"),
		),
	)
	editMessage(bot, chatID, msgID, "❓ Ushbu mahsulot o'chirilsinmi? Rasmi ham o'chadi.", &kb)
}

// prod_delok — o'chiradi
func (h *Handler) cbProductDelOK(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, idStr string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	id := mustParseInt64(idStr)
	if err := h.productSvc.Delete(id); err != nil {
		send(bot, chatID, "❌ O'chirib bo'lmadi.")
		return
	}
	deleteMessage(bot, chatID, msgID)
	send(bot, chatID, "🗑 Mahsulot o'chirildi.")
	h.showProductMenu(bot, chatID, user)
}

// handleProductInput — mahsulot qo'shish FSM oqimi
func (h *Handler) handleProductInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, user *models.User, state *UserState) {
	tgID := msg.From.ID
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	if text == "/cancel" {
		h.states.Clear(tgID)
		sendWithKeyboard(bot, chatID, "✅ Bekor qilindi.", mainMenuKeyboard(user))
		return
	}

	switch state.State {
	case StateProdName:
		name := safeText(text, 80)
		if name == "" {
			send(bot, chatID, "❗ Nom bo'sh bo'lmasin. Qayta kiriting:")
			return
		}
		h.states.SetData(tgID, "p_name", name)
		h.states.Set(tgID, StateProdCat)
		send(bot, chatID, "2️⃣ <b>Kategoriya</b>ni kiriting (masalan: Kiy, Shar, Aksessuar)\n\n<i>O'tkazib yuborish — /skip</i>")

	case StateProdCat:
		cat := ""
		if text != "/skip" {
			cat = safeText(text, 40)
		}
		h.states.SetData(tgID, "p_cat", cat)
		h.states.Set(tgID, StateProdPrice)
		send(bot, chatID, "3️⃣ <b>Narx</b>ni kiriting (so'm, faqat raqam — masalan 250000)\n\n<i>Narxsiz — /skip</i>")

	case StateProdPrice:
		var price int64
		if text != "/skip" {
			clean := strings.Map(func(r rune) rune {
				if r >= '0' && r <= '9' {
					return r
				}
				return -1
			}, text)
			price, _ = strconv.ParseInt(clean, 10, 64)
			if price == 0 && clean == "" {
				send(bot, chatID, "❗ Faqat raqam kiriting yoki /skip:")
				return
			}
		}
		h.states.SetData(tgID, "p_price", price)
		h.states.Set(tgID, StateProdDesc)
		send(bot, chatID, "4️⃣ <b>Tavsif</b> kiriting (masalan: Carbon shaft · 19oz)\n\n<i>Tavsifsiz — /skip</i>")

	case StateProdDesc:
		desc := ""
		if text != "/skip" {
			desc = safeText(text, 200)
		}
		h.states.SetData(tgID, "p_desc", desc)
		h.states.Set(tgID, StateProdPhoto)
		send(bot, chatID, "5️⃣ Mahsulot <b>rasmini</b> yuboring 📷\n\n<i>Rasmsiz (emoji bilan) — /skip</i>")

	case StateProdPhoto:
		imageURL := ""
		if msg.Photo != nil && len(msg.Photo) > 0 {
			photo := msg.Photo[len(msg.Photo)-1]
			url, err := h.downloadAndUploadPhoto(bot, photo.FileID)
			if err != nil {
				send(bot, chatID, "❌ Rasmni yuklab bo'lmadi. Qayta yuboring yoki /skip:")
				return
			}
			imageURL = url
		} else if text != "/skip" {
			send(bot, chatID, "📷 Iltimos, rasm yuboring yoki /skip bosing.")
			return
		}
		h.finishProductCreate(bot, chatID, tgID, user, imageURL)
	}
}

func (h *Handler) finishProductCreate(bot *tgbotapi.BotAPI, chatID, tgID int64, user *models.User, imageURL string) {
	name, _ := h.states.GetString(tgID, "p_name")
	cat, _ := h.states.GetString(tgID, "p_cat")
	desc, _ := h.states.GetString(tgID, "p_desc")
	price, _ := h.states.GetInt64(tgID, "p_price")
	h.states.Clear(tgID)

	_, err := h.productSvc.Create(client.Product{
		Name:        name,
		Category:    cat,
		Description: desc,
		Price:       price,
		Emoji:       "🎱",
		ImageURL:    imageURL,
		InStock:     true,
	})
	if err != nil {
		sendWithKeyboard(bot, chatID, "❌ Saqlashda xatolik: "+err.Error(), mainMenuKeyboard(user))
		return
	}
	sendWithKeyboard(bot, chatID, fmt.Sprintf("✅ <b>%s</b> qo'shildi!\nMijozlar uni saytda va botda ko'radi.", esc(name)), mainMenuKeyboard(user))
	h.showProductMenu(bot, chatID, user)
}

// downloadAndUploadPhoto — Telegram rasmini yuklab olib, web-service ga yuboradi.
func (h *Handler) downloadAndUploadPhoto(bot *tgbotapi.BotAPI, fileID string) (string, error) {
	dl, err := bot.GetFileDirectURL(fileID)
	if err != nil {
		return "", err
	}
	resp, err := http.Get(dl)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 12<<20))
	if err != nil {
		return "", err
	}
	return h.productSvc.UploadImage(data, "photo.jpg")
}

// ===================== DO'KON — MIJOZLAR UCHUN =====================

// showClientShop — mijozga sotuvdagi mahsulotlar ro'yxatini ko'rsatadi.
func (h *Handler) showClientShop(bot *tgbotapi.BotAPI, chatID int64) {
	if h.productSvc == nil {
		send(bot, chatID, "🛒 Do'kon vaqtincha ishlamayapti. Keyinroq urinib ko'ring.")
		return
	}
	list, err := h.productSvc.List()
	if err != nil {
		send(bot, chatID, "❌ Mahsulotlarni yuklab bo'lmadi. Keyinroq urinib ko'ring.")
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	count := 0
	for _, p := range list {
		if !p.InStock {
			continue
		}
		count++
		label := fmt.Sprintf("%s %s", p.Emoji, trim(p.Name, 26))
		if p.Price > 0 {
			label = fmt.Sprintf("%s %s · %s so'm", p.Emoji, trim(p.Name, 20), fmtSum(p.Price))
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("shop_item:%d", p.ID)),
		))
	}
	if count == 0 {
		send(bot, chatID, "🛒 <b>Do'kon</b>\n\nHozircha mahsulotlar yo'q — tez orada yangilari qo'shiladi! 🎱")
		return
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Yopish", "client_menu_close"),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendWithKeyboard(bot, chatID,
		"🛒 <b>Billiard King — Do'kon</b>\n\nProfessional billiard uskunalari.\nMahsulotni ko'rish uchun tanlang 👇", kb)
}

// cbShopItem — mahsulot tafsiloti (rasmi bo'lsa rasm bilan).
func (h *Handler) cbShopItem(bot *tgbotapi.BotAPI, chatID int64, msgID int, idStr string) {
	id := mustParseInt64(idStr)
	list, err := h.productSvc.List()
	if err != nil {
		send(bot, chatID, "❌ Xatolik. Keyinroq urinib ko'ring.")
		return
	}
	var p *client.Product
	for _, x := range list {
		if x.ID == id && x.InStock {
			p = x
			break
		}
	}
	if p == nil {
		send(bot, chatID, "❌ Mahsulot topilmadi yoki sotuvda emas.")
		return
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s <b>%s</b>\n", p.Emoji, esc(p.Name)))
	if p.Category != "" {
		b.WriteString(fmt.Sprintf("🏷 %s\n", esc(p.Category)))
	}
	if p.Description != "" {
		b.WriteString(fmt.Sprintf("\n%s\n", esc(p.Description)))
	}
	if p.Price > 0 {
		b.WriteString(fmt.Sprintf("\n💰 Narx: <b>%s so'm</b>\n", fmtSum(p.Price)))
	}
	b.WriteString("\n🛍 Buyurtma berish uchun tugmani bosing — admin siz bilan bog'lanadi.")

	su := strings.TrimPrefix(config.AppConfig.SupportUsername, "@")
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("🛍 Buyurtma berish", "https://t.me/"+su),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Do'kon", "shop_list"),
		),
	)

	// Rasmi bo'lsa — sayt orqali rasm bilan yuboramiz
	base := strings.TrimRight(config.AppConfig.TVBaseURL, "/")
	if p.ImageURL != "" && strings.HasPrefix(base, "http") {
		photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileURL(base+p.ImageURL))
		photo.Caption = b.String()
		photo.ParseMode = "HTML"
		photo.ReplyMarkup = kb
		if _, err := bot.Send(photo); err == nil {
			deleteMessage(bot, chatID, msgID)
			return
		}
		// Rasm yuborilmasa — matn ko'rinishiga tushamiz
	}
	editMessage(bot, chatID, msgID, b.String(), &kb)
}

// ── helpers ──

func trim(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func fmtSum(v int64) string {
	if v <= 0 {
		return "—"
	}
	s := strconv.FormatInt(v, 10)
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ' ')
		}
		out = append(out, c)
	}
	return string(out)
}
