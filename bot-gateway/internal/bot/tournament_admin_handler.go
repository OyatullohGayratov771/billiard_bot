package bot

import (
	"fmt"
	"strings"
	"time"

	"bot-gateway/internal/config"
	"bot-gateway/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ===================== TURNIR — ADMIN =====================

func (h *Handler) showAdminTournamentList(bot *tgbotapi.BotAPI, chatID int64, user *models.User) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}

	tournaments, err := h.tournamentSvc.ListTournaments("")
	if err != nil {
		send(bot, chatID, "❌ Xatolik.")
		return
	}

	header := "🏆 <b>Turnirlar</b>\n\n"
	var rows [][]tgbotapi.InlineKeyboardButton

	if len(tournaments) == 0 {
		header += "Hali turnirlar yo'q."
	} else {
		for _, t := range tournaments {
			icon := tournamentStatusIcon(t.Status)
			label := fmt.Sprintf("%s %s  •  %s  •  %d/%d",
				icon, t.Name,
				t.ScheduledAt.Format("02.01 15:04"),
				t.ApprovedCount, t.MaxPlayers,
			)
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("admin_trn_detail:%d", t.ID)),
			))
		}
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("➕ Yangi turnir yaratish", "admin_trn_create"),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendWithKeyboard(bot, chatID, header, kb)
}

func (h *Handler) cbAdminTrnCreate(bot *tgbotapi.BotAPI, chatID int64, tgID int64, user *models.User) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	h.states.Set(tgID, StateTrnName)
	send(bot, chatID, "🏆 <b>Turnir yaratish</b>\n\n1️⃣ Turnir nomini kiriting:")
}

func (h *Handler) cbAdminTrnSelBranch(bot *tgbotapi.BotAPI, chatID int64, tgID int64, branchIDStr string) {
	branchID := mustParseInt64(branchIDStr)
	h.states.SetData(tgID, "trn_branch_id", branchID)
	h.states.Set(tgID, StateTrnDateTime)
	send(bot, chatID,
		"📅 <b>Sana va vaqtni kiriting:</b>\n\nFormat: <code>27.04.2026 18:00</code>")
}

func (h *Handler) cbAdminTrnDetail(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, trnIDStr string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	trnID := mustParseInt64(trnIDStr)
	t, err := h.tournamentSvc.GetTournament(trnID)
	if err != nil {
		send(bot, chatID, "❌ Turnir topilmadi.")
		return
	}

	tableInfo := "Belgilanmagan"
	if t.TableNum > 0 {
		tableInfo = fmt.Sprintf("%d-stol", t.TableNum)
	}

	text := fmt.Sprintf(
		"🏆 <b>%s</b>\n\n"+
			"📍 Filial: %s\n"+
			"🎱 Stol: %s\n"+
			"📅 Sana: <b>%s</b>\n"+
			"👥 Ishtirokchilar: <b>%d / %d</b>\n"+
			"📊 Holat: %s",
		t.Name,
		t.BranchName,
		tableInfo,
		t.ScheduledAt.Format("02.01.2006 15:04"),
		t.ApprovedCount, t.MaxPlayers,
		tournamentStatusText(t.Status),
	)

	kb := adminTournamentDetailKeyboard(t)
	if msgID > 0 {
		editMessage(bot, chatID, msgID, text, &kb)
	} else {
		sendWithKeyboard(bot, chatID, text, kb)
	}
}

func (h *Handler) cbAdminTrnCancel(bot *tgbotapi.BotAPI, chatID int64, user *models.User, trnIDStr string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	trnID := mustParseInt64(trnIDStr)
	t, err := h.tournamentSvc.GetTournament(trnID)
	if err != nil {
		send(bot, chatID, "❌ Turnir topilmadi.")
		return
	}
	kb := confirmKeyboard(
		fmt.Sprintf("admin_trn_cancel_confirm:%d", trnID),
		fmt.Sprintf("admin_trn_detail:%d", trnID),
	)
	sendWithKeyboard(bot, chatID, fmt.Sprintf(
		"⚠️ <b>%s</b> turnirini bekor qilmoqchimisiz?\n\n"+
			"Barcha ro'yxatdan o'tgan ishtirokchilarga xabar yuboriladi.",
		t.Name,
	), kb)
}

