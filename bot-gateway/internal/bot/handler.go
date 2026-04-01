package bot

import (
	"log"
	"strings"

	"bot-gateway/internal/models"
	"bot-gateway/internal/repository"
	"bot-gateway/internal/service"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Handler — barcha bot handlerlarini birlashtiradi
type Handler struct {
	userRepo    *repository.UserRepo
	branchRepo  *repository.BranchRepo
	tableRepo   *repository.TableRepo
	sessionRepo *repository.SessionRepo
	clipRepo    *repository.ClipRepo
	auditRepo   *repository.AuditRepo

	userSvc  *service.UserService
	tableSvc *service.TableService
	clipSvc  *service.ClipService

	states *StateManager
}

func NewHandler(
	userRepo *repository.UserRepo,
	branchRepo *repository.BranchRepo,
	tableRepo *repository.TableRepo,
	sessionRepo *repository.SessionRepo,
	clipRepo *repository.ClipRepo,
	auditRepo *repository.AuditRepo,
	userSvc *service.UserService,
	tableSvc *service.TableService,
	clipSvc *service.ClipService,
) *Handler {
	return &Handler{
		userRepo:    userRepo,
		branchRepo:  branchRepo,
		tableRepo:   tableRepo,
		sessionRepo: sessionRepo,
		clipRepo:    clipRepo,
		auditRepo:   auditRepo,
		userSvc:     userSvc,
		tableSvc:    tableSvc,
		clipSvc:     clipSvc,
		states:      NewStateManager(),
	}
}

// Handle — asosiy dispatcher
func (h *Handler) Handle(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	// Callback query (inline keyboard)
	if update.CallbackQuery != nil {
		h.handleCallback(bot, update.CallbackQuery)
		return
	}

	// Oddiy xabar
	if update.Message != nil {
		h.handleMessage(bot, update.Message)
		return
	}
}

// handleMessage — matn xabarlarini qayta ishlaydi
func (h *Handler) handleMessage(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	tgID := msg.From.ID
	chatID := msg.Chat.ID

	// Foydalanuvchini ro'yxatdan o'tkaz yoki yangilash
	user, err := h.getOrRegister(tgID, msg.From.UserName, msg.From.FirstName, msg.From.LastName)
	if err != nil {
		log.Printf("getOrRegister error: %v", err)
		send(bot, chatID, "❌ Xatolik yuz berdi. Qayta urinib ko'ring.")
		return
	}

	// FSM holati bo'yicha tekshir
	state := h.states.Get(tgID)
	if state.State != StateIdle {
		h.handleStateInput(bot, msg, user, state)
		return
	}

	// Buyruqlar
	if msg.IsCommand() {
		switch msg.Command() {
		case "start":
			h.cmdStart(bot, msg, user)
		case "admin":
			h.cmdAdmin(bot, msg, user)
		case "cancel":
			h.states.Clear(tgID)
			sendWithKeyboard(bot, chatID, "✅ Bekor qilindi.", mainMenuKeyboard(user))
		default:
			send(bot, chatID, "❓ Noma'lum buyruq. /start bosing.")
		}
		return
	}

	// Reply keyboard tugmalar
	switch msg.Text {
	case "🎱 Stollar":
		h.showBranchesForStaff(bot, chatID, user)
	case "🏢 Filiallar":
		h.showBranchesForStaff(bot, chatID, user)
	case "📊 Hisobot":
		h.showReportBranches(bot, chatID, user)
	case "🎬 Klip so'rovlar":
		h.showPendingClips(bot, chatID, user)
	case "🎬 Klip so'rash":
		h.startClipRequest(bot, chatID, tgID)
	case "📋 Mening buyurtmalarim":
		h.showMyClips(bot, chatID, tgID)
	case "📋 Bugungi sessiyalar":
		h.showTodaySessions(bot, chatID, user)
	case "👥 Xodimlar":
		h.showStaffList(bot, chatID, user)
	case "⚙️ Sozlamalar":
		h.showSettings(bot, chatID, user)
	default:
		send(bot, chatID, "❓ Noma'lum buyruq. Pastdagi tugmalardan foydalaning.")
	}
}

// handleCallback — inline keyboard callback'larini qayta ishlaydi
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

	// Callback data'ni parse qil
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
	// ---- FILIALLAR ----
	case "branch":
		h.cbShowTables(bot, chatID, msgID, user, arg1)

	// ---- STOLLAR ----
	case "table":
		h.cbShowTableActions(bot, chatID, msgID, user, arg1)

	// ---- SESSIYA ----
	case "start_session":
		h.cbStartSessionPrompt(bot, chatID, tgID, arg1)
	case "end_session":
		h.cbEndSession(bot, chatID, msgID, user, arg1)
	case "view_session":
		h.cbViewSession(bot, chatID, msgID, user, arg1)

	// ---- BACK ----
	case "back":
		switch arg1 {
		case "branches":
			h.showBranchesForStaff(bot, chatID, user)
		case "tables":
			h.cbShowTablesInline(bot, chatID, msgID, user, arg2)
		}

	// ---- KLIP SO'RASH (mijoz) ----
	case "clip_branch":
		h.cbClipSelectBranch(bot, chatID, msgID, tgID, arg1)
	case "clip_table":
		h.cbClipSelectTable(bot, chatID, msgID, tgID, arg1)
	case "clip_dur":
		h.cbClipSelectDuration(bot, chatID, msgID, tgID, arg1)
	case "clip_back":
		h.cbClipBack(bot, chatID, msgID, tgID, arg1)
	case "clip_cancel":
		h.states.Clear(tgID)
		editMessage(bot, chatID, msgID, "❌ Klip so'rash bekor qilindi.", nil)

	// ---- ADMIN: KLIP ----
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

	// ---- HISOBOT ----
	case "report":
		h.cbShowReport(bot, chatID, msgID, user, arg1, arg2)

	// ---- XODIM ROLINI O'ZGARTIRISH ----
	case "set_role":
		h.cbSetRole(bot, chatID, msgID, user, arg1, arg2)
	case "admin_staff_list":
		h.showStaffList(bot, chatID, user)

	case "clip_detail":
		h.cbShowClipDetail(bot, chatID, msgID, user, arg1)

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
