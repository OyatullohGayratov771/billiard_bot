package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"bot-gateway/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ===================== KLIP SO'RASH (MIJOZ) =====================

// startClipRequest — klip so'rash oqimini boshlaydi
func (h *Handler) startClipRequest(bot *tgbotapi.BotAPI, chatID int64, tgID int64) {
	branches, err := h.tableSvc.GetBranches()
	if err != nil || len(branches) == 0 {
		send(bot, chatID, "❌ Filiallar topilmadi.")
		return
	}

	h.states.Set(tgID, StateClipBranch)

	text := "🎬 <b>Klip So'rash</b>\n\n" +
		"Narx: <b>10,000 so'm</b> / 1 klip\n\n" +
		"Qaysi filialda o'ynagansiz?"

	sendWithKeyboard(bot, chatID, text, clipBranchKeyboard(branches))
}

// cbClipSelectBranch — filial tanlanganda stol tanlashga o'tadi
func (h *Handler) cbClipSelectBranch(bot *tgbotapi.BotAPI, chatID int64, msgID int, tgID int64, branchIDStr string) {
	branchID, err := strconv.ParseInt(branchIDStr, 10, 64)
	if err != nil {
		return
	}

	tables, err := h.tableSvc.GetBranchTables(branchID)
	if err != nil || len(tables) == 0 {
		send(bot, chatID, "❌ Stollar topilmadi.")
		return
	}

	h.states.Set(tgID, StateClipTable)
	h.states.SetData(tgID, "branch_id", branchID)

	text := "🎱 Qaysi stolda o'ynagansiz?"
	kb := clipTablesKeyboard(tables, branchID)
	editMessage(bot, chatID, msgID, text, &kb)
}

// cbClipSelectTable — stol tanlanganda sana/vaqt so'raydi
func (h *Handler) cbClipSelectTable(bot *tgbotapi.BotAPI, chatID int64, msgID int, tgID int64, tableIDStr string) {
	tableID, err := strconv.ParseInt(tableIDStr, 10, 64)
	if err != nil {
		return
	}

	h.states.Set(tgID, StateClipDate)
	h.states.SetData(tgID, "table_id", tableID)

	editMessage(bot, chatID, msgID,
		"📅 O'ynaganingiz <b>sana va vaqtini</b> kiriting.\n\n"+
			"Format: <code>DD.MM.YYYY HH:MM</code>\n"+
			"Misol: <code>25.03.2026 14:30</code>", nil)
}

// cbClipSelectDuration — davomiylik tanlash
func (h *Handler) cbClipSelectDuration(bot *tgbotapi.BotAPI, chatID int64, msgID int, tgID int64, durStr string) {
	dur, err := strconv.Atoi(durStr)
	if err != nil {
		return
	}

	h.states.SetData(tgID, "duration", dur)

	// Barcha ma'lumotlarni ko'rsatish va tasdiqlash
	branchID, _ := h.states.GetInt64(tgID, "branch_id")
	tableID, _ := h.states.GetInt64(tgID, "table_id")
	reqTimeStr, _ := h.states.GetString(tgID, "req_time")

	branch, _ := h.branchRepo.GetByID(branchID)
	table, _ := h.tableRepo.GetByID(tableID)

	branchName := fmt.Sprintf("ID:%d", branchID)
	if branch != nil {
		branchName = branch.Name
	}
	tableNum := int64(0)
	if table != nil {
		tableNum = int64(table.TableNum)
	}

	durText := fmt.Sprintf("%d soniya", dur)
	if dur >= 60 {
		durText = fmt.Sprintf("%d daqiqa", dur/60)
	}

	text := fmt.Sprintf(
		"🎬 <b>Klip so'rov tasdiqlash</b>\n\n"+
			"🏢 Filial: %s\n"+
			"🎱 Stol: %d\n"+
			"🕐 Vaqt: %s\n"+
			"⏱ Davomiylik: %s\n"+
			"💰 Narx: <b>10,000 so'm</b>\n\n"+
			"To'lovni amalga oshiring va screenshotni yuboring.\n"+
			"<b>Click/Payme raqami: +998 XX XXX XX XX</b>",
		branchName, tableNum, reqTimeStr, durText,
	)

	h.states.Set(tgID, StateClipPayment)

	kb := confirmKeyboard(
		fmt.Sprintf("clip_pay_confirm:%d:%d:%s:%d", branchID, tableID, reqTimeStr, dur),
		"clip_cancel",
	)
	editMessage(bot, chatID, msgID, text, &kb)
}

