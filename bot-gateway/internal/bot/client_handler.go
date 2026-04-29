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
		"💰 Narx: <b>5,000 so'm</b> / klip\n\n" +
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

// cbClipSelectDate — kun tanlangandan keyin vaqt spinner ko'rsatadi
func (h *Handler) cbClipSelectDate(bot *tgbotapi.BotAPI, chatID int64, msgID int, tgID int64, dateStr string) {
	day, err := time.Parse("02.01.2006", dateStr)
	// 7-kun validatsiya: UTC midnight bilan solishtirish (off-by-one bug oldini olish)
	sixDaysAgo := time.Now().UTC().AddDate(0, 0, -5).Truncate(24 * time.Hour)
	if err != nil || day.UTC().Before(sixDaysAgo) {
		send(bot, chatID, "⚠️ Noto'g'ri sana tanlandi.")
		return
	}
	h.states.SetData(tgID, "date", dateStr)

	// Default: oxirgi 5-daqiqali slot
	now := time.Now()
	isToday := day.UTC().Format("2006-01-02") == now.UTC().Format("2006-01-02")
	defHour, defMin := 18, 0
	if isToday {
		defHour = now.Hour()
		defMin = (now.Minute() / 5) * 5
		if defMin >= 5 {
			defMin -= 5
		} else if defHour > 0 {
			defHour--
			defMin = 55
		}
	}
	h.states.SetData(tgID, "cur_hour", defHour)
	h.states.SetData(tgID, "cur_min", defMin)

	kb := clipTimeSpinnerKeyboard(defHour, defMin)
	editMessage(bot, chatID, msgID,
		"🎬 <b>Klip So'rash</b>\n"+clipProgress(4)+"\n\n"+
			fmt.Sprintf("📅 <b>%s</b>\n🕐 Klip boshlanish vaqtini tanlang:", dateStr), &kb)
}

// cbClipTimeAdjust — spinner ▲/▼ bosilganda vaqtni o'zgartiradi
func (h *Handler) cbClipTimeAdjust(bot *tgbotapi.BotAPI, chatID int64, msgID int, tgID int64, adj string) {
	hourVal, _ := h.states.GetData(tgID, "cur_hour")
	minVal, _ := h.states.GetData(tgID, "cur_min")
	hour, _ := hourVal.(int)
	minute, _ := minVal.(int)

	switch adj {
	case "+h":
		hour = (hour + 1) % 24
	case "-h":
		hour = (hour - 1 + 24) % 24
	case "+m":
		minute = (minute + 1) % 60
	case "-m":
		minute = (minute - 1 + 60) % 60
	}

	h.states.SetData(tgID, "cur_hour", hour)
	h.states.SetData(tgID, "cur_min", minute)

	dateStr, _ := h.states.GetString(tgID, "date")
	kb := clipTimeSpinnerKeyboard(hour, minute)
	editMessage(bot, chatID, msgID,
		"🎬 <b>Klip So'rash</b>\n"+clipProgress(4)+"\n\n"+
			fmt.Sprintf("📅 <b>%s</b>\n🕐 Klip boshlanish vaqtini tanlang:", dateStr), &kb)
}

