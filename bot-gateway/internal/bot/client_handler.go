package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"bot-gateway/internal/config"
	"bot-gateway/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// clipProgress — qaysi qadamda ekanligini ko'rsatadi (1-5)
func clipProgress(step int) string {
	labels := []string{"Filial", "Stol", "Sana", "Vaqt", "Davomiylik"}
	var parts []string
	for i, l := range labels {
		switch {
		case i+1 < step:
			parts = append(parts, "✅ "+l)
		case i+1 == step:
			parts = append(parts, "▶️ <b>"+l+"</b>")
		default:
			parts = append(parts, "⬜ "+l)
		}
	}
	return strings.Join(parts, " › ")
}

// ===================== KLIP SO'RASH (MIJOZ) =====================

// startClipRequest — klip so'rash oqimini boshlaydi
func (h *Handler) startClipRequest(bot *tgbotapi.BotAPI, chatID int64, tgID int64) {
	branches, err := h.tableSvc.GetBranches()
	if err != nil || len(branches) == 0 {
		send(bot, chatID, "❌ Filiallar topilmadi.")
		return
	}

	h.states.Set(tgID, StateClipBranch)

	text := "🎬 <b>Klip So'rash</b>\n" +
		clipProgress(1) + "\n\n" +
		"💰 Narx: <b>10,000 so'm</b> / klip\n\n" +
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

	text := "🎬 <b>Klip So'rash</b>\n" + clipProgress(2) + "\n\n🎱 Qaysi stolda o'ynagansiz?"
	kb := clipTablesKeyboard(tables, branchID)
	editMessage(bot, chatID, msgID, text, &kb)
}

// cbClipSelectTable — stol tanlanganda kun tanlash ko'rsatadi
func (h *Handler) cbClipSelectTable(bot *tgbotapi.BotAPI, chatID int64, msgID int, tgID int64, tableIDStr string) {
	tableID, err := strconv.ParseInt(tableIDStr, 10, 64)
	if err != nil {
		return
	}
	h.states.SetData(tgID, "table_id", tableID)

	kb := clipDateKeyboard()
	editMessage(bot, chatID, msgID,
		"🎬 <b>Klip So'rash</b>\n"+clipProgress(3)+"\n\n📅 Klip olmoqchi bo'lgan kunni tanlang:", &kb)
}

// cbClipSelectDate — kun tanlangandan keyin soat ko'rsatadi
func (h *Handler) cbClipSelectDate(bot *tgbotapi.BotAPI, chatID int64, msgID int, tgID int64, dateStr string) {
	day, err := time.Parse("02.01.2006", dateStr)
	if err != nil || day.Before(time.Now().Add(-6*24*time.Hour)) {
		send(bot, chatID, "⚠️ Noto'g'ri sana tanlandi.")
		return
	}
	h.states.SetData(tgID, "date", dateStr)

	isToday := day.Format("02.01.2006") == time.Now().Format("02.01.2006")
	kb := clipHourKeyboard(isToday)
	editMessage(bot, chatID, msgID,
		"🎬 <b>Klip So'rash</b>\n"+clipProgress(4)+"\n\n"+
			fmt.Sprintf("📅 <b>%s</b>\n🕐 Soatni tanlang:", dateStr), &kb)
}

// cbClipSelectHour — soat tanlangandan keyin daqiqa ko'rsatadi
func (h *Handler) cbClipSelectHour(bot *tgbotapi.BotAPI, chatID int64, msgID int, tgID int64, hourStr string) {
	hour, err := strconv.Atoi(hourStr)
	if err != nil {
		return
	}
	h.states.SetData(tgID, "hour", hour)

	dateStr, _ := h.states.GetString(tgID, "date")
	day, _ := time.Parse("02.01.2006", dateStr)
	isToday := day.Format("02.01.2006") == time.Now().Format("02.01.2006")
	isCurrentHour := isToday && hour == time.Now().Hour()
	kb := clipMinuteKeyboard(hour, isCurrentHour)
	editMessage(bot, chatID, msgID,
		"🎬 <b>Klip So'rash</b>\n"+clipProgress(4)+"\n\n"+
			fmt.Sprintf("📅 <b>%s</b>  🕐 <b>%02d:__</b>\n⏱ Daqiqani tanlang:", dateStr, hour), &kb)
}