// cbClipBack — orqaga tugmasi
func (h *Handler) cbClipBack(bot *tgbotapi.BotAPI, chatID int64, msgID int, tgID int64, step string) {
	switch step {
	case "branch":
		h.states.Set(tgID, StateClipBranch)
		branches, _ := h.tableSvc.GetBranches()
		text := "🎬 <b>Klip So'rash</b>\n\nQaysi filialda o'ynagansiz?"
		kb := clipBranchKeyboard(branches)
		editMessage(bot, chatID, msgID, text, &kb)
	case "table":
		branchID, _ := h.states.GetInt64(tgID, "branch_id")
		h.states.Set(tgID, StateClipTable)
		tables, _ := h.tableSvc.GetBranchTables(branchID)
		kb := clipTablesKeyboard(tables, branchID)
		editMessage(bot, chatID, msgID, "🎱 Qaysi stolda o'ynagansiz?", &kb)
	}
}

// ===================== FSM — STATE INPUT HANDLER =====================

func (h *Handler) handleStateInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, user *models.User, state *UserState) {
	tgID := msg.From.ID
	chatID := msg.Chat.ID

	switch state.State {

	// --- Sessiya boshlash: mijoz ismi ---
	case StateSessionClientName:
		clientName := strings.TrimSpace(msg.Text)
		if clientName == "" {
			send(bot, chatID, "⚠️ Ism bo'sh bo'lmasin. Qayta kiriting:")
			return
		}

		tableID, ok := h.states.GetInt64(tgID, "table_id")
		if !ok {
			h.states.Clear(tgID)
			send(bot, chatID, "❌ Xatolik. Qayta urinib ko'ring.")
			return
		}

		session, err := h.tableSvc.StartSession(user, tableID, clientName)
		if err != nil {
			h.states.Clear(tgID)
			send(bot, chatID, fmt.Sprintf("❌ %v", err))
			return
		}

		h.states.Clear(tgID)
		h.logAction(user, "start_session",
			fmt.Sprintf("table:%d client:%s session:%d", tableID, clientName, session.ID))

		send(bot, chatID, fmt.Sprintf(
			"✅ <b>Sessiya boshlandi!</b>\n\n"+
				"👤 Mijoz: %s\n"+
				"🕐 Boshlanish: %s\n\n"+
				"Stol tugmasi orqali yakunlash mumkin.",
			clientName, session.StartedAt.Format("15:04"),
		))

	// --- Klip so'rash: sana va vaqt ---
	case StateClipDate:
		input := strings.TrimSpace(msg.Text)
		t, err := time.Parse("02.01.2006 15:04", input)
		if err != nil {
			send(bot, chatID,
				"⚠️ Format noto'g'ri. Qayta kiriting:\n<code>DD.MM.YYYY HH:MM</code>\nMisol: <code>25.03.2026 14:30</code>")
			return
		}

		h.states.SetData(tgID, "req_time", t.Format("02.01.2006 15:04"))
		h.states.Set(tgID, StateClipDuration)

		kb := clipDurationKeyboard()
		sendWithKeyboard(bot, chatID, "⏱ Klip davomiyligini tanlang:", kb)

	// --- Klip to'lov: screenshot ---
	case StateClipPayment:
		if msg.Photo == nil || len(msg.Photo) == 0 {
			send(bot, chatID, "📸 Iltimos, to'lov screenshotini rasm sifatida yuboring.")
			return
		}

		// Eng katta rasmni ol
		photo := msg.Photo[len(msg.Photo)-1]
		fileID := photo.FileID

		// Klip so'rovini yaratish
		branchID, _ := h.states.GetInt64(tgID, "branch_id")
		tableID, _ := h.states.GetInt64(tgID, "table_id")
		reqTimeStr, _ := h.states.GetString(tgID, "req_time")
		durVal, _ := h.states.GetData(tgID, "duration")
		dur := 60
		if d, ok := durVal.(int); ok {
			dur = d
		}

		reqTime, _ := time.Parse("02.01.2006 15:04", reqTimeStr)

		cr, err := h.clipSvc.CreateRequest(models.ClipRequestInput{
			ClientTgID:    tgID,
			ClientName:    user.DisplayName(),
			BranchID:      branchID,
			TableID:       tableID,
			RequestedTime: reqTime,
			DurationSec:   dur,
		})
		if err != nil {
			h.states.Clear(tgID)
			send(bot, chatID, fmt.Sprintf("❌ %v", err))
			return
		}

		// Screenshot'ni to'lov sifatida saqlash
		_ = h.clipRepo.CreatePayment(cr.ID, fileID)

		h.states.Clear(tgID)

		send(bot, chatID, fmt.Sprintf(
			"✅ <b>So'rovingiz qabul qilindi!</b>\n\n"+
				"📋 Buyurtma #%d\n"+
				"⏳ Admin to'lovni tekshirib, klipni yuboradi.\n\n"+
				"Odatda 1-2 soat ichida yuboriladi.",
			cr.ID,
		))

		// Adminlarga xabar
		h.notifyAdminsNewClip(bot, cr)

	// --- Xodim qo'shish: telegram ID ---
	case StateAddStaffID:
		targetIDStr := strings.TrimSpace(msg.Text)
		targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
		if err != nil {
			send(bot, chatID, "⚠️ Noto'g'ri ID. Faqat raqam kiriting:")
			return
		}

		h.states.Clear(tgID)
		kb := staffRoleKeyboard(targetID)
		sendWithKeyboard(bot, chatID,
			fmt.Sprintf("👤 ID <code>%d</code> uchun rol tanlang:", targetID), kb)
	}
}

