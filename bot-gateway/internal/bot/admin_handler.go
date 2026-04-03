package bot

import (
	"fmt"
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

	h.logAction(user, "start", "")
	sendWithKeyboard(bot, msg.Chat.ID, text, mainMenuKeyboard(user))
}

// ===================== ADMIN PANEL =====================

func (h *Handler) cmdAdmin(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, user *models.User) {
	if !h.requireStaff(bot, msg.Chat.ID, user) {
		return
	}
	h.showBranchesForStaff(bot, msg.Chat.ID, user)
}

// showBranchesForStaff — filiallar ro'yxatini ko'rsatadi
func (h *Handler) showBranchesForStaff(bot *tgbotapi.BotAPI, chatID int64, user *models.User) {
	if !h.requireStaff(bot, chatID, user) {
		return
	}

	branches, err := h.tableSvc.GetBranches()
	if err != nil {
		send(bot, chatID, "❌ Filiallarni yuklashda xatolik.")
		return
	}

	// Admin/operator faqat o'z filialini ko'radi
	if !user.IsSuperadmin() && user.BranchID != nil {
		var filtered []*models.Branch
		for _, b := range branches {
			if b.ID == *user.BranchID {
				filtered = append(filtered, b)
			}
		}
		branches = filtered
	}

	if len(branches) == 0 {
		send(bot, chatID, "⚠️ Sizga filial biriktirilmagan. Superadmin bilan bog'laning.")
		return
	}

	text := "🏢 <b>Filiallar</b>\n\nQaysi filialning stollarini ko'rmoqchisiz?"
	sendWithKeyboard(bot, chatID, text, branchesKeyboard(branches))
}

// cbShowTables — filial tanlanganda stollarni ko'rsatadi
func (h *Handler) cbShowTables(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, branchIDStr string) {
	branchID, err := strconv.ParseInt(branchIDStr, 10, 64)
	if err != nil {
		return
	}

	if !branchAccessOK(user, branchID) {
		send(bot, chatID, "⛔ Bu filialni ko'rish huquqingiz yo'q.")
		return
	}

	h.cbShowTablesInline(bot, chatID, msgID, user, branchIDStr)
}

func (h *Handler) cbShowTablesInline(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, branchIDStr string) {
	branchID, _ := strconv.ParseInt(branchIDStr, 10, 64)
	branch, _ := h.tableSvc.GetBranchByID(branchID)
	tables, err := h.tableSvc.GetBranchTables(branchID)
	if err != nil || len(tables) == 0 {
		send(bot, chatID, "❌ Stollar topilmadi.")
		return
	}

	branchName := branchIDStr
	if branch != nil {
		branchName = branch.Name
	}

	free := 0
	busy := 0
	for _, t := range tables {
		if t.Status == models.TableStatusFree {
			free++
		} else {
			busy++
		}
	}

	text := fmt.Sprintf(
		"🏢 <b>%s</b>\n\n"+
			"🟢 Bo'sh: <b>%d</b>  |  🔴 Band: <b>%d</b>\n\n"+
			"Stolni tanlang:",
		branchName, free, busy,
	)

	kb := tablesKeyboard(tables)
	editMessage(bot, chatID, msgID, text, &kb)
}

// cbShowTableActions — stol tanlanganda amallar ko'rsatadi
func (h *Handler) cbShowTableActions(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, tableIDStr string) {
	tableID, err := strconv.ParseInt(tableIDStr, 10, 64)
	if err != nil {
		return
	}

	table, err := h.tableSvc.GetTable(tableID)
	if err != nil {
		send(bot, chatID, "❌ Stol topilmadi.")
		return
	}

	if !branchAccessOK(user, table.BranchID) {
		send(bot, chatID, "⛔ Bu stolga kirish huquqingiz yo'q.")
		return
	}

	statusText := "🟢 Bo'sh"
	if table.Status == models.TableStatusBusy {
		statusText = "🔴 Band"
	}

	text := fmt.Sprintf(
		"🎱 <b>%s — %d-stol</b>\n\n"+
			"Holat: %s\n"+
			"Soatlik narx: <b>%d so'm</b>",
		table.BranchName, table.TableNum,
		statusText,
		models.PricePerHourSom(table.PricePerHour),
	)

	kb := tableActionsKeyboard(table)
	editMessage(bot, chatID, msgID, text, &kb)
}

