package bot

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bot-gateway/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ===================== START =====================

func (h *Handler) cmdStart(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, user *models.User) {
	// Telefon raqami yo'q bo'lsa — oldin so'raymiz
	if user.Phone == "" {
		kb := tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButtonContact("📱 Telefon raqamni ulashish"),
			),
		)
		kb.ResizeKeyboard = true
		kb.OneTimeKeyboard = true
		sendWithKeyboard(bot, msg.Chat.ID,
			"👋 Xush kelibsiz!\n\n📱 Davom etish uchun telefon raqamingizni ulashing:", kb)
		return
	}

	h.showMainMenu(bot, msg.Chat.ID, user)
}

func (h *Handler) showMainMenu(bot *tgbotapi.BotAPI, chatID int64, user *models.User) {
	name := user.DisplayName()
	var roleText string
	switch user.Role {
	case models.RoleSuperadmin:
		roleText = "👑 Superadmin"
	case models.RoleAdmin:
		roleText = "🔧 Admin"
	case models.RoleOperator:
		roleText = "🎮 Operator"
	default:
		roleText = "👤 Mijoz"
	}

	text := fmt.Sprintf(
		"🎱 <b>Billiard Club Bot</b>\n\n"+
			"Xush kelibsiz, <b>%s</b>!\n"+
			"Sizning rolingiz: %s\n\n"+
			"Pastdagi tugmalardan foydalaning 👇",
		name, roleText,
	)

	sendWithKeyboard(bot, chatID, text, mainMenuKeyboard(user))
}

// ===================== KLIP SO'ROVLAR (ADMIN) =====================

func (h *Handler) showPendingClips(bot *tgbotapi.BotAPI, chatID int64, user *models.User) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}

	clips, err := h.clipSvc.ListPending()
	if err != nil {
		send(bot, chatID, "❌ Xatolik.")
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	var header string
	if len(clips) == 0 {
		header = "🎬 <b>Klip so'rovlar</b>\n\n✅ Faol so'rovlar yo'q."
	} else {
		header = fmt.Sprintf("🎬 <b>Klip so'rovlar</b> — <b>%d faol</b>\n\n⏳ Kutilmoqda  💰 To'langan  ⚙️ Jarayonda", len(clips))
		for _, c := range clips {
			icon := statusIcon(c.Status)
			label := fmt.Sprintf("%s #%d — %s %d-stol  %s",
				icon, c.ID, c.BranchName, c.TableNum,
				c.StartTime.Format("02.01 15:04"))
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("clip_detail:%d", c.ID)),
			))
		}
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📋 Barcha so'rovlar (tarix)", "admin_all_clips"),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendWithKeyboard(bot, chatID, header, kb)
}

func (h *Handler) showAllClips(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	clips, err := h.clipSvc.ListRecent(30)
	if err != nil {
		send(bot, chatID, "❌ Xatolik.")
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	var header string
	if len(clips) == 0 {
		header = "📋 <b>So'rovlar tarixi</b>\n\nHali birorta so'rov yo'q."
	} else {
		header = fmt.Sprintf("📋 <b>So'nggi %d ta so'rov</b>", len(clips))
		for _, c := range clips {
			icon := statusIcon(c.Status)
			label := fmt.Sprintf("%s #%d — %s %d-stol  %s",
				icon, c.ID, c.BranchName, c.TableNum,
				c.StartTime.Format("02.01 15:04"))
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("clip_detail:%d", c.ID)),
			))
		}
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Faol so'rovlar", "admin_clips_list"),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	if msgID > 0 {
		editMessage(bot, chatID, msgID, header, &kb)
	} else {
		sendWithKeyboard(bot, chatID, header, kb)
	}
}