func (h *Handler) cbAdminTrnCancelConfirm(bot *tgbotapi.BotAPI, chatID int64, user *models.User, trnIDStr string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	trnID := mustParseInt64(trnIDStr)

	t, err := h.tournamentSvc.GetTournament(trnID)
	if err != nil {
		send(bot, chatID, "❌ Turnir topilmadi.")
		return
	}

	regs, _ := h.tournamentSvc.ListRegistrations(trnID)

	if err := h.tournamentSvc.CancelTournament(trnID); err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}

	notified := 0
	for _, r := range regs {
		if r.Status == models.RegStatusPending || r.Status == models.RegStatusApproved {
			send(bot, r.UserTgID, fmt.Sprintf(
				"❌ <b>Turnir bekor qilindi</b>\n\n"+
					"🏆 %s\n"+
					"📅 %s\n\n"+
					"Kechirasiz! Boshqa turnirlarni kuzatib boring.",
				t.Name,
				t.ScheduledAt.Format("02.01.2006 15:04"),
			))
			notified++
		}
	}

	h.logAction(user, "cancel_tournament", fmt.Sprintf("trn:%d notified:%d", trnID, notified))
	send(bot, chatID, fmt.Sprintf(
		"❌ <b>%s</b> bekor qilindi.\n%d ishtirokchiga xabar yuborildi.",
		t.Name, notified,
	))
	h.showAdminTournamentList(bot, chatID, user)
}

func (h *Handler) cbAdminTrnRegs(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, trnIDStr string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	trnID := mustParseInt64(trnIDStr)
	regs, err := h.tournamentSvc.ListRegistrations(trnID)
	if err != nil {
		send(bot, chatID, "❌ Xatolik.")
		return
	}

	t, _ := h.tournamentSvc.GetTournament(trnID)
	trnName := fmt.Sprintf("#%d", trnID)
	if t != nil {
		trnName = t.Name
	}

	header := fmt.Sprintf("👥 <b>%s — Ro'yxat</b>\n\n", trnName)
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, r := range regs {
		icon := regStatusIcon(r.Status)
		label := fmt.Sprintf("%s %s", icon, r.UserName)
		if r.Status == models.RegStatusPending {
			rows = append(rows,
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(label, "clip_noop"),
				),
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("✅ Tasdiqlash",
						fmt.Sprintf("admin_trn_approve:%d:%d", trnID, r.ID)),
					tgbotapi.NewInlineKeyboardButtonData("❌ Rad",
						fmt.Sprintf("admin_trn_reject:%d:%d", trnID, r.ID)),
				),
			)
		} else {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(label, "clip_noop"),
			))
		}
	}

	if len(regs) == 0 {
		header += "Hali ro'yxatdan o'tganlar yo'q."
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Orqaga", fmt.Sprintf("admin_trn_detail:%d", trnID)),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)

	if msgID > 0 {
		editMessage(bot, chatID, msgID, header, &kb)
	} else {
		sendWithKeyboard(bot, chatID, header, kb)
	}
}

func (h *Handler) cbAdminTrnApproveReg(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, trnIDStr, regIDStr string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	trnID := mustParseInt64(trnIDStr)
	regID := mustParseInt64(regIDStr)

	regs, _ := h.tournamentSvc.ListRegistrations(trnID)
	var targetReg *models.TournamentRegistration
	for _, r := range regs {
		if r.ID == regID {
			targetReg = r
			break
		}
	}

	if err := h.tournamentSvc.ApproveRegistration(trnID, regID); err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}

	if targetReg != nil {
		t, _ := h.tournamentSvc.GetTournament(trnID)
		trnName := fmt.Sprintf("#%d", trnID)
		if t != nil {
			trnName = t.Name
		}
		send(bot, targetReg.UserTgID, fmt.Sprintf(
			"✅ <b>Tabriklaymiz! Turnirga qabul qilindingiz!</b>\n\n"+
				"🏆 %s\n"+
				"📅 %s\n\n"+
				"Turnir boshlanishidan oldin sizga xabar beriladi.",
			trnName,
			func() string {
				if t != nil {
					return t.ScheduledAt.Format("02.01.2006 15:04")
				}
				return ""
			}(),
		))
	}

	h.cbAdminTrnRegs(bot, chatID, msgID, user, trnIDStr)
}

func (h *Handler) cbAdminTrnRejectReg(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, trnIDStr, regIDStr string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	trnID := mustParseInt64(trnIDStr)
	regID := mustParseInt64(regIDStr)

	regs, _ := h.tournamentSvc.ListRegistrations(trnID)
	var targetReg *models.TournamentRegistration
	for _, r := range regs {
		if r.ID == regID {
			targetReg = r
			break
		}
	}

	if err := h.tournamentSvc.RejectRegistration(trnID, regID); err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}

	if targetReg != nil {
		send(bot, targetReg.UserTgID, fmt.Sprintf(
			"❌ <b>Turnirga qabul qilinmadingiz.</b>\n\n"+
				"🏆 Turnir #%d\n\n"+
				"Boshqa turnirlarni kuzatib boring.\n"+
				"❓ Savollar bo'lsa: %s",
			trnID, config.AppConfig.SupportUsername,
		))
	}

	h.cbAdminTrnRegs(bot, chatID, msgID, user, trnIDStr)
}

