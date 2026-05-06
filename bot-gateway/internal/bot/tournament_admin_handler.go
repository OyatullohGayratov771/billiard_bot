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
				t.ScheduledAt.In(localTZ).Format("02.01 15:04"),
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
			"🎮 Tur: %s\n"+
			"📊 Holat: %s",
		t.Name,
		t.BranchName,
		tableInfo,
		t.ScheduledAt.In(localTZ).Format("02.01.2006 15:04"),
		t.ApprovedCount, t.MaxPlayers,
		tournamentTypeText(t.Type),
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
		if (r.Status == models.RegStatusPending || r.Status == models.RegStatusApproved) && r.UserTgID > 0 {
			send(bot, r.UserTgID, fmt.Sprintf(
				"❌ <b>Turnir bekor qilindi</b>\n\n"+
					"🏆 %s\n"+
					"📅 %s\n\n"+
					"Kechirasiz! Boshqa turnirlarni kuzatib boring.",
				t.Name,
				t.ScheduledAt.In(localTZ).Format("02.01.2006 15:04"),
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
					return t.ScheduledAt.In(localTZ).Format("02.01.2006 15:04")
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
		if r.Status == models.RegStatusApproved && r.UserTgID > 0 {
			send(bot, r.UserTgID, fmt.Sprintf(
				"⚡ <b>%s — Turnir boshlanmoqda!</b>\n\n"+
					"Bracket tayyor. Turnir ro'yxatidan o'z o'yiningizni ko'ring.",
				trnName,
			))
		}
	}

	// Round-1 tayyor o'yinlarda raqiblar haqida xabar (faqat haqiqiy Telegram foydalanuvchilar)
	for _, m := range matches {
		if m.Round == 1 && m.Status == models.MatchStatusReady && m.Player1TgID != nil && m.Player2TgID != nil {
			if *m.Player1TgID > 0 {
				send(bot, *m.Player1TgID, fmt.Sprintf(
					"⚔️ <b>Sizning birinchi o'yiningiz!</b>\n\nRaqibingiz: <b>%s</b>", m.Player2Name))
			}
			if *m.Player2TgID > 0 {
				send(bot, *m.Player2TgID, fmt.Sprintf(
					"⚔️ <b>Sizning birinchi o'yiningiz!</b>\n\nRaqibingiz: <b>%s</b>", m.Player1Name))
			}
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

	winnerNextID, loserNextID, finished, err := h.tournamentSvc.SetResult(matchID, winnerTgID)
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}

	if winnerTgID > 0 {
		send(bot, winnerTgID, "🏆 <b>Tabriklaymiz! Siz g'olib bo'ldingiz!</b>\n\nKeyingi bosqichga o'tdingiz.")
	}

	if finished {
		t, _ := h.tournamentSvc.GetTournament(trnID)
		trnName := fmt.Sprintf("#%d", trnID)
		if t != nil {
			trnName = t.Name
		}
		send(bot, chatID, fmt.Sprintf("🏆 <b>%s yakunlandi! G'olib aniqlandi.</b>", trnName))
	} else {
		allMatches, _ := h.tournamentSvc.GetBracket(trnID)
		notifyMatch := func(nmID int64, label string) {
			for _, m := range allMatches {
				if m.ID == nmID && m.Status == models.MatchStatusReady && m.Player1TgID != nil && m.Player2TgID != nil {
					if *m.Player1TgID > 0 {
						send(bot, *m.Player1TgID, fmt.Sprintf("⚔️ <b>%s tayyor!</b>\n\nRaqibingiz: <b>%s</b>", label, m.Player2Name))
					}
					if *m.Player2TgID > 0 {
						send(bot, *m.Player2TgID, fmt.Sprintf("⚔️ <b>%s tayyor!</b>\n\nRaqibingiz: <b>%s</b>", label, m.Player1Name))
					}
					break
				}
			}
		}
		if winnerNextID > 0 {
			send(bot, chatID, fmt.Sprintf("✅ Natija kiritildi. Keyingi o'yin #%d.", winnerNextID))
			notifyMatch(winnerNextID, "Keyingi o'yiningiz")
		}
		if loserNextID > 0 {
			notifyMatch(loserNextID, "Losers bracket o'yiningiz")
		}
	}

	// Show updated result screen
	h.cbAdminTrnResult(bot, chatID, msgID, user, trnIDStr)
}

// ===================== TURNIRGA QOLDA O'YINCHI QO'SHISH (admin) =====================

func (h *Handler) cbAdminTrnAddPlayer(bot *tgbotapi.BotAPI, chatID int64, tgID int64, user *models.User, trnIDStr string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	trnID := mustParseInt64(trnIDStr)
	t, err := h.tournamentSvc.GetTournament(trnID)
	if err != nil {
		send(bot, chatID, "❌ Turnir topilmadi.")
		return
	}
	if t.Status != models.TournamentStatusRegistration {
		send(bot, chatID, "❌ Faqat ro'yxatga olish holatidagi turnirga o'yinchi qo'shish mumkin.")
		return
	}
	h.states.Set(tgID, StateTrnAddPlayer)
	h.states.SetData(tgID, "trn_add_player_id", trnID)
	send(bot, chatID, fmt.Sprintf(
		"➕ <b>%s</b> — O'yinchi qo'shish\n\n"+
			"O'yinchi ismini kiriting:\n"+
			"<i>(Masalan: Azizbek, Ivan yoki istalgan nom)</i>",
		t.Name,
	))
}

func (h *Handler) handleTrnAddPlayerInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, user *models.User) {
	tgID := msg.From.ID
	chatID := msg.Chat.ID
	name := safeText(msg.Text, 64)
	if name == "" {
		send(bot, chatID, "⚠️ Ism bo'sh bo'lishi mumkin emas.")
		return
	}
	trnID, _ := h.states.GetInt64(tgID, "trn_add_player_id")
	h.states.Clear(tgID)

	reg, err := h.tournamentSvc.RegisterManual(trnID, name)
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}
	h.logAction(user, "manual_player_added", fmt.Sprintf("trn:%d player:%s reg:%d", trnID, name, reg.ID))
	send(bot, chatID, fmt.Sprintf("✅ <b>%s</b> turnirga qo'shildi va tasdiqlandi.", name))
}

