package bot

import "sync"

// State nomlari
const (
	StateIdle = ""

	// Klip so'rash oqimi
	StateClipBranch   = "clip:branch"
	StateClipTable    = "clip:table"
	StateClipDate     = "clip:date"
	StateClipTime     = "clip:time"
	StateClipDuration = "clip:duration"
	StateClipPayment  = "clip:payment" // screenshot kutilmoqda

	// Admin: sessiya boshlash
	StateSessionClientName = "session:client_name"

	// Admin: xodim qo'shish
	StateAddStaffID = "staff:add_id"
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