func (h *Handler) cbAdminGenBracket(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, trnIDStr string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	trnID := mustParseInt64(trnIDStr)

	matches, err := h.tournamentSvc.GenerateBracket(trnID)
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}

	t, _ := h.tournamentSvc.GetTournament(trnID)
	trnName := fmt.Sprintf("#%d", trnID)
	if t != nil {
		trnName = t.Name
	}

	regs, _ := h.tournamentSvc.ListRegistrations(trnID)
	for _, r := range regs {
		if r.Status == models.RegStatusApproved {
			send(bot, r.UserTgID, fmt.Sprintf(
				"⚡ <b>%s — Turnir boshlanmoqda!</b>\n\n"+
					"Bracket tayyor. Turnir ro'yxatidan o'z o'yiningizni ko'ring.",
				trnName,
			))
		}
	}

	text := fmt.Sprintf("✅ Bracket yaratildi! <b>%d</b> o'yin.\n", len(matches))
	text += formatBracket(matches)

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏆 Natija kiritish",
				fmt.Sprintf("admin_trn_result:%d", trnID)),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Orqaga",
				fmt.Sprintf("admin_trn_detail:%d", trnID)),
		),
	)
	if msgID > 0 {
		editMessage(bot, chatID, msgID, text, &kb)
	} else {
		sendWithKeyboard(bot, chatID, text, kb)
	}
}

func (h *Handler) cbAdminTrnResult(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, trnIDStr string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	trnID := mustParseInt64(trnIDStr)

	matches, err := h.tournamentSvc.GetBracket(trnID)
	if err != nil {
		send(bot, chatID, "❌ Bracket topilmadi.")
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	text := fmt.Sprintf("🏆 <b>Turnir #%d — Natija kiritish</b>\n\n", trnID)

	for _, m := range matches {
		if m.Status != models.MatchStatusReady {
			continue
		}
		if m.Player1TgID == nil || m.Player2TgID == nil {
			continue
		}
		label := fmt.Sprintf("R%d M%d: %s vs %s",
			m.Round, m.MatchNum, m.Player1Name, m.Player2Name)
		rows = append(rows,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(label, "clip_noop"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🏆 "+m.Player1Name,
					fmt.Sprintf("admin_trn_winner:%d:%d:%d", trnID, m.ID, *m.Player1TgID)),
				tgbotapi.NewInlineKeyboardButtonData("🏆 "+m.Player2Name,
					fmt.Sprintf("admin_trn_winner:%d:%d:%d", trnID, m.ID, *m.Player2TgID)),
			),
		)
	}

	if len(rows) == 0 {
		text += "Hozirda natija kiritish kerak bo'lgan o'yin yo'q."
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Orqaga", fmt.Sprintf("admin_trn_detail:%d", trnID)),
	))

	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	if msgID > 0 {
		editMessage(bot, chatID, msgID, text, &kb)
	} else {
		sendWithKeyboard(bot, chatID, text, kb)
	}
}

// cbAdminTrnWinner — arg1=trnID, arg2="matchID:winnerTgID"
func (h *Handler) cbAdminTrnWinner(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, trnIDStr, matchWinner string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	trnID := mustParseInt64(trnIDStr)
	innerParts := strings.SplitN(matchWinner, ":", 2)
	if len(innerParts) != 2 {
		send(bot, chatID, "❌ Xatolik: noto'g'ri format.")
		return
	}
	matchID := mustParseInt64(innerParts[0])
	winnerTgID := mustParseInt64(innerParts[1])

	nextMatchID, finished, err := h.tournamentSvc.SetResult(matchID, winnerTgID)
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}

	send(bot, winnerTgID, "🏆 <b>Tabriklaymiz! Siz g'olib bo'ldingiz!</b>\n\nKeyingi bosqichga o'tdingiz.")

	if finished {
		t, _ := h.tournamentSvc.GetTournament(trnID)
		trnName := fmt.Sprintf("#%d", trnID)
		if t != nil {
			trnName = t.Name
		}
		send(bot, chatID, fmt.Sprintf("🏆 <b>%s yakunlandi! G'olib aniqlandi.</b>", trnName))
	} else if nextMatchID > 0 {
		send(bot, chatID, fmt.Sprintf("✅ Natija kiritildi. Keyingi o'yin #%d tayyor bo'ldi.", nextMatchID))
	}

	// Show updated result screen
	h.cbAdminTrnResult(bot, chatID, msgID, user, trnIDStr)
}

// ===================== TURNIR STATE HANDLERS (admin creation FSM) =====================