// ===================== TURNIR TAHRIRLASH (admin) =====================

func (h *Handler) cbAdminTrnEdit(bot *tgbotapi.BotAPI, chatID int64, msgID int, tgID int64, user *models.User, trnIDStr string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	trnID := mustParseInt64(trnIDStr)
	t, err := h.tournamentSvc.GetTournament(trnID)
	if err != nil {
		send(bot, chatID, "❌ Turnir topilmadi.")
		return
	}
	if t.Status != models.TournamentStatusRegistration {
		send(bot, chatID, "❌ Faqat ro'yxatga olish holatidagi turnirni tahrirlash mumkin.")
		return
	}

	text := fmt.Sprintf(
		"✏️ <b>%s — Tahrirlash</b>\n\n"+
			"📝 Nom: %s\n"+
			"📅 Sana: %s\n"+
			"👥 Max: %d ishtirokchi\n\n"+
			"Nima o'zgartirasiz?",
		t.Name, t.Name,
		t.ScheduledAt.In(localTZ).Format("02.01.2006 15:04"),
		t.MaxPlayers,
	)
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📝 Nom",
				fmt.Sprintf("admin_trn_edit_field:%d:name", trnID)),
			tgbotapi.NewInlineKeyboardButtonData("📅 Sana",
				fmt.Sprintf("admin_trn_edit_field:%d:date", trnID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👥 Max o'yinchilar",
				fmt.Sprintf("admin_trn_edit_field:%d:max", trnID)),
		),
		tgbotapi.NewInlineKeyboardRow(
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

func (h *Handler) cbAdminTrnEditField(bot *tgbotapi.BotAPI, chatID int64, tgID int64, trnIDStr, field string) {
	trnID := mustParseInt64(trnIDStr)
	h.states.SetData(tgID, "trn_edit_id", trnID)
	switch field {
	case "name":
		h.states.Set(tgID, StateTrnEditName)
		send(bot, chatID, "📝 Yangi turnir nomini kiriting:")
	case "date":
		h.states.Set(tgID, StateTrnEditDate)
		send(bot, chatID, "📅 Yangi sana va vaqtni kiriting:\n\nFormat: <code>27.04.2026 18:00</code>")
	case "max":
		h.states.Set(tgID, StateTrnEditMax)
		send(bot, chatID, "👥 Yangi maksimal ishtirokchilar sonini kiriting:")
	}
}

func (h *Handler) handleTrnEditNameInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	tgID := msg.From.ID
	chatID := msg.Chat.ID
	name := safeText(msg.Text, 100)
	if name == "" {
		send(bot, chatID, "⚠️ Nom bo'sh bo'lishi mumkin emas.")
		return
	}
	trnID, _ := h.states.GetInt64(tgID, "trn_edit_id")
	h.states.Clear(tgID)
	t, err := h.tournamentSvc.GetTournament(trnID)
	if err != nil {
		send(bot, chatID, "❌ Turnir topilmadi.")
		return
	}
	if err := h.tournamentSvc.UpdateTournament(trnID, name, t.ScheduledAt, t.MaxPlayers); err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}
	send(bot, chatID, "✅ Turnir nomi yangilandi: <b>"+name+"</b>")
}

