package bot

import (
	"log"
	"strings"

	"bot-gateway/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Handler — barcha bot handlerlarini birlashtiradi
type Handler struct {
	userSvc  UserService
	tableSvc TableService
	clipSvc  ClipService

	states *StateManager
}

func NewHandler(
	userSvc UserService,
	tableSvc TableService,
	clipSvc ClipService,
) *Handler {
	return &Handler{
		userSvc:  userSvc,
		tableSvc: tableSvc,
		clipSvc:  clipSvc,
		states:   NewStateManager(),
	}
}

// Handle — asosiy dispatcher
func (h *Handler) Handle(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	if update.CallbackQuery != nil {
		h.handleCallback(bot, update.CallbackQuery)
		return
	}
	if update.Message != nil {
		h.handleMessage(bot, update.Message)
		return
	}
}

func (h *Handler) handleMessage(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	tgID := msg.From.ID
	chatID := msg.Chat.ID

	user, err := h.getOrRegister(tgID, msg.From.UserName, msg.From.FirstName, msg.From.LastName)
	if err != nil {
		log.Printf("getOrRegister error: %v", err)
		send(bot, chatID, "❌ Xatolik yuz berdi. Qayta urinib ko'ring.")
		return
	}

	state := h.states.Get(tgID)
	if state.State != StateIdle {
		h.handleStateInput(bot, msg, user, state)
		return
	}

	if msg.IsCommand() {
		switch msg.Command() {
		case "start":
			h.cmdStart(bot, msg, user)
		case "cancel":
			h.states.Clear(tgID)
			sendWithKeyboard(bot, chatID, "✅ Bekor qilindi.", mainMenuKeyboard(user))
		default:
			send(bot, chatID, "❓ Noma'lum buyruq. /start bosing.")
		}
		return
	}

	switch msg.Text {
	case "🎬 Klip so'rovlar":
		h.showPendingClips(bot, chatID, user)
	case "🎬 Klip so'rash":
		h.startClipRequest(bot, chatID, tgID)
	case "📋 Mening buyurtmalarim":
		h.showMyClips(bot, chatID, tgID)
	case "👥 Xodimlar":
		h.showStaffList(bot, chatID, user)
	case "⚙️ Sozlamalar":
		h.showSettings(bot, chatID, user)
	default:
		send(bot, chatID, "❓ Noma'lum buyruq. Pastdagi tugmalardan foydalaning.")
	}
}

func (h *Handler) handleCallback(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery) {
	tgID := cb.From.ID
	chatID := cb.Message.Chat.ID
	msgID := cb.Message.MessageID
	data := cb.Data

	user := h.authUser(tgID)
	if user == nil {
		answerCallback(bot, cb.ID, "⛔ Avval /start bosing")
		return
	}

	answerCallback(bot, cb.ID, "")

	parts := strings.SplitN(data, ":", 3)
	action := parts[0]
	arg1 := ""
	arg2 := ""
	if len(parts) > 1 {
		arg1 = parts[1]
	}
	if len(parts) > 2 {
		arg2 = parts[2]
	}

	switch action {
	case "clip_branch":
		h.cbClipSelectBranch(bot, chatID, msgID, tgID, arg1)
	case "clip_table":
		h.cbClipSelectTable(bot, chatID, msgID, tgID, arg1)
	case "clip_date":
		h.cbClipSelectDate(bot, chatID, msgID, tgID, arg1)
	case "clip_hour":
		h.cbClipSelectHour(bot, chatID, msgID, tgID, arg1)
	case "clip_min":
		h.cbClipSelectMinute(bot, chatID, msgID, tgID, arg1)
	case "clip_dur":
		h.cbClipSelectDuration(bot, chatID, msgID, tgID, arg1)
	case "clip_back":
		h.cbClipBack(bot, chatID, msgID, tgID, arg1)
	case "clip_pay_confirm":
		h.cbClipPayConfirm(bot, chatID, msgID, tgID)
	case "clip_cancel":
		h.states.Clear(tgID)
		editMessage(bot, chatID, msgID, "❌ Klip so'rash bekor qilindi.", nil)
	case "admin_confirm_pay":
		h.cbAdminConfirmPayment(bot, chatID, msgID, user, arg1)
	case "admin_clip_done":
		h.cbAdminClipDone(bot, chatID, msgID, user, arg1)
	case "admin_clip_fail":
		h.cbAdminClipFail(bot, chatID, msgID, user, arg1)
	case "admin_refund":
		h.cbAdminRefund(bot, chatID, msgID, user, arg1)
	case "admin_clips_list":
		h.showPendingClips(bot, chatID, user)
	case "set_role":
		h.cbSetRole(bot, chatID, msgID, user, arg1, arg2)
	case "admin_staff_list":
		h.showStaffList(bot, chatID, user)
	case "clip_detail":
		h.cbShowClipDetail(bot, chatID, msgID, user, arg1)
	case "clip_record":
		h.cbRecordClip(bot, chatID, msgID, user, arg1)
	case "clip_upload":
		h.cbAdminManualUpload(bot, chatID, tgID, user, arg1)
	case "nvr_setup":
		if !h.requireRole(bot, chatID, user, models.RoleSuperadmin) {
			return
		}
		h.cbNVRSetup(bot, chatID, tgID, arg1)
	case "rtsp_branch":
		if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
			return
		}
		h.cbRTSPBranch(bot, chatID, msgID, arg1)
	case "rtsp_table":
		if !h.requireRole(bot, chatID, user, models.RoleSuperadmin, models.RoleAdmin) {
			return
		}
		h.cbRTSPTable(bot, chatID, tgID, arg1)
	case "settings_back":
		h.showSettings(bot, chatID, user)

	case "add_staff":
		if !h.requireRole(bot, chatID, user, models.RoleSuperadmin) {
			return
		}
		h.states.Set(tgID, StateAddStaffID)
		send(bot, chatID, "👤 Xodimning Telegram ID sini kiriting:\n\n<i>ID ni bilish uchun @userinfobot ga /start yozing</i>")
	default:
		log.Printf("unknown callback: %s", data)
	}
}