// ===================== SESSION =====================

func (h *Handler) cbStartSessionPrompt(bot *tgbotapi.BotAPI, chatID int64, tgID int64, tableIDStr string) {
	tableID, err := strconv.ParseInt(tableIDStr, 10, 64)
	if err != nil {
		return
	}
	h.states.Set(tgID, StateSessionClientName)
	h.states.SetData(tgID, "table_id", tableID)

	send(bot, chatID, "👤 Mijozning ismini kiriting:")
}

func (h *Handler) cbEndSession(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, tableIDStr string) {
	tableID, err := strconv.ParseInt(tableIDStr, 10, 64)
	if err != nil {
		return
	}

	session, err := h.tableSvc.EndSession(user, tableID)
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}

	duration := time.Since(session.StartedAt)
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60

	text := fmt.Sprintf(
		"✅ <b>Sessiya yakunlandi!</b>\n\n"+
			"👤 Mijoz: %s\n"+
			"⏱ Davomiyligi: %dh %dm\n"+
			"💰 Narx: <b>%d so'm</b>",
		session.ClientName,
		hours, minutes,
		session.TotalPrice/100,
	)

	h.logAction(user, "end_session", fmt.Sprintf("table:%d", tableID))

	table, _ := h.tableSvc.GetTable(tableID)
	var kb tgbotapi.InlineKeyboardMarkup
	if table != nil {
		kb = tableActionsKeyboard(table)
		editMessage(bot, chatID, msgID, text, &kb)
	} else {
		send(bot, chatID, text)
	}
}

func (h *Handler) cbViewSession(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, tableIDStr string) {
	tableID, err := strconv.ParseInt(tableIDStr, 10, 64)
	if err != nil {
		return
	}

	session, err := h.tableSvc.ActiveSession(tableID)
	if err != nil {
		send(bot, chatID, "⚠️ Faol sessiya topilmadi.")
		return
	}

	elapsed := time.Since(session.StartedAt)
	hours := int(elapsed.Hours())
	minutes := int(elapsed.Minutes()) % 60

	table, _ := h.tableSvc.GetTable(tableID)
	pricePerMin := int64(0)
	if table != nil {
		pricePerMin = table.PricePerHour / 60
	}
	currentPrice := pricePerMin * int64(elapsed.Minutes()) / 100

	text := fmt.Sprintf(
		"⏱ <b>Faol sessiya</b>\n\n"+
			"👤 Mijoz: %s\n"+
			"🕐 Boshlangan: %s\n"+
			"⏳ O'tgan vaqt: %dh %dm\n"+
			"💰 Hozirgi narx: ~<b>%d so'm</b>",
		session.ClientName,
		session.StartedAt.Format("15:04"),
		hours, minutes,
		currentPrice,
	)

	kb := tableActionsKeyboard(table)
	editMessage(bot, chatID, msgID, text, &kb)
}

// ===================== HISOBOT =====================

func (h *Handler) showReportBranches(bot *tgbotapi.BotAPI, chatID int64, user *models.User) {
	if !h.requireStaff(bot, chatID, user) {
		return
	}

	branches, err := h.tableSvc.GetBranches()
	if err != nil {
		send(bot, chatID, "❌ Xatolik.")
		return
	}

	if !user.IsSuperadmin() && user.BranchID != nil {
		var filtered []*models.Branch
		for _, b := range branches {
			if b.ID == *user.BranchID {
				filtered = append(filtered, b)
			}
		}
		branches = filtered
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, b := range branches {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("🏢 %s", b.Name),
				fmt.Sprintf("report:select:%d", b.ID),
			),
		))
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendWithKeyboard(bot, chatID, "📊 <b>Hisobot</b>\n\nFilial tanlang:", kb)
}

