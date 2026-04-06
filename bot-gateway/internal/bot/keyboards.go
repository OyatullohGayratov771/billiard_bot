package bot

import (
	"fmt"
	"time"

	"bot-gateway/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ===================== MAIN MENU =====================

func mainMenuKeyboard(user *models.User) tgbotapi.ReplyKeyboardMarkup {
	var rows [][]tgbotapi.KeyboardButton

	switch user.Role {
	case models.RoleSuperadmin:
		rows = [][]tgbotapi.KeyboardButton{
			{btn("🎬 Klip so'rovlar"), btn("👥 Xodimlar")},
			{btn("⚙️ Sozlamalar")},
		}
	case models.RoleAdmin, models.RoleOperator:
		rows = [][]tgbotapi.KeyboardButton{
			{btn("🎬 Klip so'rovlar")},
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

	for i := 0; i < 7; i++ {
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
		if len(row) == 2 || i == 6 {
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

// clipHourKeyboard — soat tanlash (00-23, barcha soatlar)
func clipHourKeyboard() tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	row := []tgbotapi.InlineKeyboardButton{}

	for h := 0; h <= 23; h++ {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%02d:__", h),
			fmt.Sprintf("clip_hour:%d", h),
		))
		if len(row) == 6 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Orqaga", "clip_back:date"),
		tgbotapi.NewInlineKeyboardButtonData("❌ Bekor", "clip_cancel"),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// clipMinuteKeyboard — daqiqa tanlash (0-55, har 5 daqiqada)
func clipMinuteKeyboard(hour int) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	row := []tgbotapi.InlineKeyboardButton{}

	for m := 0; m < 60; m += 5 {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%02d:%02d", hour, m),
			fmt.Sprintf("clip_min:%d", m),
		))
		if len(row) == 4 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Orqaga", "clip_back:hour"),
		tgbotapi.NewInlineKeyboardButtonData("❌ Bekor", "clip_cancel"),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// clipDurationKeyboard — davomiylik tanlash (1-10 daqiqa)
func clipDurationKeyboard() tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	row := []tgbotapi.InlineKeyboardButton{}

	for d := 1; d <= 10; d++ {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%d daq", d),
			fmt.Sprintf("clip_dur:%d", d),
		))
		if len(row) == 5 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Orqaga", "clip_back:minute"),
		tgbotapi.NewInlineKeyboardButtonData("❌ Bekor", "clip_cancel"),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
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
