package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"bot-gateway/internal/models"
	"bot-gateway/internal/repository"

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
	branch, _ := h.branchRepo.GetByID(branchID)
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
	branch, _ := h.branchRepo.GetByID(branchID)
	branchName := branchIDStr
	if branch != nil {
		branchName = branch.Name
	}

	sessionRepo := h.sessionRepo
	var total int
	var priceSom int64
	var periodText string

	switch period {
	case "day":
		t, p, err := sessionRepo.DailyReport(branchID)
		if err != nil {
			send(bot, chatID, "❌ Xatolik.")
			return
		}
		total, priceSom, periodText = t, p, "Bugun"
	case "month":
		t, p, err := sessionRepo.MonthlyReport(branchID)
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

	total, priceSom, err := h.sessionRepo.DailyReport(branchID)
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
			c.RequestedTime.Format("02.01 15:04"))
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
			"🕐 Vaqt: %s\n"+
			"⏱ Davomiylik: %d soniya\n"+
			"📊 Holat: <b>%s</b>",
		cr.ID, cr.ClientName, cr.BranchName, cr.TableNum,
		cr.RequestedTime.Format("02.01.2006 15:04"),
		cr.DurationSec, cr.Status,
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
	send(bot, chatID, "⚙️ <b>Sozlamalar</b>\n\nKelgusida qo'shiladi: NVR sozlamalari, narx o'zgartirish.")
}

// ===================== HELPERS =====================

func mustParseInt64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