func (h *Handler) cbShowReport(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, period, branchIDStr string) {
	if period == "select" {
		// Filial tanlangandan so'ng period tanlash
		kb := reportKeyboard(mustParseInt64(branchIDStr))
		editMessage(bot, chatID, msgID, "📊 Davr tanlang:", &kb)
		return
	}

	branchID := mustParseInt64(branchIDStr)
	branch, _ := h.tableSvc.GetBranchByID(branchID)
	branchName := branchIDStr
	if branch != nil {
		branchName = branch.Name
	}

	var total int
	var priceSom int64
	var periodText string

	switch period {
	case "day":
		t, p, err := h.tableSvc.DailyReport(branchID)
		if err != nil {
			send(bot, chatID, "❌ Xatolik.")
			return
		}
		total, priceSom, periodText = t, p, "Bugun"
	case "month":
		t, p, err := h.tableSvc.MonthlyReport(branchID)
		if err != nil {
			send(bot, chatID, "❌ Xatolik.")
			return
		}
		total, priceSom, periodText = t, p, "Bu oy"
	}

	text := fmt.Sprintf(
		"📊 <b>Hisobot — %s</b>\n"+
			"🏢 Filial: %s\n\n"+
			"🎱 Sessiyalar: <b>%d</b>\n"+
			"💰 Jami daromad: <b>%d so'm</b>",
		periodText, branchName, total, priceSom,
	)

	editMessage(bot, chatID, msgID, text, nil)
}

// showTodaySessions — operatorning bugungi sessiyalarini ko'rsatadi
func (h *Handler) showTodaySessions(bot *tgbotapi.BotAPI, chatID int64, user *models.User) {
	if !h.requireStaff(bot, chatID, user) {
		return
	}

	branchID := int64(0)
	if user.BranchID != nil {
		branchID = *user.BranchID
	}
	if branchID == 0 {
		send(bot, chatID, "⚠️ Sizga filial biriktirilmagan.")
		return
	}

	total, priceSom, err := h.tableSvc.DailyReport(branchID)
	if err != nil {
		send(bot, chatID, "❌ Xatolik.")
		return
	}

	text := fmt.Sprintf(
		"📋 <b>Bugungi sessiyalar</b>\n\n"+
			"🎱 Jami: <b>%d</b> ta sessiya\n"+
			"💰 Daromad: <b>%d so'm</b>",
		total, priceSom,
	)
	send(bot, chatID, text)
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

	if len(clips) == 0 {
		send(bot, chatID, "✅ Hozircha kutilayotgan klip so'rovlar yo'q.")
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, c := range clips {
		statusIcon := "⏳"
		if c.Status == models.ClipStatusPaid {
			statusIcon = "💰"
		}
		label := fmt.Sprintf("%s #%d — %s %d-stol %s",
			statusIcon, c.ID, c.BranchName, c.TableNum,
			c.StartTime.Format("02.01 15:04"))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label,
				fmt.Sprintf("clip_detail:%d", c.ID)),
		))
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendWithKeyboard(bot, chatID,
		fmt.Sprintf("🎬 <b>Klip so'rovlar</b> (%d ta):", len(clips)), kb)
}