// cbClipSelectMinute — daqiqa tanlangandan keyin davomiylik ko'rsatadi
func (h *Handler) cbClipSelectMinute(bot *tgbotapi.BotAPI, chatID int64, msgID int, tgID int64, minStr string) {
	minute, err := strconv.Atoi(minStr)
	if err != nil {
		return
	}
	h.states.SetData(tgID, "minute", minute)

	dateStr, _ := h.states.GetString(tgID, "date")
	hourVal, _ := h.states.GetData(tgID, "hour")
	hour, _ := hourVal.(int)

	// Kelajak vaqt tekshirish
	startTime, _ := time.ParseInLocation("02.01.2006 15:04",
		fmt.Sprintf("%s %02d:%02d", dateStr, hour, minute), time.Local)
	if startTime.After(time.Now()) {
		dateKb := clipDateKeyboard()
		editMessage(bot, chatID, msgID,
			"⚠️ Kelajak vaqtni tanlab bo'lmaydi. Iltimos boshqa vaqt tanlang.\n\n📅 Kunni qaytadan tanlang:",
			&dateKb)
		h.states.SetData(tgID, "date", "")
		return
	}

	kb := clipDurationKeyboard()
	editMessage(bot, chatID, msgID,
		"🎬 <b>Klip So'rash</b>\n"+clipProgress(5)+"\n\n"+
			fmt.Sprintf("📅 <b>%s</b>  🕐 <b>%02d:%02d</b>\n⏳ Klip davomiyligini tanlang:", dateStr, hour, minute), &kb)
}

// cbClipPayConfirm — mijoz tasdiqladi, endi screenshot so'raymiz
func (h *Handler) cbClipPayConfirm(bot *tgbotapi.BotAPI, chatID int64, msgID int, tgID int64) {
	h.states.Set(tgID, StateClipPayment)

	payDetails := ""
	if config.AppConfig.PaymentCard != "" {
		payDetails += "\n💳 Karta: <code>" + config.AppConfig.PaymentCard + "</code>"
	}
	if config.AppConfig.PaymentPhone != "" {
		payDetails += "\n📱 Telefon: <code>" + config.AppConfig.PaymentPhone + "</code>"
	}
	if payDetails == "" {
		payDetails = "\n<i>To'lov rekvizitlari uchun admin bilan bog'laning</i>"
	}

	editMessage(bot, chatID, msgID,
		"💳 <b>To'lov</b>\n\n"+
			"Click yoki Payme orqali <b>10,000 so'm</b> to'lang:"+
			payDetails+
			"\n\n📸 To'lovdan so'ng <b>screenshot rasmini yuboring</b>", nil)
}