func (h *Handler) handleTrnEditDateInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	tgID := msg.From.ID
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)
	dt, err := time.ParseInLocation("02.01.2006 15:04", text, localTZ)
	if err != nil {
		send(bot, chatID, "⚠️ Format noto'g'ri.\n\nMisol: <code>27.04.2026 18:00</code>")
		return
	}
	if dt.Before(time.Now()) {
		send(bot, chatID, "⚠️ Sana o'tib ketgan. Kelajak sanani kiriting.")
		return
	}
	trnID, _ := h.states.GetInt64(tgID, "trn_edit_id")
	h.states.Clear(tgID)
	t, err := h.tournamentSvc.GetTournament(trnID)
	if err != nil {
		send(bot, chatID, "❌ Turnir topilmadi.")
		return
	}
	if err := h.tournamentSvc.UpdateTournament(trnID, t.Name, dt, t.MaxPlayers); err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}
	send(bot, chatID, fmt.Sprintf("✅ Turnir sanasi yangilandi: <b>%s</b>", dt.Format("02.01.2006 15:04")))
}

func (h *Handler) handleTrnEditMaxInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	tgID := msg.From.ID
	chatID := msg.Chat.ID
	maxPlayers := int(mustParseInt64(strings.TrimSpace(msg.Text)))
	if maxPlayers < 2 {
		send(bot, chatID, "⚠️ Kamida 2 ta ishtirokchi bo'lishi kerak.")
		return
	}
	trnID, _ := h.states.GetInt64(tgID, "trn_edit_id")
	h.states.Clear(tgID)
	t, err := h.tournamentSvc.GetTournament(trnID)
	if err != nil {
		send(bot, chatID, "❌ Turnir topilmadi.")
		return
	}
	if err := h.tournamentSvc.UpdateTournament(trnID, t.Name, t.ScheduledAt, maxPlayers); err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}
	send(bot, chatID, fmt.Sprintf("✅ Max ishtirokchilar yangilandi: <b>%d</b>", maxPlayers))
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

	dt, err := time.ParseInLocation("02.01.2006 15:04", text, localTZ)
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

	h.states.SetData(tgID, "trn_max_players", int64(maxPlayers))

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🎯 Single Elimination", "admin_trn_sel_type:single_elimination"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Double Elimination", "admin_trn_sel_type:double_elimination"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔁 Round Robin", "admin_trn_sel_type:round_robin"),
		),
	)
	sendWithKeyboard(bot, chatID,
		"4️⃣ <b>Turnir turini tanlang:</b>\n\n"+
			"🎯 <b>Single Elimination</b> — mag'lub bo'lsa turnirdan chiqadi\n"+
			"🔄 <b>Double Elimination</b> — ikkinchi imkoniyat bor (4/8/16 o'yinchi)\n"+
			"🔁 <b>Round Robin</b> — hamma hammaga qarshi o'ynaydi",
		kb)
}

func (h *Handler) cbAdminTrnSelType(bot *tgbotapi.BotAPI, chatID int64, tgID int64, typeName string) {
	h.states.SetData(tgID, "trn_type", typeName)
	h.states.Set(tgID, StateTrnSetCode)
	send(bot, chatID,
		"5️⃣ <b>Maxfiy kod (ixtiyoriy):</b>\n\n"+
			"Turnirga faqat siz bergan kodni bilganlar kira oladi.\n\n"+
			"Kod o'rnatmaslik uchun <code>-</code> yuboring.")
}