// Klip detail ko'rsatish uchun alohida callback ham qo'shish kerak
func (h *Handler) cbShowClipDetail(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, clipIDStr string) {
	clipID := mustParseInt64(clipIDStr)
	cr, err := h.clipSvc.GetByID(clipID)
	if err != nil {
		send(bot, chatID, "❌ Buyurtma topilmadi.")
		return
	}

	text := fmt.Sprintf(
		"🎬 <b>Klip #%d</b>\n\n"+
			"👤 Mijoz: %s\n"+
			"🏢 Filial: %s\n"+
			"🎱 Stol: %d\n"+
			"🕐 Boshlanish: %s\n"+
			"🕑 Tugash: %s\n"+
			"📊 Holat: <b>%s</b>",
		cr.ID, cr.ClientName, cr.BranchName, cr.TableNum,
		cr.StartTime.Format("02.01.2006 15:04"),
		cr.EndTime.Format("02.01.2006 15:04"),
		cr.Status,
	)

	kb := clipRequestActionsKeyboard(cr.ID, cr.Status)
	editMessage(bot, chatID, msgID, text, &kb)
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
		for i := 0; i < 36; i++ { // max 6 daqiqa (36 * 10s)
			time.Sleep(10 * time.Second)

			cur, err := h.clipSvc.GetByID(clipID)
			if err != nil {
				return
			}

			if cur.Status == models.ClipStatusFailed {
				send(bot, chatID, fmt.Sprintf("❌ Klip #%d yozishda xatolik. NVR sozlamalarini tekshiring.", clipID))
				return
			}

			if cur.Status == models.ClipStatusDone {
				send(bot, chatID, fmt.Sprintf("✅ Klip #%d tayyor! Mijozga yuborilmoqda...", clipID))

				caption := fmt.Sprintf(
					"🎬 <b>Sizning klipingiz tayyor!</b>\n\n📋 Buyurtma #%d\n🏢 %s | 🎱 %d-stol\n🕐 %s — %s",
					cr.ID, cr.BranchName, cr.TableNum,
					cr.StartTime.Format("15:04"), cr.EndTime.Format("15:04"),
				)

				// To'g'ridan shared volume dan o'qib stream qilish
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

				if sendErr != nil {
					// Video bo'lmasa document sifatida qayta urinib ko'r
					f2, err2 := os.Open(cur.ClipPath)
					if err2 == nil {
						docMsg := tgbotapi.NewDocument(cur.ClientTgID,
							tgbotapi.FileReader{Name: fileName, Reader: f2})
						docMsg.Caption = caption
						docMsg.ParseMode = "HTML"
						_, _ = bot.Send(docMsg)
						f2.Close()
					}
				}

				send(bot, chatID, fmt.Sprintf("✅ Klip #%d mijozga yuborildi!", clipID))
				return
			}
		}
		send(bot, chatID, fmt.Sprintf("⏰ Klip #%d yozish vaqti tugadi (6 daqiqa). Qayta urinib ko'ring.", clipID))
	}()
}

func (h *Handler) cbAdminClipDone(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, clipIDStr string) {
	clipID := mustParseInt64(clipIDStr)
	if err := h.clipSvc.SetStatus(user.ID, clipID, models.ClipStatusDone); err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}
	editMessage(bot, chatID, msgID, fmt.Sprintf("✅ Klip #%d yuborildi deb belgilandi.", clipID), nil)
}

func (h *Handler) cbAdminClipFail(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, clipIDStr string) {
	clipID := mustParseInt64(clipIDStr)
	if err := h.clipSvc.SetStatus(user.ID, clipID, models.ClipStatusFailed); err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}
	editMessage(bot, chatID, msgID, fmt.Sprintf("❌ Klip #%d muvaffaqiyatsiz deb belgilandi.", clipID), nil)
}

func (h *Handler) cbAdminRefund(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, clipIDStr string) {
	clipID := mustParseInt64(clipIDStr)
	if err := h.clipSvc.SetStatus(user.ID, clipID, models.ClipStatusRefunded); err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}
	editMessage(bot, chatID, msgID, fmt.Sprintf("↩️ Klip #%d qaytarildi deb belgilandi.", clipID), nil)
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
		sb.WriteString(fmt.Sprintf("%s %s %s | ID: <code>%d</code>\n",
			active, roleIcon, u.DisplayName(), u.TelegramID))
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

// ===================== HELPERS =====================

func mustParseInt64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