// cbClipTimeOK — vaqtni tasdiqlaydi, davomiylik tanlashga o'tadi
func (h *Handler) cbClipTimeOK(bot *tgbotapi.BotAPI, chatID int64, msgID int, tgID int64) {
	hourVal, _ := h.states.GetData(tgID, "cur_hour")
	minVal, _ := h.states.GetData(tgID, "cur_min")
	hour, _ := hourVal.(int)
	minute, _ := minVal.(int)
	dateStr, _ := h.states.GetString(tgID, "date")

	startTime, _ := time.ParseInLocation("02.01.2006 15:04",
		fmt.Sprintf("%s %02d:%02d", dateStr, hour, minute), time.Local)
	if startTime.After(time.Now()) {
		kb := clipTimeSpinnerKeyboard(hour, minute)
		editMessage(bot, chatID, msgID,
			"⚠️ <b>Kelajak vaqtni tanlab bo'lmaydi!</b>\n\n"+
				"🎬 <b>Klip So'rash</b>\n"+clipProgress(4)+"\n\n"+
				fmt.Sprintf("📅 <b>%s</b>\n🕐 Boshlanish vaqtini qaytadan tanlang:", dateStr), &kb)
		return
	}

	h.states.SetData(tgID, "hour", hour)
	h.states.SetData(tgID, "minute", minute)

	kb := clipDurationKeyboard()
	editMessage(bot, chatID, msgID,
		"🎬 <b>Klip So'rash</b>\n"+clipProgress(5)+"\n\n"+
			fmt.Sprintf("📅 <b>%s</b>  🕐 <b>%02d:%02d</b>\n⏳ Klip davomiyligini tanlang (max 3 daqiqa):", dateStr, hour, minute), &kb)
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
			"Click yoki Payme orqali <b>5,000 so'm</b> to'lang:"+
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

	// Tugash vaqti kelajakda bo'lmasligi kerak
	if endTime.After(time.Now()) {
		kb := clipDurationKeyboard()
		editMessage(bot, chatID, msgID, fmt.Sprintf(
			"⚠️ <b>%d daqiqali klip kelajakda tugaydi!</b>\n\n"+
				"🕐 Boshlanish: <b>%s</b>\n🕑 Tugash: <b>%s</b> (hali bo'lmagan)\n\n"+
				"Kamroq daqiqa tanlang yoki 🔙 boshqa vaqt tanlang:",
			durMin, startTime.Format("15:04"), endTime.Format("15:04"),
		), &kb)
		return
	}

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
			"💰 Narx: <b>5,000 so'm</b>\n\n"+
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
	case "time":
		dateStr, _ := h.states.GetString(tgID, "date")
		hourVal, _ := h.states.GetData(tgID, "cur_hour")
		minVal, _ := h.states.GetData(tgID, "cur_min")
		hour, _ := hourVal.(int)
		minute, _ := minVal.(int)
		kb := clipTimeSpinnerKeyboard(hour, minute)
		editMessage(bot, chatID, msgID,
			"🎬 <b>Klip So'rash</b>\n"+clipProgress(4)+"\n\n"+
				fmt.Sprintf("📅 <b>%s</b>\n🕐 Klip boshlanish vaqtini tanlang:", dateStr), &kb)
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

		// BranchName va TableNum uchun GetByID (Create JOIN maydonlarni qaytarmaydi)
		if fullCR, err2 := h.clipSvc.GetByID(cr.ID); err2 == nil {
			cr = fullCR
		}

		h.states.Clear(tgID)

		durMin := int(cr.EndTime.Sub(cr.StartTime).Minutes())
		send(bot, chatID, fmt.Sprintf(
			"✅ <b>So'rovingiz qabul qilindi!</b>\n\n"+
				"━━━━━━━━━━━━━━━━━\n"+
				"🏢 %s  •  🎱 %d-stol\n"+
				"🕐 %s – %s  <b>(%d daq)</b>\n"+
				"📅 %s\n"+
				"━━━━━━━━━━━━━━━━━\n\n"+
				"📸 To'lov screenshoti qabul qilindi\n"+
				"⏳ Admin tekshirib, klipingizni tayyorlaydi\n\n"+
				"📲 <i>Klip tayyor bo'lgach, shu yerga yuboriladi!</i>",
			cr.BranchName, cr.TableNum,
			cr.StartTime.Format("15:04"), cr.EndTime.Format("15:04"), durMin,
			cr.StartTime.Format("02.01.2006"),
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

	// --- Stol D-kanal ---
	case StateTableChannel:
		h.handleTableChannelInput(bot, msg, state)

	// --- Turnir yaratish FSM ---
	case StateTrnName:
		h.handleTrnNameInput(bot, msg)
	case StateTrnDateTime:
		h.handleTrnDateTimeInput(bot, msg)
	case StateTrnMaxPlayers:
		h.handleTrnMaxPlayersInput(bot, msg)
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
			"%s %s  %d-stol\n   🕐 %s – %s%s\n\n",
			statusText(c.Status), c.BranchName, c.TableNum,
			c.StartTime.Format("02.01.2006 15:04"),
			c.EndTime.Format("15:04"),
			noteStr,
		))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%s %d-stol • %s", c.BranchName, c.TableNum, c.StartTime.Format("02.01 15:04")),
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
		"📋 <b>Buyurtma tafsiloti</b>\n\n"+
			"🏢 Filial: %s\n"+
			"🎱 Stol: %d\n"+
			"🕐 Vaqt: %s – %s\n"+
			"📊 Holat: <b>%s</b>%s",
		cr.BranchName, cr.TableNum,
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

// ===================== AKKAUNT =====================

func (h *Handler) showMyProfile(bot *tgbotapi.BotAPI, chatID int64, user *models.User) {
	phone := user.Phone
	if phone == "" {
		phone = "📵 Ulashilmagan"
	}

	username := "—"
	if user.Username != "" {
		username = "@" + user.Username
	}

	since := "—"
	if !user.CreatedAt.IsZero() {
		since = user.CreatedAt.Format("02.01.2006")
	}

	text := fmt.Sprintf(
		"👤 <b>Mening akkaunt</b>\n\n"+
			"━━━━━━━━━━━━━━━━━━━\n"+
			"🧑 Ism: <b>%s</b>\n"+
			"🔖 Username: %s\n"+
			"📱 Telefon: <b>%s</b>\n"+
			"🆔 Telegram ID: <code>%d</code>\n"+
			"📅 A'zo bo'lgan: %s\n"+
			"━━━━━━━━━━━━━━━━━━━\n\n"+
			"❓ Savolingiz bo'lsa: %s",
		user.DisplayName(),
		username,
		phone,
		user.TelegramID,
		since,
		config.AppConfig.SupportUsername,
	)

	send(bot, chatID, text)
}

// ===================== ADMINLARGA XABAR =====================

func (h *Handler) notifyAdminsNewClip(bot *tgbotapi.BotAPI, cr *models.ClipRequest, screenshotFileID string) {
	admins, err := h.userSvc.ListByRole(models.RoleSuperadmin)
	if err != nil {
		return
	}
	admins2, _ := h.userSvc.ListByRole(models.RoleAdmin)
	admins = append(admins, admins2...)

	clientPhone := "—"
	if cu, err2 := h.userSvc.GetByTelegramID(cr.ClientTgID); err2 == nil && cu.Phone != "" {
		clientPhone = cu.Phone
	}

	durMin := int(cr.EndTime.Sub(cr.StartTime).Minutes())
	caption := fmt.Sprintf(
		"🔔 <b>Yangi klip so'rovi #%d</b>\n\n"+
			"👤 %s  •  📱 %s\n"+
			"🏢 %s  •  🎱 %d-stol\n"+
			"🕐 %s – %s  (%d daq)\n"+
			"📅 %s",
		cr.ID,
		cr.ClientName, clientPhone,
		cr.BranchName, cr.TableNum,
		cr.StartTime.Format("15:04"), cr.EndTime.Format("15:04"), durMin,
		cr.StartTime.Format("02.01.2006"),
	)

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"✅ To'lovni tasdiqlash", fmt.Sprintf("admin_confirm_pay:%d", cr.ID),
			),
			tgbotapi.NewInlineKeyboardButtonData(
				"📋 Batafsil", fmt.Sprintf("clip_detail:%d", cr.ID),
			),
		),
	)

	for _, admin := range admins {
		if screenshotFileID != "" {
			photoMsg := tgbotapi.NewPhoto(admin.TelegramID, tgbotapi.FileID(screenshotFileID))
			photoMsg.Caption = caption
			photoMsg.ParseMode = "HTML"
			photoMsg.ReplyMarkup = kb
			if _, err3 := bot.Send(photoMsg); err3 != nil {
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
