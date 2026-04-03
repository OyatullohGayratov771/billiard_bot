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

// cbClipSelectTable — stol tanlanganda boshlanish vaqtini so'raydi
func (h *Handler) cbClipSelectTable(bot *tgbotapi.BotAPI, chatID int64, msgID int, tgID int64, tableIDStr string) {
	tableID, err := strconv.ParseInt(tableIDStr, 10, 64)
	if err != nil {
		return
	}

	h.states.Set(tgID, StateClipStartTime)
	h.states.SetData(tgID, "table_id", tableID)

	editMessage(bot, chatID, msgID,
		"🕐 <b>Boshlanish vaqtini</b> kiriting.\n\n"+
			"Format: <code>DD.MM.YYYY HH:MM</code>\n"+
			"Misol: <code>01.04.2026 14:30</code>\n\n"+
			"⚠️ Eng ko'pi bilan 6 kun oldingi vaqt kiritish mumkin.", nil)
}

// cbClipPayConfirm — mijoz tasdiqladi, endi screenshot so'raymiz
func (h *Handler) cbClipPayConfirm(bot *tgbotapi.BotAPI, chatID int64, msgID int, tgID int64) {
	h.states.Set(tgID, StateClipPayment)
	editMessage(bot, chatID, msgID,
		"💳 <b>To'lov</b>\n\n"+
			"Click yoki Payme orqali <b>10,000 so'm</b> to'lang:\n"+
			"<b>+998 XX XXX XX XX</b>\n\n"+
			"📸 To'lovdan so'ng <b>screenshot rasmini yuboring</b>", nil)
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

	// --- Klip so'rash: boshlanish vaqti ---
	case StateClipStartTime:
		input := strings.TrimSpace(msg.Text)
		t, err := time.Parse("02.01.2006 15:04", input)
		if err != nil {
			send(bot, chatID,
				"⚠️ Format noto'g'ri. Qayta kiriting:\n<code>DD.MM.YYYY HH:MM</code>\nMisol: <code>01.04.2026 14:30</code>")
			return
		}
		// Max 6 kun oldin
		if t.Before(time.Now().Add(-6 * 24 * time.Hour)) {
			send(bot, chatID, "⚠️ Eng ko'pi bilan <b>6 kun oldingi</b> vaqt kiritish mumkin.")
			return
		}
		// Kelajak bo'lmasin
		if t.After(time.Now()) {
			send(bot, chatID, "⚠️ Kelajak vaqt kiritib bo'lmaydi.")
			return
		}
		h.states.SetData(tgID, "start_time", t.Format("02.01.2006 15:04"))
		h.states.Set(tgID, StateClipEndTime)
		send(bot, chatID,
			"🕑 <b>Tugash vaqtini</b> kiriting.\n\n"+
				"Format: <code>DD.MM.YYYY HH:MM</code>\n"+
				"Misol: <code>01.04.2026 14:40</code>\n\n"+
				"⚠️ Boshlanishdan maksimum <b>10 daqiqa</b> keyin bo'lishi kerak.")

	// --- Klip so'rash: tugash vaqti ---
	case StateClipEndTime:
		input := strings.TrimSpace(msg.Text)
		endT, err := time.Parse("02.01.2006 15:04", input)
		if err != nil {
			send(bot, chatID,
				"⚠️ Format noto'g'ri. Qayta kiriting:\n<code>DD.MM.YYYY HH:MM</code>")
			return
		}
		startStr, _ := h.states.GetString(tgID, "start_time")
		startT, _ := time.Parse("02.01.2006 15:04", startStr)

		dur := endT.Sub(startT)
		if dur <= 0 {
			send(bot, chatID, "⚠️ Tugash vaqti boshlanishdan keyin bo'lishi kerak.")
			return
		}
		if dur > 10*time.Minute {
			send(bot, chatID, "⚠️ Maksimal davomiylik — <b>10 daqiqa</b>.\nQayta kiriting:")
			return
		}

		h.states.SetData(tgID, "end_time", endT.Format("02.01.2006 15:04"))

		// Tasdiqlash xabari
		branchID, _ := h.states.GetInt64(tgID, "branch_id")
		tableID, _ := h.states.GetInt64(tgID, "table_id")
		branch, _ := h.tableSvc.GetBranchByID(branchID)
		table, _ := h.tableSvc.GetTable(tableID)

		branchName := fmt.Sprintf("ID:%d", branchID)
		if branch != nil {
			branchName = branch.Name
		}
		tableNum := int64(0)
		if table != nil {
			tableNum = int64(table.TableNum)
		}

		mins := int(dur.Minutes())
		secs := int(dur.Seconds()) % 60
		durText := fmt.Sprintf("%d daqiqa", mins)
		if secs > 0 {
			durText = fmt.Sprintf("%d daqiqa %d soniya", mins, secs)
		}

		text := fmt.Sprintf(
			"🎬 <b>Klip so'rov tasdiqlash</b>\n\n"+
				"🏢 Filial: %s\n"+
				"🎱 Stol: %d\n"+
				"🕐 Boshlanish: %s\n"+
				"🕑 Tugash: %s\n"+
				"⏱ Davomiylik: %s\n"+
				"💰 Narx: <b>10,000 so'm</b>\n\n"+
				"Tasdiqlang va to'lov screenshotini yuboring.",
			branchName, tableNum, startStr, endT.Format("02.01.2006 15:04"), durText,
		)
		kb := confirmKeyboard("clip_pay_confirm", "clip_cancel")
		sendWithKeyboard(bot, chatID, text, kb)

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
		startStr, _ := h.states.GetString(tgID, "start_time")
		endStr, _ := h.states.GetString(tgID, "end_time")

		startTime, _ := time.Parse("02.01.2006 15:04", startStr)
		endTime, _ := time.Parse("02.01.2006 15:04", endStr)

		cr, err := h.clipSvc.CreateRequest(models.ClipRequestInput{
			ClientTgID: tgID,
			ClientName: user.DisplayName(),
			BranchID:   branchID,
			TableID:    tableID,
			StartTime:  startTime,
			EndTime:    endTime,
		})
		if err != nil {
			h.states.Clear(tgID)
			send(bot, chatID, fmt.Sprintf("❌ %v", err))
			return
		}

		// Screenshot'ni to'lov sifatida saqlash
		_ = h.clipSvc.CreatePayment(cr.ID, fileID)

		h.states.Clear(tgID)

		send(bot, chatID, fmt.Sprintf(
			"✅ <b>So'rovingiz qabul qilindi!</b>\n\n"+
				"📋 Buyurtma #%d\n"+
				"⏳ Admin to'lovni tekshirib, klipni yuboradi.\n\n"+
				"Odatda 1-2 soat ichida yuboriladi.",
			cr.ID,
		))

		// Adminlarga xabar (screenshot bilan)
		h.notifyAdminsNewClip(bot, cr, fileID)

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

	// --- NVR sozlash qadamlari ---
	case StateNVRIP, StateNVRPort, StateNVRUser, StateNVRPass:
		h.handleNVRInput(bot, msg, state)

	// --- Stol RTSP URL ---
	case StateTableRTSP:
		h.handleTableRTSPInput(bot, msg, state)
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
			"%s <b>#%d</b> — %s %d-stol\n   📅 %s — %s\n\n",
			statusIcon, c.ID, c.BranchName, c.TableNum,
			c.StartTime.Format("02.01.2006 15:04"),
			c.EndTime.Format("15:04"),
		))
	}

	send(bot, chatID, sb.String())
}