func (h *Handler) cbShowClipDetail(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, clipIDStr string) {
	clipID := mustParseInt64(clipIDStr)
	cr, err := h.clipSvc.GetByID(clipID)
	if err != nil {
		send(bot, chatID, "❌ Buyurtma topilmadi.")
		return
	}

	clientPhone := "—"
	if cu, err2 := h.userSvc.GetByTelegramID(cr.ClientTgID); err2 == nil && cu.Phone != "" {
		clientPhone = cu.Phone
	}

	durMin := int(cr.EndTime.Sub(cr.StartTime).Minutes())
	noteStr := ""
	if cr.Notes != "" {
		noteStr = fmt.Sprintf("\n📝 <i>%s</i>", cr.Notes)
	}

	text := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━\n"+
			"🎬 <b>Klip #%d</b>  %s\n"+
			"━━━━━━━━━━━━━━━━━━━━\n"+
			"👤 %s\n"+
			"📱 %s\n"+
			"🏢 %s  •  🎱 %d-stol\n"+
			"🕐 %s – %s  <b>(%d daq)</b>\n"+
			"📅 %s\n"+
			"━━━━━━━━━━━━━━━━━━━━%s",
		cr.ID, statusText(cr.Status),
		cr.ClientName, clientPhone,
		cr.BranchName, cr.TableNum,
		cr.StartTime.Format("15:04"), cr.EndTime.Format("15:04"), durMin,
		cr.StartTime.Format("02.01.2006"),
		noteStr,
	)

	kb := clipRequestActionsKeyboard(cr.ID, cr.Status)
	sendWithKeyboard(bot, chatID, text, kb)
}

func (h *Handler) cbAdminConfirmPayment(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, clipIDStr string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	clipID := mustParseInt64(clipIDStr)
	if err := h.clipSvc.ConfirmPayment(user.ID, clipID); err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}
	h.logAction(user, "confirm_payment", fmt.Sprintf("clip:%d", clipID))
	send(bot, chatID, fmt.Sprintf("✅ Klip #%d uchun to'lov tasdiqlandi.", clipID))

	// Mijozga marketing xabar
	if cr, err := h.clipSvc.GetByID(clipID); err == nil {
		durMin := int(cr.EndTime.Sub(cr.StartTime).Minutes())
		send(bot, cr.ClientTgID, fmt.Sprintf(
			"🎬 <b>Ajoyib! Klipingiz tayyorlanmoqda!</b>\n\n"+
				"━━━━━━━━━━━━━━━━━\n"+
				"📋 Buyurtma #%d\n"+
				"🏢 %s  •  🎱 %d-stol\n"+
				"🕐 %s – %s  (%d daq)\n"+
				"📅 %s\n"+
				"━━━━━━━━━━━━━━━━━\n\n"+
				"⏳ <b>10–15 daqiqa</b> ichida klipingiz yuboriladi!\n\n"+
				"🎯 <i>O'yiningizning eng yaxshi lahzalari saqlanmoqda...</i>\n"+
				"🎱 <i>Billiard Club — har bir zarbda g'alaba!</i> 🏆",
			cr.ID, cr.BranchName, cr.TableNum,
			cr.StartTime.Format("15:04"), cr.EndTime.Format("15:04"), durMin,
			cr.StartTime.Format("02.01.2006"),
		))
	}

	h.cbShowClipDetail(bot, chatID, msgID, user, clipIDStr)
}