// cbClipSelectDuration — davomiylik tanlangandan keyin tasdiqlash ko'rsatadi
func (h *Handler) cbClipSelectDuration(bot *tgbotapi.BotAPI, chatID int64, msgID int, tgID int64, durStr string) {
	durMin, err := strconv.Atoi(durStr)
	if err != nil {
		return
	}

	dateStr, _ := h.states.GetString(tgID, "date")
	hourVal, _ := h.states.GetData(tgID, "hour")
	minVal, _ := h.states.GetData(tgID, "minute")
	hour, _ := hourVal.(int)
	minute, _ := minVal.(int)

	startTime, _ := time.ParseInLocation("02.01.2006 15:04",
		fmt.Sprintf("%s %02d:%02d", dateStr, hour, minute), time.Local)
	endTime := startTime.Add(time.Duration(durMin) * time.Minute)

	h.states.SetData(tgID, "start_time", startTime.Format("02.01.2006 15:04"))
	h.states.SetData(tgID, "end_time", endTime.Format("02.01.2006 15:04"))

	branchID, _ := h.states.GetInt64(tgID, "branch_id")
	tableID, _ := h.states.GetInt64(tgID, "table_id")
	branch, _ := h.tableSvc.GetBranchByID(branchID)
	table, _ := h.tableSvc.GetTable(tableID)

	branchName := fmt.Sprintf("ID:%d", branchID)
	branchAddr := ""
	if branch != nil {
		branchName = branch.Name
		if branch.Address != "" {
			branchAddr = "\n📍 " + branch.Address
		}
	}
	tableNum := 0
	if table != nil {
		tableNum = table.TableNum
	}

	text := fmt.Sprintf(
		"🎬 <b>Klip so'rov tasdiqlash</b>\n"+
			"✅ ✅ ✅ ✅ ✅\n\n"+
			"🏢 Filial: <b>%s</b>%s\n"+
			"🎱 Stol: <b>%d</b>\n"+
			"🕐 Boshlanish: <b>%s</b>\n"+
			"🕑 Tugash: <b>%s</b>\n"+
			"⏱ Davomiylik: <b>%d daqiqa</b>\n"+
			"💰 Narx: <b>10,000 so'm</b>\n\n"+
			"Tasdiqlaysizmi?",
		branchName, branchAddr, tableNum,
		startTime.Format("02.01.2006 15:04"),
		endTime.Format("15:04"),
		durMin,
	)
	kb := confirmKeyboard("clip_pay_confirm", "clip_cancel")
	editMessage(bot, chatID, msgID, text, &kb)
}

// cbClipBack — orqaga tugmasi
func (h *Handler) cbClipBack(bot *tgbotapi.BotAPI, chatID int64, msgID int, tgID int64, step string) {
	switch step {
	case "branch":
		branches, _ := h.tableSvc.GetBranches()
		kb := clipBranchKeyboard(branches)
		editMessage(bot, chatID, msgID, "🎬 <b>Klip So'rash</b>\n\nQaysi filialda o'ynagansiz?", &kb)
	case "table":
		branchID, _ := h.states.GetInt64(tgID, "branch_id")
		tables, _ := h.tableSvc.GetBranchTables(branchID)
		kb := clipTablesKeyboard(tables, branchID)
		editMessage(bot, chatID, msgID, "🎱 Qaysi stolda o'ynagansiz?", &kb)
	case "date":
		kb := clipDateKeyboard()
		editMessage(bot, chatID, msgID, "📅 <b>Qaysi kuni?</b>", &kb)
	case "hour":
		dateStr, _ := h.states.GetString(tgID, "date")
		day, _ := time.Parse("02.01.2006", dateStr)
		isToday := day.Format("02.01.2006") == time.Now().Format("02.01.2006")
		kb := clipHourKeyboard(isToday)
		editMessage(bot, chatID, msgID, fmt.Sprintf("📅 %s\n\n🕐 <b>Soatni tanlang:</b>", dateStr), &kb)
	case "minute":
		dateStr, _ := h.states.GetString(tgID, "date")
		hourVal, _ := h.states.GetData(tgID, "hour")
		hour, _ := hourVal.(int)
		day, _ := time.Parse("02.01.2006", dateStr)
		isToday := day.Format("02.01.2006") == time.Now().Format("02.01.2006")
		isCurrentHour := isToday && hour == time.Now().Hour()
		kb := clipMinuteKeyboard(hour, isCurrentHour)
		editMessage(bot, chatID, msgID,
			fmt.Sprintf("📅 %s  🕐 %02d:__\n\n⏱ <b>Daqiqani tanlang:</b>", dateStr, hour), &kb)
	}
}

// ===================== FSM — STATE INPUT HANDLER =====================