// ===================== ADMINLARGA XABAR =====================

func (h *Handler) notifyAdminsNewClip(bot *tgbotapi.BotAPI, cr *models.ClipRequest, screenshotFileID string) {
	admins, err := h.userSvc.ListByRole(models.RoleSuperadmin)
	if err != nil {
		return
	}
	admins2, _ := h.userSvc.ListByRole(models.RoleAdmin)
	admins = append(admins, admins2...)

	caption := fmt.Sprintf(
		"🔔 <b>Yangi klip so'rovi!</b>\n\n"+
			"📋 #%d\n"+
			"👤 Mijoz: %s\n"+
			"🏢 Filial: %s | 🎱 Stol: %d\n"+
			"🕐 Boshlanish: %s\n"+
			"🕑 Tugash: %s",
		cr.ID, cr.ClientName, cr.BranchName, cr.TableNum,
		cr.StartTime.Format("02.01.2006 15:04"),
		cr.EndTime.Format("02.01.2006 15:04"),
	)

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"📋 Ko'rish", fmt.Sprintf("clip_detail:%d", cr.ID),
			),
		),
	)

	for _, admin := range admins {
		if screenshotFileID != "" {
			photoMsg := tgbotapi.NewPhoto(admin.TelegramID, tgbotapi.FileID(screenshotFileID))
			photoMsg.Caption = caption
			photoMsg.ParseMode = "HTML"
			photoMsg.ReplyMarkup = kb
			if _, err := bot.Send(photoMsg); err != nil {
				msg := tgbotapi.NewMessage(admin.TelegramID, caption)
				msg.ParseMode = "HTML"
				msg.ReplyMarkup = kb
				_, _ = bot.Send(msg)
			}
		} else {
			msg := tgbotapi.NewMessage(admin.TelegramID, caption)
			msg.ParseMode = "HTML"
			msg.ReplyMarkup = kb
			_, _ = bot.Send(msg)
		}
	}
}