// cbRecordClip — NVR dan klip yozishni boshlaydi va bot mijozga yuboradi
func (h *Handler) cbRecordClip(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, clipIDStr string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	clipID := mustParseInt64(clipIDStr)

	cr, err := h.clipSvc.GetByID(clipID)
	if err != nil {
		send(bot, chatID, "❌ Buyurtma topilmadi.")
		return
	}

	if err := h.clipSvc.TriggerRecording(clipID); err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}

	editMessage(bot, chatID, msgID,
		fmt.Sprintf("⏳ <b>Klip #%d yozilmoqda...</b>\n\n%s — %s\nTayyor bo'lganda avtomatik yuboriladi.",
			clipID, cr.StartTime.Format("15:04"), cr.EndTime.Format("15:04")), nil)

	// Fon goroutine — klip tayyor bo'lganda mijozga yuboradi
	go func() {
		maxPolls := 90 // 90 × 10s = 15 daqiqa

		for i := 0; i < maxPolls; i++ {
			time.Sleep(10 * time.Second)

			cur, err := h.clipSvc.GetByID(clipID)
			if err != nil {
				return
			}

			if cur.Status == models.ClipStatusFailed {
				errDetail := cur.Notes
				if errDetail == "" {
					errDetail = "NVR sozlamalarini tekshiring."
				}
				send(bot, chatID, fmt.Sprintf(
					"❌ <b>Klip #%d yozishda xatolik!</b>\n\n<code>%s</code>",
					clipID, html.EscapeString(errDetail)))
				return
			}

			if cur.Status == models.ClipStatusDone {
				send(bot, chatID, fmt.Sprintf("✅ Klip #%d tayyor! Mijozga yuborilmoqda...", clipID))

				caption := fmt.Sprintf(
					"🎬 <b>Sizning klipingiz tayyor!</b>\n\n📋 Buyurtma #%d\n🏢 %s | 🎱 %d-stol\n🕐 %s — %s",
					cr.ID, cr.BranchName, cr.TableNum,
					cr.StartTime.Format("15:04"), cr.EndTime.Format("15:04"),
				)

				f, err := os.Open(cur.ClipPath)
				if err != nil {
					send(bot, chatID, fmt.Sprintf("❌ Klip fayl ochilmadi: %v", err))
					return
				}

				fileName := filepath.Base(cur.ClipPath)
				videoMsg := tgbotapi.NewVideo(cur.ClientTgID,
					tgbotapi.FileReader{Name: fileName, Reader: f})
				videoMsg.Caption = caption
				videoMsg.ParseMode = "HTML"
				_, sendErr := bot.Send(videoMsg)
				f.Close()

				os.Remove(cur.ClipPath)

				if sendErr != nil {
					send(bot, chatID, fmt.Sprintf(
						"❌ Klip #%d Telegramga yuborishda xatolik: %v\n\n"+
							"Sababi: fayl hajmi juda katta (50MB dan oshib ketgan) bo'lishi mumkin.",
						clipID, sendErr))
					return
				}

				send(bot, chatID, fmt.Sprintf("✅ Klip #%d mijozga yuborildi!", clipID))
				return
			}
		}
		send(bot, chatID, fmt.Sprintf("⏰ Klip #%d yozish vaqti tugadi. Qayta urinib ko'ring.", clipID))
	}()
}

func (h *Handler) cbAdminClipDone(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, clipIDStr string) {
	clipID := mustParseInt64(clipIDStr)
	if err := h.clipSvc.SetStatus(user.ID, clipID, models.ClipStatusDone, ""); err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}
	editMessage(bot, chatID, msgID, fmt.Sprintf("✅ Klip #%d yuborildi deb belgilandi.", clipID), nil)
}

// cbAdminClipFail — adminga izoh yozishni so'raydi, keyin rad etadi
func (h *Handler) cbAdminClipFail(bot *tgbotapi.BotAPI, chatID int64, tgID int64, user *models.User, clipIDStr string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	clipID := mustParseInt64(clipIDStr)
	h.states.Set(tgID, StateAdminRejectNote)
	h.states.SetData(tgID, "clip_id", clipID)
	send(bot, chatID, fmt.Sprintf(
		"❌ Klip #%d rad etilmoqda.\n\n✏️ Mijozga yuboriladigan <b>izoh</b> yozing:\n<i>(Masalan: NVR da yozuv topilmadi, kamera ishlamagan, boshqa sabab)</i>",
		clipID))
}

// cbAdminRefund — adminga izoh yozishni so'raydi, keyin qaytaradi
func (h *Handler) cbAdminRefund(bot *tgbotapi.BotAPI, chatID int64, tgID int64, user *models.User, clipIDStr string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	clipID := mustParseInt64(clipIDStr)
	h.states.Set(tgID, StateAdminRefundNote)
	h.states.SetData(tgID, "clip_id", clipID)
	send(bot, chatID, fmt.Sprintf(
		"↩️ Klip #%d qaytarilmoqda.\n\n✏️ Mijozga yuboriladigan <b>izoh</b> yozing:\n<i>(Masalan: to'lov tasdiqlanmadi, texnik muammo)</i>",
		clipID))
}

