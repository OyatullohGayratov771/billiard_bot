package bot

import (
	"fmt"
	"strings"

	"bot-gateway/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ===================== TURNIR — MIJOZ =====================

func (h *Handler) showClientTournamentList(bot *tgbotapi.BotAPI, chatID int64, tgID int64) {
	tournaments, err := h.tournamentSvc.ListTournaments(models.TournamentStatusRegistration)
	if err != nil {
		send(bot, chatID, "❌ Xatolik.")
		return
	}

	header := "🏆 <b>Billiard King — Turnirlar</b>\n\n"
	var rows [][]tgbotapi.InlineKeyboardButton

	if len(tournaments) == 0 {
		header += "Hozirda faol turnirlar yo'q.\n\nKuzatib boring — yangi turnirlar tez orada e'lon qilinadi! 🎱"
	} else {
		for _, t := range tournaments {
			label := fmt.Sprintf("🏆 %s — %s (%d/%d)",
				t.Name,
				t.ScheduledAt.In(localTZ).Format("02.01 15:04"),
				t.ApprovedCount, t.MaxPlayers,
			)
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("trn_detail:%d", t.ID)),
			))
		}
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📋 Mening turnirlarim", "trn_my"),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendWithKeyboard(bot, chatID, header, kb)
}

func (h *Handler) cbClientTrnDetail(bot *tgbotapi.BotAPI, chatID int64, msgID int, tgID int64, trnIDStr string) {
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

	lockIcon := ""
	if t.JoinCode != "" {
		lockIcon = " 🔒"
	}
	text := fmt.Sprintf(
		"🏆 <b>%s</b>%s\n\n"+
			"📍 Filial: %s\n"+
			"🎱 Stol: %s\n"+
			"📅 Sana: <b>%s</b>\n"+
			"👥 O'rinlar: <b>%d / %d</b>\n"+
			"🎮 Tur: %s\n"+
			"📊 Holat: %s",
		t.Name, lockIcon,
		t.BranchName,
		tableInfo,
		t.ScheduledAt.In(localTZ).Format("02.01.2006 15:04"),
		t.ApprovedCount, t.MaxPlayers,
		tournamentTypeText(t.Type),
		tournamentStatusText(t.Status),
	)

	var rows [][]tgbotapi.InlineKeyboardButton

	reg, _ := h.tournamentSvc.GetUserRegistration(trnID, tgID)

	switch t.Status {
	case models.TournamentStatusRegistration:
		if reg == nil || reg.Status == models.RegStatusRejected {
			if reg != nil && reg.Status == models.RegStatusRejected {
				text += "\n\n❌ <i>So'rovingiz rad etildi. Qaytadan urinib ko'ring yoki boshqa turnirga qatnashing!</i>"
			}
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Ro'yxatdan o'tish",
					fmt.Sprintf("trn_register:%d", trnID)),
			))
		} else {
			switch reg.Status {
			case models.RegStatusPending:
				text += "\n\n⏳ <i>So'rovingiz adminlar tomonidan ko'rib chiqilmoqda. Sabr qiling!</i>"
			case models.RegStatusApproved:
				text += "\n\n✅ <i>Siz turnirga qabul qilindingiz! Omad tilaymiz 🍀</i>"
			}
		}

	case models.TournamentStatusInProgress:
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏆 Bracket ko'rish",
				fmt.Sprintf("trn_bracket:%d", trnID)),
		))

	case models.TournamentStatusFinished:
		text += "\n\n🏆 <i>Turnir yakunlandi! Barcha ishtirokchilarga rahmat.</i>"
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏆 Natijalarni ko'rish",
				fmt.Sprintf("trn_bracket:%d", trnID)),
		))

	case models.TournamentStatusCancelled:
		text += "\n\n❌ <i>Bu turnir bekor qilindi. Boshqa turnirlarni kuzatib boring! 🎱</i>"
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Turnirlar", "trn_list"),
	))

	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	if msgID > 0 {
		editMessage(bot, chatID, msgID, text, &kb)
	} else {
		sendWithKeyboard(bot, chatID, text, kb)
	}
}

func (h *Handler) cbClientRegister(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, trnIDStr string) {
	trnID := mustParseInt64(trnIDStr)

	t, err := h.tournamentSvc.GetTournament(trnID)
	if err != nil {
		send(bot, chatID, "❌ Turnir topilmadi.")
		return
	}

	if t.JoinCode != "" {
		h.states.Set(user.TelegramID, StateTrnJoinCode)
		h.states.SetData(user.TelegramID, "trn_join_id", trnID)
		kb := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Bekor", fmt.Sprintf("trn_detail:%d", trnID)),
			),
		)
		editMessage(bot, chatID, msgID,
			"🔑 <b>Bu turnir maxfiy kod bilan himoyalangan.</b>\n\nKodni kiriting:", &kb)
		return
	}

	h.finishTrnRegister(bot, chatID, msgID, user, trnID, "")
}

func (h *Handler) handleTrnJoinCodeInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, user *models.User) {
	tgID := msg.From.ID
	chatID := msg.Chat.ID
	code := strings.TrimSpace(msg.Text)
	trnID, _ := h.states.GetInt64(tgID, "trn_join_id")
	h.states.Clear(tgID)
	h.finishTrnRegister(bot, chatID, 0, user, trnID, code)
}