func (h *Handler) handleTrnSetCodeInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	tgID := msg.From.ID
	chatID := msg.Chat.ID

	raw := strings.TrimSpace(msg.Text)
	joinCode := ""
	if raw != "-" {
		joinCode = safeText(raw, 32)
	}

	name, _ := h.states.GetString(tgID, "trn_name")
	branchID, _ := h.states.GetInt64(tgID, "trn_branch_id")
	maxVal, _ := h.states.GetInt64(tgID, "trn_max_players")
	dtVal, _ := h.states.GetData(tgID, "trn_datetime")
	trnType, _ := h.states.GetString(tgID, "trn_type")
	h.states.Clear(tgID)

	dt, _ := dtVal.(time.Time)
	maxPlayers := int(maxVal)
	if trnType == "" {
		trnType = models.TournamentTypeSingleElim
	}

	t, err := h.tournamentSvc.CreateTournament(name, branchID, nil, dt, 0, maxPlayers, tgID, joinCode, trnType)
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}

	codeInfo := "🔓 Kodsiz (ochiq)"
	if t.JoinCode != "" {
		codeInfo = fmt.Sprintf("🔑 Maxfiy kod: <code>%s</code>", t.JoinCode)
	}
	send(bot, chatID, fmt.Sprintf(
		"✅ <b>Turnir yaratildi!</b>\n\n"+
			"🏆 %s\n"+
			"📅 %s\n"+
			"👥 Max: %d ishtirokchi\n"+
			"%s\n\n"+
			"Ro'yxatga olish boshlandi.",
		t.Name,
		t.ScheduledAt.In(localTZ).Format("02.01.2006 15:04"),
		t.MaxPlayers,
		codeInfo,
	))
}

// ===================== HELPERS =====================

func tournamentTypeText(t string) string {
	switch t {
	case models.TournamentTypeDoubleElim:
		return "🔄 Double Elimination"
	case models.TournamentTypeRoundRobin:
		return "🔁 Round Robin"
	default:
		return "🎯 Single Elimination"
	}
}

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

// ===================== TV BRACKET =====================

func (h *Handler) cmdTVBracket(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, user *models.User) {
	chatID := msg.Chat.ID
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	args := strings.Fields(msg.CommandArguments())
	if len(args) == 0 {
		send(bot, chatID, "❗ Ishlatish: /tv_bracket <turnir_id>\n\nMisol: /tv_bracket 5")
		return
	}
	var trnID int64
	if _, err := fmt.Sscan(args[0], &trnID); err != nil || trnID <= 0 {
		send(bot, chatID, "❌ Noto'g'ri turnir ID.")
		return
	}
	token, err := h.tournamentSvc.GenerateTVToken(trnID)
	if err != nil {
		send(bot, chatID, "❌ Token yaratib bo'lmadi: "+err.Error())
		return
	}
	base := config.AppConfig.TVBaseURL
	if base == "" {
		send(bot, chatID, fmt.Sprintf(
			"🖥 <b>TV Bracket token:</b>\n\n<code>%s</code>\n\n"+
				"⚠️ <code>TV_BASE_URL</code> env o'rnatilmagan.\n"+
				"URL ko'rinishi: <code>https://sizning-domen.uz/tv/%s</code>",
			token, token))
		return
	}
	url := base + "/tv/" + token
	send(bot, chatID, fmt.Sprintf(
		"🖥 <b>TV Bracket URL:</b>\n\n<code>%s</code>\n\n"+
			"Ushbu URLni TVda oching.\n"+
			"Yangilash uchun: /tv_update %d",
		url, trnID))
}

func (h *Handler) cmdTVUpdate(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, user *models.User) {
	chatID := msg.Chat.ID
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	args := strings.Fields(msg.CommandArguments())
	if len(args) == 0 {
		send(bot, chatID, "❗ Ishlatish: /tv_update <turnir_id>\n\nMisol: /tv_update 5")
		return
	}
	var trnID int64
	if _, err := fmt.Sscan(args[0], &trnID); err != nil || trnID <= 0 {
		send(bot, chatID, "❌ Noto'g'ri turnir ID.")
		return
	}
	token, err := h.tournamentSvc.GetTVTokenByTournament(trnID)
	if err != nil {
		send(bot, chatID, "❌ Bu turnir uchun TV URL yo'q.\nAvval /tv_bracket "+fmt.Sprint(trnID)+" bilan URL oling.")
		return
	}
	viewers, err := h.tournamentSvc.PushTVUpdate(token)
	if err != nil {
		send(bot, chatID, "❌ Push yuborib bo'lmadi: "+err.Error())
		return
	}
	if viewers == 0 {
		send(bot, chatID, "📺 Signal yuborildi.\n<i>Hozir hech kim TVni ko'rmayapti.</i>")
	} else {
		send(bot, chatID, fmt.Sprintf("✅ <b>%d</b> ta TV ekran yangilandi! 📺", viewers))
	}
}