// handleAdminNoteInput — admin izoh yozganda status o'rnatib foydalanuvchiga xabar yuboradi
func (h *Handler) handleAdminNoteInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, user *models.User, isRefund bool) {
	tgID := msg.From.ID
	chatID := msg.Chat.ID
	note := strings.TrimSpace(msg.Text)

	clipIDVal, _ := h.states.GetData(tgID, "clip_id")
	h.states.Clear(tgID)

	clipID, _ := clipIDVal.(int64)
	if clipID == 0 {
		send(bot, chatID, "❌ Xatolik: buyurtma ID topilmadi.")
		return
	}

	cr, err := h.clipSvc.GetByID(clipID)
	if err != nil {
		send(bot, chatID, "❌ Buyurtma topilmadi.")
		return
	}

	var newStatus string
	var adminMsg, clientMsg string
	if isRefund {
		newStatus = models.ClipStatusRefunded
		adminMsg = fmt.Sprintf("↩️ Klip #%d qaytarildi.", clipID)
		clientMsg = fmt.Sprintf(
			"↩️ <b>Klip #%d qaytarildi</b>\n\n"+
				"🏢 %s | 🎱 %d-stol\n"+
				"🕐 %s — %s\n\n"+
				"📝 <b>Sabab:</b> %s\n\n"+
				"Savollar bo'lsa admin bilan bog'laning.",
			cr.ID, cr.BranchName, cr.TableNum,
			cr.StartTime.Format("02.01.2006 15:04"),
			cr.EndTime.Format("15:04"),
			note,
		)
	} else {
		newStatus = models.ClipStatusFailed
		adminMsg = fmt.Sprintf("❌ Klip #%d rad etildi.", clipID)
		clientMsg = fmt.Sprintf(
			"❌ <b>Klip #%d rad etildi</b>\n\n"+
				"🏢 %s | 🎱 %d-stol\n"+
				"🕐 %s — %s\n\n"+
				"📝 <b>Sabab:</b> %s\n\n"+
				"Yangi so'rov yuborishingiz mumkin.",
			cr.ID, cr.BranchName, cr.TableNum,
			cr.StartTime.Format("02.01.2006 15:04"),
			cr.EndTime.Format("15:04"),
			note,
		)
	}

	if err := h.clipSvc.SetStatus(user.ID, clipID, newStatus, note); err != nil {
		send(bot, chatID, fmt.Sprintf("❌ Saqlashda xatolik: %v", err))
		return
	}

	send(bot, chatID, adminMsg)
	send(bot, cr.ClientTgID, clientMsg)
	h.logAction(user, "clip_"+newStatus, fmt.Sprintf("clip:%d note:%s", clipID, note))
}

// ===================== XODIMLAR =====================

func (h *Handler) showStaffList(bot *tgbotapi.BotAPI, chatID int64, user *models.User) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin) {
		return
	}

	users, err := h.userSvc.ListStaff()
	if err != nil {
		send(bot, chatID, "❌ Xatolik.")
		return
	}

	var sb strings.Builder
	sb.WriteString("👥 <b>Xodimlar ro'yxati</b>\n\n")

	for _, u := range users {
		if u.Role == models.RoleClient {
			continue
		}
		roleIcon := "🎮"
		switch u.Role {
		case models.RoleSuperadmin:
			roleIcon = "👑"
		case models.RoleAdmin:
			roleIcon = "🔧"
		}
		active := "✅"
		if !u.IsActive {
			active = "❌"
		}
		phone := u.Phone
		if phone == "" {
			phone = "—"
		}
		sb.WriteString(fmt.Sprintf("%s %s %s | 📱 %s | ID: <code>%d</code>\n",
			active, roleIcon, u.DisplayName(), phone, u.TelegramID))
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("➕ Xodim qo'shish/tahrirlash", "add_staff"),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendWithKeyboard(bot, chatID, sb.String(), kb)
}

func (h *Handler) cbSetRole(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, targetTgIDStr, role string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin) {
		return
	}

	targetTgID := mustParseInt64(targetTgIDStr)
	if err := h.userSvc.SetRole(user.TelegramID, targetTgID, role, nil); err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}

	h.userCache.Invalidate(targetTgID)
	editMessage(bot, chatID, msgID,
		fmt.Sprintf("✅ ID %d uchun rol <b>%s</b> ga o'zgartirildi.", targetTgID, role), nil)
	h.logAction(user, "set_role", fmt.Sprintf("target:%d role:%s", targetTgID, role))
}