func (h *Handler) finishTrnRegister(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, trnID int64, joinCode string) {
	reg, err := h.tournamentSvc.Register(trnID, user.TelegramID, user.DisplayName(), joinCode)
	if err != nil {
		if msgID > 0 {
			send(bot, chatID, fmt.Sprintf("❌ %v", err))
		} else {
			send(bot, chatID, fmt.Sprintf("❌ %v", err))
		}
		return
	}

	t, _ := h.tournamentSvc.GetTournament(trnID)
	trnName := fmt.Sprintf("#%d", trnID)
	if t != nil {
		trnName = t.Name
	}

	admins, _ := h.userSvc.ListStaff()

	fullName := user.FirstName
	if user.LastName != "" {
		fullName += " " + user.LastName
	}
	if fullName == "" {
		fullName = user.DisplayName()
	}
	usernameInfo := ""
	if user.Username != "" {
		usernameInfo = "  @" + user.Username
	}
	phoneInfo := ""
	if user.Phone != "" {
		phoneInfo = "\n📱 " + user.Phone
	}

	adminText := fmt.Sprintf(
		"🔔 <b>Yangi turnir so'rovi</b>\n\n"+
			"🏆 <b>%s</b>\n"+
			"👤 <b>%s</b>%s"+
			"%s\n"+
			"🆔 <code>%d</code>\n\n"+
			"Tasdiqlash yoki rad etishingiz mumkin:",
		trnName, fullName, usernameInfo, phoneInfo, user.TelegramID,
	)
	adminKb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Tasdiqlash",
				fmt.Sprintf("admin_trn_approve:%d:%d", trnID, reg.ID)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Rad etish",
				fmt.Sprintf("admin_trn_reject:%d:%d", trnID, reg.ID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("✉️ Xabar yozish",
				fmt.Sprintf("tg://user?id=%d", user.TelegramID)),
			tgbotapi.NewInlineKeyboardButtonData("👥 Barcha so'rovlar",
				fmt.Sprintf("admin_trn_regs:%d", trnID)),
		),
	)
	for _, a := range admins {
		if a.Role == models.RoleSuperadmin || a.Role == models.RoleAdmin {
			sendWithKeyboard(bot, a.TelegramID, adminText, adminKb)
		}
	}

	confirmText := fmt.Sprintf(
		"✅ <b>So'rovingiz muvaffaqiyatli yuborildi!</b>\n\n"+
			"🏆 <b>%s</b>\n\n"+
			"⏳ Admin so'rovingizni ko'rib chiqmoqda.\n"+
			"Qaror haqida sizga darhol xabar beriladi. Sabr qiling! 🎱",
		trnName,
	)
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Turnirga qaytish",
				fmt.Sprintf("trn_detail:%d", trnID)),
		),
	)
	if msgID > 0 {
		editMessage(bot, chatID, msgID, confirmText, &kb)
	} else {
		sendWithKeyboard(bot, chatID, confirmText, kb)
	}
}

func (h *Handler) showMyTournaments(bot *tgbotapi.BotAPI, chatID int64, tgID int64) {
	regs, err := h.tournamentSvc.GetUserTournaments(tgID)
	if err != nil {
		send(bot, chatID, "❌ Xatolik.")
		return
	}

	if len(regs) == 0 {
		send(bot, chatID,
			"🏆 Siz hali hech qanday turnirga ro'yxatdan o'tmagansiz.\n\n"+
				"<b>🏆 Turnirlar</b> tugmasidan faol turnirlarni ko'ring.")
		return
	}

	var sb strings.Builder
	sb.WriteString("🏆 <b>Mening turnirlarim</b>\n\n")

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, r := range regs {
		icon := regStatusIcon(r.Status)
		name := r.TournamentName
		if name == "" {
			name = fmt.Sprintf("Turnir #%d", r.TournamentID)
		}
		sb.WriteString(fmt.Sprintf("%s %s — %s\n",
			icon, name, r.RegisteredAt.In(localTZ).Format("02.01.2006")))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%s %s", icon, name),
				fmt.Sprintf("trn_detail:%d", r.TournamentID),
			),
		))
	}

	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendWithKeyboard(bot, chatID, sb.String(), kb)
}

func (h *Handler) showTournamentHistory(bot *tgbotapi.BotAPI, chatID int64) {
	tournaments, err := h.tournamentSvc.ListTournaments("")
	if err != nil {
		send(bot, chatID, "❌ Xatolik.")
		return
	}

	header := "📋 <b>Turnir tarixi</b>\n\n"
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, t := range tournaments {
		if t.Status == models.TournamentStatusRegistration {
			continue
		}
		icon := tournamentStatusIcon(t.Status)
		label := fmt.Sprintf("%s %s — %s", icon, t.Name, t.ScheduledAt.In(localTZ).Format("02.01.2006"))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("trn_detail:%d", t.ID)),
		))
	}

	if len(rows) == 0 {
		header += "Hali yakunlangan yoki bekor qilingan turnirlar yo'q."
	}

	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendWithKeyboard(bot, chatID, header, kb)
}

func (h *Handler) cbClientBracket(bot *tgbotapi.BotAPI, chatID int64, msgID int, tgID int64, trnIDStr string) {
	trnID := mustParseInt64(trnIDStr)

	matches, err := h.tournamentSvc.GetBracket(trnID)
	if err != nil || len(matches) == 0 {
		send(bot, chatID, "❌ Bracket hali tayyor emas.")
		return
	}

	text := fmt.Sprintf("🏆 <b>Turnir #%d — Bracket</b>\n", trnID)
	text += formatBracket(matches)

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Turnirga qaytish",
				fmt.Sprintf("trn_detail:%d", trnID)),
		),
	)
	if msgID > 0 {
		editMessage(bot, chatID, msgID, text, &kb)
	} else {
		sendWithKeyboard(bot, chatID, text, kb)
	}
}