// ===================== MENING BUYURTMALARIM =====================

func (h *Handler) showMyClips(bot *tgbotapi.BotAPI, chatID int64, tgID int64) {
	clips, err := h.clipSvc.ListByClient(tgID)
	if err != nil {
		send(bot, chatID, "❌ Xatolik.")
		return
	}

	if len(clips) == 0 {
		send(bot, chatID, "📋 Sizda hali buyurtmalar yo'q.\n\n🎬 Klip so'rash tugmasini bosing.")
		return
	}

	var sb strings.Builder
	sb.WriteString("📋 <b>Mening buyurtmalarim</b>\n\n")

	for _, c := range clips {
		statusIcon := "⏳"
		switch c.Status {
		case models.ClipStatusPaid:
			statusIcon = "💰"
		case models.ClipStatusProcessing:
			statusIcon = "⚙️"
		case models.ClipStatusDone:
			statusIcon = "✅"
		case models.ClipStatusFailed:
			statusIcon = "❌"
		case models.ClipStatusRefunded:
			statusIcon = "↩️"
		}

		sb.WriteString(fmt.Sprintf(
			"%s <b>#%d</b> — %s %d-stol\n   📅 %s | %ds\n\n",
			statusIcon, c.ID, c.BranchName, c.TableNum,
			c.RequestedTime.Format("02.01.2006 15:04"),
			c.DurationSec,
		))
	}

	send(bot, chatID, sb.String())
}

// ===================== ADMINLARGA XABAR =====================

func (h *Handler) notifyAdminsNewClip(bot *tgbotapi.BotAPI, cr *models.ClipRequest) {
	admins, err := h.userRepo.ListByRole(models.RoleSuperadmin)
	if err != nil {
		return
	}
	admins2, _ := h.userRepo.ListByRole(models.RoleAdmin)
	admins = append(admins, admins2...)

	text := fmt.Sprintf(
		"🔔 <b>Yangi klip so'rovi!</b>\n\n"+
			"📋 #%d\n"+
			"👤 Mijoz: %s\n"+
			"🏢 Filial: %s | 🎱 Stol: %d\n"+
			"🕐 Vaqt: %s\n"+
			"⏱ Davomiylik: %d soniya",
		cr.ID, cr.ClientName, cr.BranchName, cr.TableNum,
		cr.RequestedTime.Format("02.01.2006 15:04"),
		cr.DurationSec,
	)

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"📋 Ko'rish", fmt.Sprintf("clip_detail:%d", cr.ID),
			),
		),
	)

	for _, admin := range admins {
		msg := tgbotapi.NewMessage(admin.TelegramID, text)
		msg.ParseMode = "HTML"
		msg.ReplyMarkup = kb
		_, _ = bot.Send(msg)
	}
}