func (h *Handler) showSettings(bot *tgbotapi.BotAPI, chatID int64, user *models.User) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin) {
		return
	}
	branches, err := h.tableSvc.GetBranches()
	if err != nil {
		send(bot, chatID, "❌ Filiallar yuklanmadi.")
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, b := range branches {
		nvrStatus := "❌"
		if b.NVRHost != "" {
			nvrStatus = fmt.Sprintf("✅ %s:%d", b.NVRHost, b.NVRPort)
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("🔌 %s NVR — %s", b.Name, nvrStatus),
				fmt.Sprintf("nvr_setup:%d", b.ID),
			),
		))
	}
	// Har bir filial uchun per-stol RTSP sozlash
	for _, b := range branches {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("📹 %s — stol RTSP", b.Name),
				fmt.Sprintf("rtsp_branch:%d", b.ID),
			),
		))
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendWithKeyboard(bot, chatID,
		"⚙️ <b>Sozlamalar</b>\n\n"+
			"🔌 <b>NVR</b> — branch darajasida (eski usul)\n"+
			"📹 <b>Stol RTSP</b> — har bir stol uchun alohida URL", kb)
}

// cbRTSPBranch — RTSP sozlash uchun filial tanlanganda stollarni ko'rsatadi
func (h *Handler) cbRTSPBranch(bot *tgbotapi.BotAPI, chatID int64, msgID int, branchIDStr string) {
	branchID := mustParseInt64(branchIDStr)
	tables, err := h.tableSvc.GetBranchTables(branchID)
	if err != nil || len(tables) == 0 {
		send(bot, chatID, "❌ Stollar topilmadi.")
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, t := range tables {
		rtspIcon := "❌"
		if t.RTSPUrl != "" {
			rtspIcon = "✅"
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%s %d-stol", rtspIcon, t.TableNum),
				fmt.Sprintf("rtsp_table:%d", t.ID),
			),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Orqaga", "settings_back"),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	editMessage(bot, chatID, msgID, "📹 RTSP URL o'rnatish uchun stol tanlang:\n\n✅ — URL belgilangan\n❌ — URL yo'q", &kb)
}

// cbRTSPTable — stol tanlanganda RTSP URL kiritishni so'raydi
func (h *Handler) cbRTSPTable(bot *tgbotapi.BotAPI, chatID int64, tgID int64, tableIDStr string) {
	tableID := mustParseInt64(tableIDStr)
	table, err := h.tableSvc.GetTable(tableID)
	if err != nil {
		send(bot, chatID, "❌ Stol topilmadi.")
		return
	}

	h.states.Set(tgID, StateTableRTSP)
	h.states.SetData(tgID, "table_id", tableID)

	current := "Belgilanmagan"
	if table.RTSPUrl != "" {
		current = table.RTSPUrl
	}

	send(bot, chatID, fmt.Sprintf(
		"📹 <b>%s — %d-stol RTSP URL</b>\n\n"+
			"Hozirgi: <code>%s</code>\n\n"+
			"Yangi RTSP URL ni kiriting:\n"+
			"<i>Misol: rtsp://admin:parol@192.168.1.100:554/Streaming/Channels/101</i>",
		table.BranchName, table.TableNum, current,
	))
}

// cbNVRSetup — NVR sozlash oqimini boshlaydi
func (h *Handler) cbNVRSetup(bot *tgbotapi.BotAPI, chatID int64, tgID int64, branchIDStr string) {
	branchID := mustParseInt64(branchIDStr)
	branch, err := h.tableSvc.GetBranchByID(branchID)
	if err != nil {
		send(bot, chatID, "❌ Filial topilmadi.")
		return
	}

	h.states.Set(tgID, StateNVRIP)
	h.states.SetData(tgID, "branch_id", branchID)

	current := "Hozircha yo'q"
	if branch.NVRHost != "" {
		current = fmt.Sprintf("%s:%d (user: %s)", branch.NVRHost, branch.NVRPort, branch.NVRUser)
	}

	send(bot, chatID, fmt.Sprintf(
		"📷 <b>%s — NVR sozlash</b>\n\n"+
			"Hozirgi: <code>%s</code>\n\n"+
			"NVR ning IP manzilini kiriting:\n<i>Misol: 192.168.1.100</i>",
		branch.Name, current,
	))
}

// handleNVRInput — NVR sozlash FSM qadamlari
func (h *Handler) handleNVRInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, state *UserState) {
	tgID := msg.From.ID
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	switch state.State {
	case StateNVRIP:
		h.states.SetData(tgID, "nvr_ip", text)
		h.states.Set(tgID, StateNVRPort)
		send(bot, chatID, "🔌 NVR portini kiriting:\n<i>Odatda: 554 (RTSP)</i>")

	case StateNVRPort:
		port := 554
		if p, err := strconv.Atoi(text); err == nil && p > 0 {
			port = p
		}
		h.states.SetData(tgID, "nvr_port", port)
		h.states.Set(tgID, StateNVRUser)
		send(bot, chatID, "👤 NVR login (username) ni kiriting:\n<i>Misol: admin</i>")

	case StateNVRUser:
		h.states.SetData(tgID, "nvr_user", text)
		h.states.Set(tgID, StateNVRPass)
		send(bot, chatID, "🔑 NVR parolini kiriting:")

	case StateNVRPass:
		branchID, _ := h.states.GetInt64(tgID, "branch_id")
		ip, _ := h.states.GetString(tgID, "nvr_ip")
		portVal, _ := h.states.GetData(tgID, "nvr_port")
		user, _ := h.states.GetString(tgID, "nvr_user")
		port := 554
		if p, ok := portVal.(int); ok {
			port = p
		}

		h.states.Clear(tgID)

		if err := h.tableSvc.UpdateBranchNVR(branchID, ip, port, user, text); err != nil {
			send(bot, chatID, fmt.Sprintf("❌ Saqlashda xatolik: %v", err))
			return
		}

		send(bot, chatID, fmt.Sprintf(
			"✅ <b>NVR sozlandi!</b>\n\n"+
				"🌐 IP: <code>%s</code>\n"+
				"🔌 Port: <code>%d</code>\n"+
				"👤 Login: <code>%s</code>\n\n"+
				"📷 Kamera test qilish uchun klip so'rovi orqali yoki admin paneldan foydalaning.",
			ip, port, user,
		))
	}
}

// handleTableRTSPInput — stol RTSP URL kiritish
func (h *Handler) handleTableRTSPInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, state *UserState) {
	tgID := msg.From.ID
	chatID := msg.Chat.ID
	rtspURL := strings.TrimSpace(msg.Text)

	tableID, _ := h.states.GetInt64(tgID, "table_id")
	h.states.Clear(tgID)

	if err := h.tableSvc.SetTableRTSP(tableID, rtspURL); err != nil {
		send(bot, chatID, fmt.Sprintf("❌ Saqlashda xatolik: %v", err))
		return
	}

	send(bot, chatID, fmt.Sprintf(
		"✅ <b>RTSP URL saqlandi!</b>\n\n"+
			"🎱 Stol #%d\n"+
			"<code>%s</code>",
		tableID, rtspURL,
	))
}

