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

	header := "🏆 <b>Faol Turnirlar</b>\n\n"
	var rows [][]tgbotapi.InlineKeyboardButton

	if len(tournaments) == 0 {
		header += "Hozirda faol turnirlar yo'q.\nKeyinroq qaytib keling! 👀"
	} else {
		for _, t := range tournaments {
			label := fmt.Sprintf("🏆 %s — %s (%s so'm)",
				t.Name,
				t.ScheduledAt.Format("02.01 15:04"),
				formatTrnPrice(t.Price),
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

	text := fmt.Sprintf(
		"🏆 <b>%s</b>\n\n"+
			"📍 Filial: %s\n"+
			"🎱 Stol: %s\n"+
			"📅 Sana: <b>%s</b>\n"+
			"💰 Qatnashish: <b>%s so'm</b>\n"+
			"👥 O'rinlar: <b>%d / %d</b>\n"+
			"📊 Holat: %s",
		t.Name,
		t.BranchName,
		tableInfo,
		t.ScheduledAt.Format("02.01.2006 15:04"),
		formatTrnPrice(t.Price),
		t.ApprovedCount, t.MaxPlayers,
		tournamentStatusText(t.Status),
	)

	var rows [][]tgbotapi.InlineKeyboardButton

	reg, _ := h.tournamentSvc.GetUserRegistration(trnID, tgID)

	if t.Status == models.TournamentStatusRegistration {
		if reg == nil {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Ro'yxatdan o'tish",
					fmt.Sprintf("trn_register:%d", trnID)),
			))
		} else {
			switch reg.Status {
			case models.RegStatusPending:
				text += "\n\n⏳ <i>Sizning so'rovingiz tekshirilmoqda...</i>"
			case models.RegStatusApproved:
				text += "\n\n✅ <i>Siz ro'yxatdan o'tgansiz!</i>"
				rows = append(rows, tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("🏆 Bracket",
						fmt.Sprintf("trn_bracket:%d", trnID)),
				))
			case models.RegStatusRejected:
				text += "\n\n❌ <i>Sizning so'rovingiz rad etildi.</i>"
			}
		}
	} else if t.Status == models.TournamentStatusInProgress {
		if reg != nil && reg.Status == models.RegStatusApproved {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🏆 Bracket ko'rish",
					fmt.Sprintf("trn_bracket:%d", trnID)),
			))
		}
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

	_, err := h.tournamentSvc.Register(trnID, user.TelegramID, user.DisplayName())
	if err != nil {
		send(bot, chatID, fmt.Sprintf("❌ %v", err))
		return
	}

	t, _ := h.tournamentSvc.GetTournament(trnID)
	trnName := fmt.Sprintf("#%d", trnID)
	if t != nil {
		trnName = t.Name
	}

	admins, _ := h.userSvc.ListStaff()
	adminText := fmt.Sprintf(
		"🔔 <b>Yangi turnir so'rovi</b>\n\n"+
			"🏆 %s\n"+
			"👤 %s  (<code>%d</code>)\n\n"+
			"Admin tasdiqlashi kerak.",
		trnName, user.DisplayName(), user.TelegramID,
	)
	for _, a := range admins {
		if a.Role == models.RoleSuperadmin || a.Role == models.RoleAdmin {
			send(bot, a.TelegramID, adminText)
		}
	}

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Turnirga qaytish",
				fmt.Sprintf("trn_detail:%d", trnID)),
		),
	)
	editMessage(bot, chatID, msgID, fmt.Sprintf(
		"✅ <b>So'rovingiz yuborildi!</b>\n\n"+
			"🏆 %s\n\n"+
			"⏳ Admin tasdiqlashini kuting.\n"+
			"Natija haqida sizga xabar beriladi.",
		trnName,
	), &kb)
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
		sb.WriteString(fmt.Sprintf("%s Turnir #%d — %s\n",
			icon, r.TournamentID, r.RegisteredAt.Format("02.01.2006")))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%s Turnir #%d", icon, r.TournamentID),
				fmt.Sprintf("trn_detail:%d", r.TournamentID),
			),
		))
	}

	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	sendWithKeyboard(bot, chatID, sb.String(), kb)
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
