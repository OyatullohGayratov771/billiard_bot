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

// clipTimeSlotKeyboard — sana uchun 30 daqiqali vaqt slotlarini ko'rsatadi
func clipTimeSlotKeyboard(dateStr string, isToday bool) tgbotapi.InlineKeyboardMarkup {
	day, _ := time.Parse("02.01.2006", dateStr)

	startH, startM := 10, 0
	endT := time.Date(day.Year(), day.Month(), day.Day(), 23, 30, 0, 0, time.Local)
	if isToday {
		now := time.Now().Add(-5 * time.Minute)
		endT = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), (now.Minute()/30)*30, 0, 0, time.Local)
	}

	var slots []time.Time
	for t := time.Date(day.Year(), day.Month(), day.Day(), startH, startM, 0, 0, time.Local); !t.After(endT); t = t.Add(30 * time.Minute) {
		slots = append(slots, t)
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	row := []tgbotapi.InlineKeyboardButton{}
	for i := len(slots) - 1; i >= 0; i-- {
		t := slots[i]
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(
			t.Format("15:04"),
			fmt.Sprintf("clip_time:%02d:%02d", t.Hour(), t.Minute()),
		))
		if len(row) == 4 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚠️ Bugun hali vaqt yo'q", "clip_cancel"),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Orqaga", "clip_back:date"),
		tgbotapi.NewInlineKeyboardButtonData("❌ Bekor", "clip_cancel"),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// clipDurationKeyboard — davomiylik tanlash (1-5 daqiqa)
func clipDurationKeyboard() tgbotapi.InlineKeyboardMarkup {
	row := []tgbotapi.InlineKeyboardButton{}
	for d := 1; d <= 5; d++ {
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
			tgbotapi.NewInlineKeyboardButtonData("📤 Video yuborish (ruchnoy)",
				fmt.Sprintf("clip_upload:%d", clipID)),
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
