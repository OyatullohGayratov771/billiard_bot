package bot

import "sync"

// State nomlari
const (
	StateIdle = ""

	// Klip so'rash oqimi (vaqt tanlash faqat tugmalar orqali)
	StateClipBranch  = "clip:branch"
	StateClipTable   = "clip:table"
	StateClipPayment = "clip:payment" // screenshot kutilmoqda

	// Admin: xodim qo'shish
	StateAddStaffID = "staff:add_id"

	// Admin: NVR sozlamalari (branch)
	StateNVRIP   = "nvr:ip"
	StateNVRPort = "nvr:port"
	StateNVRUser = "nvr:user"
	StateNVRPass = "nvr:pass"

	// Admin: stol RTSP URL
	StateTableRTSP = "table:rtsp"

	// Admin: ruchnoy video yuborish
	StateAdminUploadClip = "admin:upload_clip"

	// Admin: rad etish / qaytarish izoh
	StateAdminRejectNote = "admin:reject_note"
	StateAdminRefundNote = "admin:refund_note"

	// Turnir yaratish oqimi (admin)
	StateTrnName       = "trn:name"
	StateTrnDateTime   = "trn:datetime"
	StateTrnPrice      = "trn:price"
	StateTrnMaxPlayers = "trn:max_players"

	// Turnirga qo'lda o'yinchi qo'shish (admin)
	StateTrnAddPlayer = "trn:add_player"

	// Turnir yaratishda maxfiy kod o'rnatish (admin)
	StateTrnSetCode = "trn:set_code"

	// Turnirga kirish uchun maxfiy kod kiritish (client)
	StateTrnJoinCode = "trn:join_code"

	// Turnir tahrirlash oqimi (admin)
	StateTrnEditName = "trn:edit_name"
	StateTrnEditDate = "trn:edit_date"
	StateTrnEditMax  = "trn:edit_max"

	// O'yin boshlash — stol raqami kiritish (admin)
	StateTrnMatchTableNum = "trn:match_table"

	// Profil tahrirlash
	StateEditName = "profile:edit_name"
)

// UserState — bitta foydalanuvchining joriy holati
type UserState struct {
	State string
	Data  map[string]interface{}
}

// StateManager — thread-safe holat boshqaruvchisi
type StateManager struct {
	mu     sync.RWMutex
	states map[int64]*UserState
}

func NewStateManager() *StateManager {
	return &StateManager{
		states: make(map[int64]*UserState),
	}
}

func (sm *StateManager) Get(tgID int64) *UserState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	s, ok := sm.states[tgID]
	if !ok {
		return &UserState{State: StateIdle, Data: make(map[string]interface{})}
	}
	return s
}

func (sm *StateManager) Set(tgID int64, state string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	existing, ok := sm.states[tgID]
	if !ok {
		sm.states[tgID] = &UserState{State: state, Data: make(map[string]interface{})}
		return
	}
	existing.State = state
}

func (sm *StateManager) SetData(tgID int64, key string, val interface{}) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	s, ok := sm.states[tgID]
	if !ok {
		s = &UserState{State: StateIdle, Data: make(map[string]interface{})}
		sm.states[tgID] = s
	}
	s.Data[key] = val
}

func (sm *StateManager) GetData(tgID int64, key string) (interface{}, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	s, ok := sm.states[tgID]
	if !ok {
		return nil, false
	}
	v, ok := s.Data[key]
	return v, ok
}

func (sm *StateManager) Clear(tgID int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.states, tgID)
}

// GetInt64 — Data dan int64 qiymat oladi
func (sm *StateManager) GetInt64(tgID int64, key string) (int64, bool) {
	v, ok := sm.GetData(tgID, key)
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case int64:
		return val, true
	case int:
		return int64(val), true
	}
	return 0, false
}

// GetString — Data dan string qiymat oladi
func (sm *StateManager) GetString(tgID int64, key string) (string, bool) {
	v, ok := sm.GetData(tgID, key)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