// cbAdminManualUpload — adminga video yuborishni so'raydi
func (h *Handler) cbAdminManualUpload(bot *tgbotapi.BotAPI, chatID int64, tgID int64, user *models.User, clipIDStr string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	clipID := mustParseInt64(clipIDStr)
	cr, err := h.clipSvc.GetByID(clipID)
	if err != nil {
		send(bot, chatID, "❌ Buyurtma topilmadi.")
		return
	}

	h.states.Set(tgID, StateAdminUploadClip)
	h.states.SetData(tgID, "clip_id", clipID)
	h.states.SetData(tgID, "client_tg_id", cr.ClientTgID)

	send(bot, chatID, fmt.Sprintf(
		"📤 <b>Klip #%d uchun video yuborish</b>\n\n"+
			"👤 Mijoz: %s\n"+
			"🕐 %s — %s\n\n"+
			"📹 Endi video faylni yuboring:",
		cr.ID, cr.ClientName,
		cr.StartTime.Format("02.01.2006 15:04"),
		cr.EndTime.Format("15:04"),
	))
}

// handleAdminUploadInput — admin yuborgan videoni mijozga jo'natadi
func (h *Handler) handleAdminUploadInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, user *models.User) {
	tgID := msg.From.ID
	chatID := msg.Chat.ID

	clipIDVal, _ := h.states.GetData(tgID, "clip_id")
	clientTgIDVal, _ := h.states.GetData(tgID, "client_tg_id")
	h.states.Clear(tgID)

	clipID, _ := clipIDVal.(int64)
	clientTgID, _ := clientTgIDVal.(int64)

	if clipID == 0 || clientTgID == 0 {
		send(bot, chatID, "❌ Xatolik: buyurtma ma'lumotlari topilmadi.")
		return
	}

	cr, err := h.clipSvc.GetByID(clipID)
	if err != nil {
		send(bot, chatID, "❌ Buyurtma topilmadi.")
		return
	}

	caption := fmt.Sprintf(
		"🎬 <b>Sizning klipingiz tayyor!</b>\n\n"+
			"📋 Buyurtma #%d\n"+
			"🏢 %s | 🎱 %d-stol\n"+
			"🕐 %s — %s",
		cr.ID, cr.BranchName, cr.TableNum,
		cr.StartTime.Format("02.01.2006 15:04"),
		cr.EndTime.Format("15:04"),
	)

	var sendErr error

	if msg.Video != nil {
		videoMsg := tgbotapi.NewVideo(clientTgID, tgbotapi.FileID(msg.Video.FileID))
		videoMsg.Caption = caption
		videoMsg.ParseMode = "HTML"
		_, sendErr = bot.Send(videoMsg)
	} else if msg.Document != nil {
		docMsg := tgbotapi.NewDocument(clientTgID, tgbotapi.FileID(msg.Document.FileID))
		docMsg.Caption = caption
		docMsg.ParseMode = "HTML"
		_, sendErr = bot.Send(docMsg)
	} else {
		send(bot, chatID, "⚠️ Video yoki fayl yuborish kerak edi. Qayta urinib ko'ring.")
		h.states.Set(tgID, StateAdminUploadClip)
		h.states.SetData(tgID, "clip_id", clipID)
		h.states.SetData(tgID, "client_tg_id", clientTgID)
		return
	}

	if sendErr != nil {
		send(bot, chatID, fmt.Sprintf("❌ Mijozga yuborishda xatolik: %v", sendErr))
		return
	}

	// Klipni done qilish
	_ = h.clipSvc.SetStatus(user.ID, clipID, models.ClipStatusDone, "")

	send(bot, chatID, fmt.Sprintf("✅ Klip #%d mijozga muvaffaqiyatli yuborildi!", clipID))
	h.logAction(user, "manual_upload", fmt.Sprintf("clip:%d -> client:%d", clipID, clientTgID))
}

