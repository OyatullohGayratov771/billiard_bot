package bot

import (
	"fmt"

	"bot-gateway/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ===================== MAIN MENU =====================

func mainMenuKeyboard(user *models.User) tgbotapi.ReplyKeyboardMarkup {
	var rows [][]tgbotapi.KeyboardButton

	switch user.Role {
	case models.RoleSuperadmin:
		rows = [][]tgbotapi.KeyboardButton{
			{btn("🏢 Filiallar"), btn("👥 Xodimlar")},
			{btn("🎬 Klip so'rovlar"), btn("📊 Hisobot")},
			{btn("⚙️ Sozlamalar")},
		}
	case models.RoleAdmin:
		rows = [][]tgbotapi.KeyboardButton{
			{btn("🎱 Stollar"), btn("📊 Hisobot")},
			{btn("🎬 Klip so'rovlar")},
		}
	case models.RoleOperator:
		rows = [][]tgbotapi.KeyboardButton{
			{btn("🎱 Stollar")},
			{btn("📋 Bugungi sessiyalar")},
		}
	default:
		rows = [][]tgbotapi.KeyboardButton{
			{btn("🎬 Klip so'rash"), btn("📋 Mening buyurtmalarim")},
		}
	}

	return tgbotapi.NewReplyKeyboard(rows...)
}

func btn(text string) tgbotapi.KeyboardButton {
	return tgbotapi.NewKeyboardButton(text)
}

// ===================== BRANCH SELECTION =====================

func branchesKeyboard(branches []*models.Branch) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, b := range branches {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("🏢 %s", b.Name),
				fmt.Sprintf("branch:%d", b.ID),
			),
		))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// ===================== TABLES =====================

func tablesKeyboard(tables []*models.Table) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	row := []tgbotapi.InlineKeyboardButton{}

	for i, t := range tables {
		icon := "🟢"
		if t.Status == models.TableStatusBusy {
			icon = "🔴"
		}
		label := fmt.Sprintf("%s %d", icon, t.TableNum)
		cb := fmt.Sprintf("table:%d", t.ID)
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, cb))

		// 3 ta stoldan keyin yangi qator
		if (i+1)%3 == 0 || i == len(tables)-1 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Orqaga", "back:branches"),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// ===================== TABLE ACTIONS =====================

func tableActionsKeyboard(table *models.Table) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	if table.Status == models.TableStatusFree {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"▶️ Sessiya boshlash",
				fmt.Sprintf("start_session:%d", table.ID),
			),
		))
	} else {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"⏹ Sessiya yakunlash",
				fmt.Sprintf("end_session:%d", table.ID),
			),
		))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"⏱ Vaqtni ko'rish",
				fmt.Sprintf("view_session:%d", table.ID),
			),
		))
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(
			"🔙 Orqaga",
			fmt.Sprintf("back:tables:%d", table.BranchID),
		),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
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

// ===================== CLIP REQUEST =====================

func clipBranchKeyboard(branches []*models.Branch) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, b := range branches {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("🏢 %s", b.Name),
				fmt.Sprintf("clip_branch:%d", b.ID),
			),
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

func clipDurationKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("30 soniya", "clip_dur:30"),
			tgbotapi.NewInlineKeyboardButtonData("1 daqiqa", "clip_dur:60"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("2 daqiqa", "clip_dur:120"),
			tgbotapi.NewInlineKeyboardButtonData("3 daqiqa", "clip_dur:180"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Orqaga", "clip_back:table"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Bekor", "clip_cancel"),
		),
	)
}

// ===================== ADMIN: CLIP REQUESTS =====================

func clipRequestActionsKeyboard(clipID int64, status string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	if status == models.ClipStatusPending {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ To'lovni tasdiqlash",
				fmt.Sprintf("admin_confirm_pay:%d", clipID)),
		))
	}
	if status == models.ClipStatusPaid {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📹 NVR dan yozish",
				fmt.Sprintf("clip_record:%d", clipID)),
		))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Klip yuborildi (Done)",
				fmt.Sprintf("admin_clip_done:%d", clipID)),
		))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Muvaffaqiyatsiz",
				fmt.Sprintf("admin_clip_fail:%d", clipID)),
		))
	}
	if status == models.ClipStatusProcessing {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Holat yangilash",
				fmt.Sprintf("clip_detail:%d", clipID)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("↩️ Qaytarish (Refund)",
			fmt.Sprintf("admin_refund:%d", clipID)),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Ro'yxat", "admin_clips_list"),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// ===================== REPORT =====================

func reportKeyboard(branchID int64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📅 Bugun",
				fmt.Sprintf("report:day:%d", branchID)),
			tgbotapi.NewInlineKeyboardButtonData("📆 Oy",
				fmt.Sprintf("report:month:%d", branchID)),
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
