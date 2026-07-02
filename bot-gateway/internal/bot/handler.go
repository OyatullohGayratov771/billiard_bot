package bot

import (
	"log"
	"strings"

	"bot-gateway/internal/client"
	"bot-gateway/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Handler — barcha bot handlerlarini birlashtiradi
type Handler struct {
	userSvc       UserService
	tableSvc      TableService
	clipSvc       ClipService
	tournamentSvc TournamentService
	productSvc    *client.ProductClient

	states    *StateManager
	userCache *UserCache
	limiter   *rateLimiter
}

func NewHandler(
	userSvc UserService,
	tableSvc TableService,
	clipSvc ClipService,
	tournamentSvc TournamentService,
	productSvc *client.ProductClient,
) *Handler {
	return &Handler{
		userSvc:       userSvc,
		tableSvc:      tableSvc,
		clipSvc:       clipSvc,
		tournamentSvc: tournamentSvc,
		productSvc:    productSvc,
		states:        NewStateManager(),
		userCache:     newUserCache(),
		limiter:       newRateLimiter(),
	}
}

// Handle — asosiy dispatcher
func (h *Handler) Handle(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	tgID, _, _, _ := getUserFromUpdate(update)
	if tgID != 0 && h.limiter.isBlocked(tgID) {
		return
	}

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

	// Kontakt ulashganda telefon raqamini saqlash
	if msg.Contact != nil && msg.Contact.UserID == tgID {
		phone := msg.Contact.PhoneNumber
		if err := h.userSvc.SavePhone(tgID, phone); err == nil {
			user.Phone = phone
			h.userCache.Invalidate(tgID)
		}
		h.showMainMenu(bot, chatID, user)
		return
	}

	// Telefon raqami bo'lmasa — faqat mijozlar bloklanadi (xodimlar callback bilan
	// bir xil: telefonsiz ham ishlay oladi, aks holda ular tiqilib qolardi)
	if user.Phone == "" && !user.IsStaff() {
		h.requirePhone(bot, chatID)
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
		case "help":
			h.cmdHelp(bot, msg, user)
		case "trns":
			if user.IsStaff() {
				h.showAdminTournamentList(bot, chatID, user)
			} else {
				h.showClientTournamentList(bot, chatID, tgID)
			}
		case "tv_bracket":
			h.cmdTVBracket(bot, msg, user)
		case "tv_update":
			h.cmdTVUpdate(bot, msg, user)
		default:
			send(bot, chatID, "❓ Noma'lum buyruq. /help bosing.")
		}
		return
	}

	switch msg.Text {
	case "🎬 Kliplar":
		if user.IsStaff() {
			h.showPendingClips(bot, chatID, user)
		} else {
			h.openClientSubmenu(bot, chatID, tgID, "🎬 Kliplar", clipClientSubmenu())
		}
	case "🏆 Turnirlar":
		if user.IsStaff() {
			h.showAdminTournamentList(bot, chatID, user)
		} else {
			h.openClientSubmenu(bot, chatID, tgID, "🏆 Turnirlar", tournamentClientSubmenu())
		}
	case "🛒 Do'kon":
		h.showProductMenu(bot, chatID, user)
	case "👥 Xodimlar":
		h.showStaffList(bot, chatID, user)
	case "⚙️ Sozlamalar":
		h.showSettings(bot, chatID, user)
	case "👤 Akkaunt":
		h.showMyProfile(bot, chatID, user)
	default:
		// Har doim menyu tugmalarini qayta ko'rsatamiz — foydalanuvchi tiqilib qolmaydi
		sendWithKeyboard(bot, chatID, "🤔 Tushunmadim. Quyidagi menyudan tanlang 👇", mainMenuKeyboard(user))
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

	if user.Phone == "" && !user.IsStaff() {
		answerCallback(bot, cb.ID, "📱 Avval telefon raqamingizni ulashing")
		h.requirePhone(bot, chatID)
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
	case "clip_time_adj":
		h.cbClipTimeAdjust(bot, chatID, msgID, tgID, arg1)
	case "clip_time_ok":
		h.cbClipTimeOK(bot, chatID, msgID, tgID)
	case "clip_noop":
		// silent — faqat display uchun tugmalar
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
		h.cbAdminClipFail(bot, chatID, tgID, user, arg1)
	case "admin_refund":
		h.cbAdminRefund(bot, chatID, tgID, user, arg1)
	case "admin_clips_list":
		h.showPendingClips(bot, chatID, user)
	case "admin_all_clips":
		h.showAllClips(bot, chatID, msgID, user)
	case "my_clip_detail":
		h.cbShowMyClipDetail(bot, chatID, msgID, tgID, arg1)
	case "my_clips_back":
		h.showMyClips(bot, chatID, tgID)
	case "set_role":
		h.cbSetRole(bot, chatID, msgID, user, arg1, arg2)
	case "admin_staff_list":
		h.showStaffList(bot, chatID, user)
	case "clip_detail":
		h.cbShowClipDetail(bot, chatID, msgID, user, arg1)
	case "clip_record":
		h.cbRecordClip(bot, chatID, msgID, user, arg1)
	case "clip_retry":
		h.cbRetryRecording(bot, chatID, msgID, user, arg1)
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

	case "admin_trn_sel_branch":
		h.cbAdminTrnSelBranch(bot, chatID, tgID, arg1)
	case "admin_trn_sel_type":
		h.cbAdminTrnSelType(bot, chatID, tgID, arg1)

	// ── TURNIR: admin ──
	case "admin_trn_list":
		h.showAdminTournamentList(bot, chatID, user)
	case "admin_trn_create":
		h.cbAdminTrnCreate(bot, chatID, tgID, user)
	case "admin_trn_detail":
		h.cbAdminTrnDetail(bot, chatID, msgID, user, arg1)
	case "admin_trn_add_player":
		h.cbAdminTrnAddPlayer(bot, chatID, tgID, user, arg1)
	case "admin_trn_edit":
		h.cbAdminTrnEdit(bot, chatID, msgID, tgID, user, arg1)
	case "admin_trn_edit_field":
		h.cbAdminTrnEditField(bot, chatID, tgID, arg1, arg2)
	case "admin_trn_cancel":
		h.cbAdminTrnCancel(bot, chatID, user, arg1)
	case "admin_trn_cancel_confirm":
		h.cbAdminTrnCancelConfirm(bot, chatID, user, arg1)
	case "admin_trn_delete":
		h.cbAdminTrnDelete(bot, chatID, user, arg1)
	case "admin_trn_delete_confirm":
		h.cbAdminTrnDeleteConfirm(bot, chatID, user, arg1)
	case "admin_trn_regs":
		h.cbAdminTrnRegs(bot, chatID, msgID, user, arg1)
	case "admin_trn_approve":
		h.cbAdminTrnApproveReg(bot, chatID, msgID, user, arg1, arg2)
	case "admin_trn_reject":
		h.cbAdminTrnRejectReg(bot, chatID, msgID, user, arg1, arg2)
	case "admin_trn_bracket":
		h.cbAdminGenBracket(bot, chatID, msgID, user, arg1)
	case "admin_trn_shuffle":
		h.cbAdminTrnShuffle(bot, chatID, msgID, user, arg1)
	case "admin_trn_result":
		h.cbAdminTrnResult(bot, chatID, msgID, user, arg1)
	case "admin_match_start":
		h.cbAdminMatchStart(bot, chatID, msgID, tgID, user, arg1, arg2)
	case "admin_trn_winner":
		h.cbAdminTrnWinner(bot, chatID, msgID, user, arg1, arg2)

	// ── TURNIR: client ──
	case "trn_history":
		deleteMessage(bot, chatID, msgID)
		h.showTournamentHistory(bot, chatID)
	case "trn_list", "trn_menu_list":
		deleteMessage(bot, chatID, msgID)
		h.showClientTournamentList(bot, chatID, tgID)
	case "trn_detail":
		h.cbClientTrnDetail(bot, chatID, msgID, tgID, arg1)
	case "trn_register":
		h.cbClientRegister(bot, chatID, msgID, user, arg1)
	case "trn_my", "trn_menu_my":
		deleteMessage(bot, chatID, msgID)
		h.showMyTournaments(bot, chatID, tgID)
	case "trn_players":
		h.cbClientTrnPlayers(bot, chatID, msgID, arg1)
	case "trn_bracket":
		h.cbClientBracket(bot, chatID, msgID, tgID, arg1)

	// ── KLIP: client submenu ──
	case "clip_menu_request":
		deleteMessage(bot, chatID, msgID)
		h.startClipRequest(bot, chatID, tgID)
	case "clip_menu_my":
		deleteMessage(bot, chatID, msgID)
		h.showMyClips(bot, chatID, tgID)

	// ── DO'KON (admin) ──
	case "prod_add":
		h.cbProductAdd(bot, chatID, tgID, user)
	case "prod_del":
		h.cbProductDelAsk(bot, chatID, msgID, user, arg1)
	case "prod_delok":
		h.cbProductDelOK(bot, chatID, msgID, user, arg1)
	case "prod_menu":
		deleteMessage(bot, chatID, msgID)
		h.showProductMenu(bot, chatID, user)

	case "client_menu_close":
		deleteMessage(bot, chatID, msgID)
		h.states.SetData(tgID, "menu_msg_id", 0)

	case "profile":
		switch arg1 {
		case "edit_name":
			h.cbProfileEditName(bot, chatID, msgID, tgID)
		case "cancel_edit":
			h.states.Clear(tgID)
			user2 := h.authUser(tgID)
			if user2 != nil {
				kb := profileKeyboard()
				editMessage(bot, chatID, msgID, profileText(user2), &kb)
			}
		}

	default:
		log.Printf("unknown callback: %s", data)
	}
}

// openClientSubmenu — eski submenu xabarini o'chirib yangi submenu yuboradi
func (h *Handler) openClientSubmenu(bot *tgbotapi.BotAPI, chatID, tgID int64, title string, kb tgbotapi.InlineKeyboardMarkup) {
	if oldID, ok := h.states.GetInt64(tgID, "menu_msg_id"); ok && oldID != 0 {
		deleteMessage(bot, chatID, int(oldID))
	}
	newID := sendAndGetMsgID(bot, chatID, title, kb)
	h.states.SetData(tgID, "menu_msg_id", int64(newID))
}