func (h *Handler) handleTrnNameInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	tgID := msg.From.ID
	chatID := msg.Chat.ID
	name := safeText(msg.Text, 100)
	if name == "" {
		send(bot, chatID, "⚠️ Nom bo'sh bo'lishi mumkin emas.")
		return
	}
	h.states.SetData(tgID, "trn_name", name)

	branches, err := h.tableSvc.GetBranches()
	if err != nil || len(branches) == 0 {
		send(bot, chatID, "❌ Filiallar topilmadi.")
		h.states.Clear(tgID)
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, b := range branches {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏢 "+b.Name,
				fmt.Sprintf("admin_trn_sel_branch:%d", b.ID)),
		))
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendWithKeyboard(bot, chatID, "2️⃣ Qaysi filialda o'tkaziladi?", kb)
}

func (h *Handler) handleTrnDateTimeInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	tgID := msg.From.ID
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	dt, err := time.ParseInLocation("02.01.2006 15:04", text, time.Local)
	if err != nil {
		send(bot, chatID, "⚠️ Format noto'g'ri.\n\nMisol: <code>27.04.2026 18:00</code>")
		return
	}
	if dt.Before(time.Now()) {
		send(bot, chatID, "⚠️ Sana o'tib ketgan. Kelajak sanani kiriting.")
		return
	}
	h.states.SetData(tgID, "trn_datetime", dt)
	h.states.Set(tgID, StateTrnMaxPlayers)
	send(bot, chatID, "3️⃣ Maksimal ishtirokchilar sonini kiriting:\n<i>Misol: 4, 8, 16, 32</i>")
}

func (h *Handler) handleTrnMaxPlayersInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	tgID := msg.From.ID
	chatID := msg.Chat.ID

	maxPlayers := int(mustParseInt64(strings.TrimSpace(msg.Text)))
	if maxPlayers < 2 {
		send(bot, chatID, "⚠️ Kamida 2 ta ishtirokchi bo'lishi kerak.")
		return
	}

	name, _ := h.states.GetString(tgID, "trn_name")
	branchID, _ := h.states.GetInt64(tgID, "trn_branch_id")
	dtVal, _ := h.states.GetData(tgID, "trn_datetime")

	h.states.Clear(tgID)

	dt, _ := dtVal.(time.Time)

	t, err := h.tournamentSvc.CreateTournament(name, branchID, nil, dt, 0, maxPlayers, tgID)
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}

	send(bot, chatID, fmt.Sprintf(
		"✅ <b>Turnir yaratildi!</b>\n\n"+
			"🏆 %s\n"+
			"📅 %s\n"+
			"👥 Max: %d ishtirokchi\n\n"+
			"Ro'yxatga olish boshlandi.",
		t.Name,
		t.ScheduledAt.Format("02.01.2006 15:04"),
		t.MaxPlayers,
	))
}

// ===================== HELPERS =====================

func tournamentStatusIcon(status string) string {
	switch status {
	case models.TournamentStatusRegistration:
		return "📋"
	case models.TournamentStatusInProgress:
		return "⚡"
	case models.TournamentStatusFinished:
		return "🏆"
	case models.TournamentStatusCancelled:
		return "❌"
	}
	return "•"
}

func tournamentStatusText(status string) string {
	switch status {
	case models.TournamentStatusRegistration:
		return "📋 Ro'yxatga olish"
	case models.TournamentStatusInProgress:
		return "⚡ Jarayonda"
	case models.TournamentStatusFinished:
		return "🏆 Yakunlandi"
	case models.TournamentStatusCancelled:
		return "❌ Bekor qilindi"
	}
	return status
}

func regStatusIcon(status string) string {
	switch status {
	case models.RegStatusPending:
		return "⏳"
	case models.RegStatusApproved:
		return "✅"
	case models.RegStatusRejected:
		return "❌"
	}
	return "•"
}

func formatTrnPrice(price int64) string {
	if price == 0 {
		return "Bepul"
	}
	return fmt.Sprintf("%d", price)
}

func formatBracket(matches []*models.TournamentMatch) string {
	var sb strings.Builder
	curRound := 0
	for _, m := range matches {
		if m.Round != curRound {
			curRound = m.Round
			sb.WriteString(fmt.Sprintf("\n<b>📍 %d-bosqich</b>\n", curRound))
		}
		switch m.Status {
		case models.MatchStatusBye:
			sb.WriteString(fmt.Sprintf("  • %s → keyingi bosqich\n", m.Player1Name))
		case models.MatchStatusVoid:
			sb.WriteString(fmt.Sprintf("  • O'yin %d (bo'sh)\n", m.MatchNum))
		case models.MatchStatusDone:
			sb.WriteString(fmt.Sprintf("  • %s vs %s → 🏆 %s\n",
				m.Player1Name, m.Player2Name, m.WinnerName))
		case models.MatchStatusReady:
			sb.WriteString(fmt.Sprintf("  ⚡ %s vs %s\n", m.Player1Name, m.Player2Name))
		default:
			sb.WriteString(fmt.Sprintf("  • O'yin %d (kutilmoqda)\n", m.MatchNum))
		}
	}
	return sb.String()
}
