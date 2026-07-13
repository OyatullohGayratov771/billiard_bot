package bot

import (
	"fmt"
	"time"

	"bot-gateway/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ===================== MAIN MENU =====================

func mainMenuKeyboard(user *models.User) replyKeyboard {
	var rows [][]replyBtn
	switch user.Role {
	case models.RoleSuperadmin:
		rows = [][]replyBtn{
			{btn("🎬 Kliplar"), btn("🏆 Turnirlar")},
			{btn("🛒 Do'kon"), btn("👥 Xodimlar")},
			{btn("⚙️ Sozlamalar")},
		}
	case models.RoleAdmin:
		rows = [][]replyBtn{
			{btn("🎬 Kliplar"), btn("🏆 Turnirlar")},
			{btn("🛒 Do'kon")},
		}
	case models.RoleOperator:
		rows = [][]replyBtn{
			{btn("🎬 Kliplar")},
		}
	default: // client
		rows = [][]replyBtn{
			{btn("🎬 Kliplar"), btn("🏆 Turnirlar")},
			{btn("🛒 Do'kon"), btn("👤 Akkaunt")},
		}
	}

	return replyKeyboard{Keyboard: rows, ResizeKeyboard: true}
}

// isMainMenuButton — matn asosiy menyu tugmalaridan birimi? (holatdan qochish uchun)
func isMainMenuButton(s string) bool {
	switch s {
	case "🎬 Kliplar", "🏆 Turnirlar", "🛒 Do'kon", "👥 Xodimlar", "⚙️ Sozlamalar", "👤 Akkaunt":
		return true
	}
	return false
}

func btn(text string) replyBtn {
	return replyBtn{Text: text}
}

func webAppBtn(text, url string) replyBtn {
	return replyBtn{Text: text, WebApp: &webAppInfo{URL: url}}
}

// replyBtn is a KeyboardButton superset that includes web_app (not in v5.5.1).
type replyBtn struct {
	Text            string       `json:"text"`
	RequestContact  bool         `json:"request_contact,omitempty"`
	RequestLocation bool         `json:"request_location,omitempty"`
	WebApp          *webAppInfo  `json:"web_app,omitempty"`
}

type webAppInfo struct {
	URL string `json:"url"`
}

type replyKeyboard struct {
	Keyboard        [][]replyBtn `json:"keyboard"`
	ResizeKeyboard  bool         `json:"resize_keyboard,omitempty"`
	OneTimeKeyboard bool         `json:"one_time_keyboard,omitempty"`
}

// ===================== CONFIRM / CANCEL =====================

func confirmKeyboard(confirmCB, cancelCB string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Ha", confirmCB),
			tgbotapi.NewInlineKeyboardButtonData("❌ Yo'q", cancelCB),
		),
	)
}

// ===================== CLIP REQUEST — KALENDAR =====================

// clipDateKeyboard — oxirgi 7 kunni tugma sifatida ko'rsatadi
func clipDateKeyboard() tgbotapi.InlineKeyboardMarkup {
	now := time.Now()
	var rows [][]tgbotapi.InlineKeyboardButton
	row := []tgbotapi.InlineKeyboardButton{}

	for i := 0; i < 6; i++ {
		day := now.AddDate(0, 0, -i)
		label := day.Format("02.01 Mon")
		switch i {
		case 0:
			label = "📅 Bugun " + day.Format("02.01")
		case 1:
			label = "📅 Kecha " + day.Format("02.01")
		default:
			label = "📅 " + day.Format("02.01")
		}
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(
			label, fmt.Sprintf("clip_date:%s", day.Format("02.01.2006")),
		))
		if len(row) == 2 || i == 5 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Orqaga", "clip_back:table"),
		tgbotapi.NewInlineKeyboardButtonData("❌ Bekor", "clip_cancel"),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// clipTimeSpinnerKeyboard — soat:daqiqa spinner (▲/▼ tugmalar)
func clipTimeSpinnerKeyboard(hour, minute int) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("soat (±1)", "clip_noop"),
			tgbotapi.NewInlineKeyboardButtonData("daqiqa (±5)", "clip_noop"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("▲", "clip_time_adj:+h"),
			tgbotapi.NewInlineKeyboardButtonData("▲", "clip_time_adj:+m"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("  %02d  ", hour), "clip_noop"),
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("  %02d  ", minute), "clip_noop"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("▼", "clip_time_adj:-h"),
			tgbotapi.NewInlineKeyboardButtonData("▼", "clip_time_adj:-m"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Tasdiqlash", "clip_time_ok"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Orqaga", "clip_back:date"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Bekor", "clip_cancel"),
		),
	)
}

// clipDurationKeyboard — davomiylik tanlash (1-3 daqiqa)
func clipDurationKeyboard() tgbotapi.InlineKeyboardMarkup {
	row := []tgbotapi.InlineKeyboardButton{}
	for d := 1; d <= 3; d++ {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%d daq", d),
			fmt.Sprintf("clip_dur:%d", d),
		))
	}
	return tgbotapi.NewInlineKeyboardMarkup(
		row,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Orqaga", "clip_back:time"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Bekor", "clip_cancel"),
		),
	)
}

// ===================== CLIP REQUEST =====================