// ===================== HELP =====================

func (h *Handler) cmdHelp(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, user *models.User) {
	var text string
	if user.IsStaff() {
		text = "ℹ️ <b>Yordam — Xodim</b>\n\n" +
			"🎬 <b>Klip so'rovlar</b> — Mijozlardan kelgan so'rovlar\n" +
			"   └ Ro'yxatdan so'rovni bosing → tafsilot\n" +
			"   └ ✅ To'lovni tasdiqlash → klip yozing yoki yuklang\n" +
			"   └ ❌ Rad etish / ↩️ Qaytarish → izoh yozing\n\n" +
			"👥 <b>Xodimlar</b> — Xodim qo'shish, rol berish\n\n" +
			"⚙️ <b>Sozlamalar</b> — NVR IP/port/login va stol RTSP URL\n\n" +
			"📌 <b>Buyruqlar:</b>\n" +
			"/start — Bosh menyu\n" +
			"/cancel — Joriy amaliyotni bekor qilish\n" +
			"/help — Ushbu yordam"
	} else {
		text = "ℹ️ <b>Yordam</b>\n\n" +
			"🎬 <b>Klip so'rash</b>\n" +
			"   └ Filial → Stol → Sana → Vaqt → Davomiylik tanlang\n" +
			"   └ Tasdiqlang → 10,000 so'm to'lab screenshot yuboring\n" +
			"   └ Admin tasdiqlasa klipingiz yuboriladi\n\n" +
			"📋 <b>Mening buyurtmalarim</b> — So'rovlaringiz holati\n\n" +
			"📌 <b>Buyruqlar:</b>\n" +
			"/start — Bosh menyu\n" +
			"/cancel — Joriy amaliyotni bekor qilish\n" +
			"/help — Ushbu yordam"
	}
	send(bot, msg.Chat.ID, text)
}

// ===================== HELPERS =====================

func mustParseInt64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