func (h *Handler) handleStateInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, user *models.User, state *UserState) {
	tgID := msg.From.ID
	chatID := msg.Chat.ID

	switch state.State {

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

	// --- Admin: ruchnoy video yuborish ---
	case StateAdminUploadClip:
		h.handleAdminUploadInput(bot, msg, user)

	// --- Admin: rad etish izoh ---
	case StateAdminRejectNote:
		h.handleAdminNoteInput(bot, msg, user, false)

	// --- Admin: qaytarish izoh ---
	case StateAdminRefundNote:
		h.handleAdminNoteInput(bot, msg, user, true)

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

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, c := range clips {
		noteStr := ""
		if c.Notes != "" && (c.Status == models.ClipStatusFailed || c.Status == models.ClipStatusRefunded) {
			noteStr = "\n   📝 <i>" + c.Notes + "</i>"
		}
		sb.WriteString(fmt.Sprintf(
			"%s <b>#%d</b> — %s %d-stol\n   📅 %s — %s%s\n\n",
			statusText(c.Status), c.ID, c.BranchName, c.TableNum,
			c.StartTime.Format("02.01.2006 15:04"),
			c.EndTime.Format("15:04"),
			noteStr,
		))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("#%d — %s", c.ID, c.StartTime.Format("02.01 15:04")),
				fmt.Sprintf("my_clip_detail:%d", c.ID),
			),
		))
	}

	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendWithKeyboard(bot, chatID, sb.String(), kb)
}

// cbShowMyClipDetail — mijoz o'z buyurtmasini batafsil ko'radi
func (h *Handler) cbShowMyClipDetail(bot *tgbotapi.BotAPI, chatID int64, msgID int, tgID int64, clipIDStr string) {
	clipID := mustParseInt64(clipIDStr)
	cr, err := h.clipSvc.GetByID(clipID)
	if err != nil || cr.ClientTgID != tgID {
		send(bot, chatID, "❌ Buyurtma topilmadi.")
		return
	}

	noteStr := ""
	if cr.Notes != "" {
		noteStr = fmt.Sprintf("\n\n📝 <b>Izoh:</b> <i>%s</i>", cr.Notes)
	}

	text := fmt.Sprintf(
		"📋 <b>Buyurtma #%d</b>\n\n"+
			"🏢 Filial: %s\n"+
			"🎱 Stol: %d\n"+
			"🕐 Boshlanish: %s\n"+
			"🕑 Tugash: %s\n"+
			"📊 Holat: <b>%s</b>%s",
		cr.ID, cr.BranchName, cr.TableNum,
		cr.StartTime.Format("02.01.2006 15:04"),
		cr.EndTime.Format("15:04"),
		statusText(cr.Status), noteStr,
	)

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Orqaga", "my_clips_back"),
		),
	)
	editMessage(bot, chatID, msgID, text, &kb)
}

// ===================== ADMINLARGA XABAR =====================

func (h *Handler) notifyAdminsNewClip(bot *tgbotapi.BotAPI, cr *models.ClipRequest, screenshotFileID string) {
	admins, err := h.userSvc.ListByRole(models.RoleSuperadmin)
	if err != nil {
		return
	}
	admins2, _ := h.userSvc.ListByRole(models.RoleAdmin)
	admins = append(admins, admins2...)

	// Mijoz telefon raqamini olish
	clientPhone := "—"
	if clientUser, err := h.userSvc.GetByTelegramID(cr.ClientTgID); err == nil && clientUser.Phone != "" {
		clientPhone = clientUser.Phone
	}

	caption := fmt.Sprintf(
		"🔔 <b>Yangi klip so'rovi!</b>\n\n"+
			"📋 #%d\n"+
			"👤 Mijoz: %s\n"+
			"📱 Telefon: %s\n"+
			"🏢 Filial: %s | 🎱 Stol: %d\n"+
			"🕐 Boshlanish: %s\n"+
			"🕑 Tugash: %s",
		cr.ID, cr.ClientName, clientPhone, cr.BranchName, cr.TableNum,
		cr.StartTime.Format("02.01.2006 15:04"),
		cr.EndTime.Format("02.01.2006 15:04"),
	)

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"✅ To'lovni tasdiqlash", fmt.Sprintf("admin_confirm_pay:%d", cr.ID),
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"📋 Batafsil ko'rish", fmt.Sprintf("clip_detail:%d", cr.ID),
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