func clipBranchKeyboard(branches []*models.Branch) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, b := range branches {
		label := fmt.Sprintf("🏢 %s", b.Name)
		if b.NVRHost == "" {
			label = fmt.Sprintf("🚧 %s (tez orada)", b.Name)
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("clip_branch:%d", b.ID)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("❌ Bekor qilish", "clip_cancel"),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func clipTablesKeyboard(tables []*models.Table, branchID int64) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	row := []tgbotapi.InlineKeyboardButton{}

	for i, t := range tables {
		label := fmt.Sprintf("🎱 %d-stol", t.TableNum)
		cb := fmt.Sprintf("clip_table:%d", t.ID)
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, cb))

		if (i+1)%3 == 0 || i == len(tables)-1 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Orqaga", fmt.Sprintf("clip_back:branch")),
		tgbotapi.NewInlineKeyboardButtonData("❌ Bekor", "clip_cancel"),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// ===================== ADMIN: CLIP REQUESTS =====================

func statusIcon(status string) string {
	switch status {
	case models.ClipStatusPending:
		return "⏳"
	case models.ClipStatusPaid:
		return "💰"
	case models.ClipStatusProcessing:
		return "⚙️"
	case models.ClipStatusDone:
		return "✅"
	case models.ClipStatusFailed:
		return "❌"
	case models.ClipStatusRefunded:
		return "↩️"
	}
	return "•"
}

func clipRequestActionsKeyboard(clipID int64, status string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	switch status {
	case models.ClipStatusPending:
		rows = append(rows,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ To'lovni tasdiqlash",
					fmt.Sprintf("admin_confirm_pay:%d", clipID)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Rad etish",
					fmt.Sprintf("admin_clip_fail:%d", clipID)),
				tgbotapi.NewInlineKeyboardButtonData("↩️ Pul qaytarish",
					fmt.Sprintf("admin_refund:%d", clipID)),
			),
		)
	case models.ClipStatusPaid:
		rows = append(rows,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📹 NVR dan yozish",
					fmt.Sprintf("clip_record:%d", clipID)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📤 Qo'lda yuborish",
					fmt.Sprintf("clip_upload:%d", clipID)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("↩️ Pul qaytarish",
					fmt.Sprintf("admin_refund:%d", clipID)),
			),
		)
	case models.ClipStatusProcessing:
		rows = append(rows,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔄 Yangilash",
					fmt.Sprintf("clip_detail:%d", clipID)),
			),
		)
	case models.ClipStatusFailed:
		rows = append(rows,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔄 Qayta urinish",
					fmt.Sprintf("clip_retry:%d", clipID)),
				tgbotapi.NewInlineKeyboardButtonData("📤 Qo'lda yuborish",
					fmt.Sprintf("clip_upload:%d", clipID)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("↩️ Pul qaytarish",
					fmt.Sprintf("admin_refund:%d", clipID)),
			),
		)
	default:
		// done, refunded — faqat navigatsiya
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Faol so'rovlar", "admin_clips_list"),
		tgbotapi.NewInlineKeyboardButtonData("📋 Barcha", "admin_all_clips"),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// ===================== TOURNAMENT — ADMIN DETAIL =====================

func adminTournamentDetailKeyboard(t *models.Tournament) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	switch t.Status {
	case models.TournamentStatusRegistration:
		rows = append(rows,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("👥 Ro'yxat",
					fmt.Sprintf("admin_trn_regs:%d", t.ID)),
				tgbotapi.NewInlineKeyboardButtonData("✏️ Tahrirlash",
					fmt.Sprintf("admin_trn_edit:%d", t.ID)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("➕ O'yinchi qo'shish",
					fmt.Sprintf("admin_trn_add_player:%d", t.ID)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🎲 Bracket (yangilash)",
					fmt.Sprintf("admin_trn_bracket:%d", t.ID)),
				tgbotapi.NewInlineKeyboardButtonData("🔀 Aralashtirish",
					fmt.Sprintf("admin_trn_shuffle:%d", t.ID)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🎮 O'yinlarni boshqarish",
					fmt.Sprintf("admin_trn_result:%d", t.ID)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Bekor qilish",
					fmt.Sprintf("admin_trn_cancel:%d", t.ID)),
				tgbotapi.NewInlineKeyboardButtonData("🗑 O'chirish",
					fmt.Sprintf("admin_trn_delete:%d", t.ID)),
			),
		)
	case models.TournamentStatusInProgress:
		rows = append(rows,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🎮 O'yinlarni boshqarish",
					fmt.Sprintf("admin_trn_result:%d", t.ID)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("👥 Ishtirokchilar",
					fmt.Sprintf("admin_trn_regs:%d", t.ID)),
			),
		)
	case models.TournamentStatusCancelled, models.TournamentStatusFinished:
		rows = append(rows,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🗑 O'chirish",
					fmt.Sprintf("admin_trn_delete:%d", t.ID)),
			),
		)
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Turnirlar", "admin_trn_list"),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// ===================== CLIENT SUBMENUS =====================

func clipClientSubmenu() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🎬 Klip so'rash", "clip_menu_request"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Mening kliplarim", "clip_menu_my"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Yopish", "client_menu_close"),
		),
	)
}

func tournamentClientSubmenu() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏆 Faol turnirlar", "trn_menu_list"),
			tgbotapi.NewInlineKeyboardButtonData("🥇 Mening turnirlar", "trn_menu_my"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Tarix", "trn_history"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Yopish", "client_menu_close"),
		),
	)
}

// ===================== STAFF MANAGEMENT =====================

func staffRoleKeyboard(targetTgID int64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👑 Superadmin",
				fmt.Sprintf("set_role:%d:superadmin", targetTgID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔧 Admin",
				fmt.Sprintf("set_role:%d:admin", targetTgID)),
			tgbotapi.NewInlineKeyboardButtonData("🎮 Operator",
				fmt.Sprintf("set_role:%d:operator", targetTgID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👤 Client (o'chirish)",
				fmt.Sprintf("set_role:%d:client", targetTgID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Bekor", "admin_staff_list"),
		),
	)
}