func formatBracket(matches []*models.TournamentMatch) string {
	if len(matches) == 0 {
		return ""
	}
	// Detect tournament type from first match's match_type
	first := matches[0]
	switch first.MatchType {
	case models.MatchTypeRoundRobin:
		return formatRRBracket(matches)
	}
	// Check if any match is DE-specific
	for _, m := range matches {
		if m.MatchType == models.MatchTypeLosers || m.MatchType == models.MatchTypeGrandFinal {
			return formatDEBracket(matches)
		}
	}
	return formatSEBracket(matches)
}

func formatMatchLine(m *models.TournamentMatch) string {
	switch m.Status {
	case models.MatchStatusBye:
		name := m.Player1Name
		if name == "" {
			name = m.Player2Name
		}
		return fmt.Sprintf("  • %s → keyingi bosqich\n", name)
	case models.MatchStatusVoid:
		return fmt.Sprintf("  • O'yin %d (bo'sh)\n", m.MatchNum)
	case models.MatchStatusDone:
		return fmt.Sprintf("  • %s vs %s → 🏆 %s\n", m.Player1Name, m.Player2Name, m.WinnerName)
	case models.MatchStatusReady:
		return fmt.Sprintf("  ⚡ %s vs %s\n", m.Player1Name, m.Player2Name)
	default:
		return fmt.Sprintf("  • O'yin %d (kutilmoqda)\n", m.MatchNum)
	}
}

func formatSEBracket(matches []*models.TournamentMatch) string {
	var sb strings.Builder
	curRound := 0
	for _, m := range matches {
		if m.Round != curRound {
			curRound = m.Round
			sb.WriteString(fmt.Sprintf("\n<b>📍 %d-bosqich</b>\n", curRound))
		}
		sb.WriteString(formatMatchLine(m))
	}
	return sb.String()
}

func formatDEBracket(matches []*models.TournamentMatch) string {
	var sb strings.Builder
	var wb, lb, gf []*models.TournamentMatch
	for _, m := range matches {
		switch m.MatchType {
		case models.MatchTypeLosers:
			lb = append(lb, m)
		case models.MatchTypeGrandFinal:
			gf = append(gf, m)
		default:
			wb = append(wb, m)
		}
	}

	sb.WriteString("\n🏆 <b>Winners Bracket</b>\n")
	curRound := 0
	for _, m := range wb {
		if m.Round != curRound {
			curRound = m.Round
			sb.WriteString(fmt.Sprintf("<b>R%d</b>\n", curRound))
		}
		sb.WriteString(formatMatchLine(m))
	}

	if len(lb) > 0 {
		sb.WriteString("\n⬇️ <b>Losers Bracket</b>\n")
		curRound = 0
		for _, m := range lb {
			lr := m.Round - models.MatchLBRoundOffset
			if lr != curRound {
				curRound = lr
				sb.WriteString(fmt.Sprintf("<b>LR%d</b>\n", curRound))
			}
			sb.WriteString(formatMatchLine(m))
		}
	}

	if len(gf) > 0 {
		sb.WriteString("\n🥇 <b>Grand Final</b>\n")
		for _, m := range gf {
			sb.WriteString(formatMatchLine(m))
		}
	}
	return sb.String()
}

func formatRRBracket(matches []*models.TournamentMatch) string {
	var sb strings.Builder
	sb.WriteString("\n🔁 <b>Round Robin — O'yinlar</b>\n\n")

	// Build standings map: tgID → {name, wins}
	type standing struct {
		name string
		wins int
	}
	standings := map[int64]*standing{}
	ensure := func(tgID int64, name string) {
		if _, ok := standings[tgID]; !ok {
			standings[tgID] = &standing{name: name}
		}
	}

	for i, m := range matches {
		if m.Player1TgID != nil {
			ensure(*m.Player1TgID, m.Player1Name)
		}
		if m.Player2TgID != nil {
			ensure(*m.Player2TgID, m.Player2Name)
		}
		if m.Status == models.MatchStatusDone && m.WinnerTgID != nil {
			if s, ok := standings[*m.WinnerTgID]; ok {
				s.wins++
			}
		}
		sb.WriteString(fmt.Sprintf("%d. ", i+1))
		sb.WriteString(formatMatchLine(m))
	}

	// Standings
	sb.WriteString("\n📊 <b>Jadvali</b>\n")
	type row struct {
		tgID int64
		name string
		wins int
	}
	var rows []row
	for id, s := range standings {
		rows = append(rows, row{id, s.name, s.wins})
	}
	// Sort by wins descending
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[j].wins > rows[i].wins {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	for i, r := range rows {
		sb.WriteString(fmt.Sprintf("%d. %s — %d g'alaba\n", i+1, r.name, r.wins))
	}
	return sb.String()
}
