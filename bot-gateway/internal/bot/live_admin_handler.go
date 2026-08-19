package bot

import (
	"fmt"
	"log"
	"runtime/debug"
	"time"

	"bot-gateway/internal/client"
	"bot-gateway/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ===================== JONLI EFIR (LIVE) — ADMIN =====================

// cbLiveMenu — filial ro'yxatini ko'rsatadi (live boshqarish uchun).
func (h *Handler) cbLiveMenu(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	branches, err := h.tableSvc.GetBranches()
	if err != nil || len(branches) == 0 {
		send(bot, chatID, "❌ Filiallar topilmadi.")
		return
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, b := range branches {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏢 "+b.Name, fmt.Sprintf("live_branch:%d", b.ID)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Orqaga", "settings_back"),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	text := "📡 <b>Jonli efir</b>\n\nQaysi filial stollarini boshqarmoqchisiz?"
	if msgID > 0 {
		editMessage(bot, chatID, msgID, text, &kb)
	} else {
		sendWithKeyboard(bot, chatID, text, kb)
	}
}

// cbLiveBranch — filial stollarini holati bilan ko'rsatadi (🔴 live / ⚪ o'chiq).
func (h *Handler) cbLiveBranch(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, branchIDStr string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	branchID := mustParseInt64(branchIDStr)
	tables, err := h.tableSvc.GetBranchTables(branchID)
	if err != nil || len(tables) == 0 {
		send(bot, chatID, "❌ Stollar topilmadi.")
		return
	}
	active, _ := h.liveSvc.Active() // xato bo'lsa bo'sh map — hammasi ⚪ ko'rinadi, xavfsiz

	var rows [][]tgbotapi.InlineKeyboardButton
	liveCount := 0
	for _, t := range tables {
		icon := "⚪"
		if active[t.ID] {
			icon = "🔴"
			liveCount++
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%s %d-stol", icon, t.TableNum),
				fmt.Sprintf("live_toggle:%d", t.ID),
			),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Orqaga", "live_menu"),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	text := fmt.Sprintf(
		"📡 <b>Jonli efir</b> — %d ta stol yoniq\n\n🔴 — hozir jonli\n⚪ — o'chirilgan\n\nYoqish/o'chirish uchun stolni bosing:",
		liveCount,
	)
	editMessage(bot, chatID, msgID, text, &kb)
}

// cbLiveToggle — bosilgan stolning live holatini almashtiradi (yoqadi/o'chiradi).
func (h *Handler) cbLiveToggle(bot *tgbotapi.BotAPI, chatID int64, msgID int, user *models.User, tableIDStr string) {
	if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
		return
	}
	tableID := mustParseInt64(tableIDStr)
	table, err := h.tableSvc.GetTable(tableID)
	if err != nil {
		send(bot, chatID, "❌ Stol topilmadi.")
		return
	}

	active, _ := h.liveSvc.Active()
	if active[tableID] {
		if err := h.liveSvc.Stop(tableID); err != nil {
			send(bot, chatID, fmt.Sprintf("❌ To'xtatib bo'lmadi: %v", err))
		} else {
			send(bot, chatID, fmt.Sprintf(
				"⏹ <b>%d-stol</b> live'i to'xtatildi.\n\n<i>Yozuv tayyorlanmoqda, tayyor bo'lgach %s kanaliga avtomatik yuboriladi...</i>",
				table.TableNum, h.liveRecordingChannel))
			go h.finishRecordingUpload(bot, chatID, tableID, table.TableNum, table.BranchID)
		}
	} else {
		res, err := h.liveSvc.Start(tableID)
		if err != nil {
			send(bot, chatID, fmt.Sprintf(
				"❌ Boshlab bo'lmadi: %v\n\n<i>Kamera manzili sozlanganini tekshiring (⚙️ Sozlamalar → NVR/RTSP).</i>",
				err))
		} else {
			send(bot, chatID, fmt.Sprintf(
				"🔴 <b>%d-stol jonli efirga chiqdi!</b>\n\n"+
					"Tomoshabinlar shu havoladan ko'rishlari mumkin (login shart emas):\n%s",
				table.TableNum, res.LiveURL))
		}
	}
	h.cbLiveBranch(bot, chatID, msgID, user, fmt.Sprintf("%d", table.BranchID))
}

// finishRecordingUpload — live to'xtagach fon jarayonida ishlaydi: yozuv
// tayyor bo'lishini kutadi (live-service uni birlashtirib, kerak bo'lsa
// siqadi), so'ng Telegram kanaliga yuboradi va serverdagi nusxani tozalaydi.
func (h *Handler) finishRecordingUpload(bot *tgbotapi.BotAPI, chatID, tableID int64, tableNum int, branchID int64) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("⚠️ panic recovered (finishRecordingUpload): %v\n%s", r, debug.Stack())
		}
	}()

	const maxWait = 5 * time.Minute
	const pollInterval = 4 * time.Second
	deadline := time.Now().Add(maxWait)

	var status *client.RecordingStatus
	for time.Now().Before(deadline) {
		s, err := h.liveSvc.RecordingStatus(tableID)
		if err == nil && s.Exists && s.Ready {
			status = s
			break
		}
		time.Sleep(pollInterval)
	}

	if status == nil {
		send(bot, chatID, fmt.Sprintf("⚠️ %d-stol yozuvi tayyorlanishi juda uzoq davom etdi, kanalga yuborilmadi.", tableNum))
		return
	}
	if status.Error != "" {
		send(bot, chatID, fmt.Sprintf("⚠️ %d-stol yozuvini tayyorlashda xato: %s", tableNum, status.Error))
		_ = h.liveSvc.DeleteRecording(tableID)
		return
	}

	data, err := h.liveSvc.DownloadRecording(tableID)
	if err != nil || len(data) == 0 {
		send(bot, chatID, fmt.Sprintf("⚠️ %d-stol yozuvini yuklab bo'lmadi.", tableNum))
		return
	}

	branchName := ""
	if branch, err := h.tableSvc.GetBranchByID(branchID); err == nil {
		branchName = branch.Name
	}
	caption := fmt.Sprintf("🎱 <b>%d-stol</b>", tableNum)
	if branchName != "" {
		caption += " — " + esc(branchName)
	}
	caption += "\n📅 " + time.Now().Format("02.01.2006 15:04")

	video := tgbotapi.VideoConfig{
		BaseFile: tgbotapi.BaseFile{
			BaseChat: tgbotapi.BaseChat{ChannelUsername: h.liveRecordingChannel},
			File:     tgbotapi.FileBytes{Name: fmt.Sprintf("stol_%d_%d.mp4", tableNum, time.Now().Unix()), Bytes: data},
		},
		Caption:   caption,
		ParseMode: "HTML",
	}

	if _, err := bot.Send(video); err != nil {
		send(bot, chatID, fmt.Sprintf("⚠️ %d-stol yozuvini kanalga yuborib bo'lmadi: %v", tableNum, err))
		return
	}
	_ = h.liveSvc.DeleteRecording(tableID)
	send(bot, chatID, fmt.Sprintf("✅ %d-stol yozuvi %s kanaliga yuborildi.", tableNum, h.liveRecordingChannel))
}
