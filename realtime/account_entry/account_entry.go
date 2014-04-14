package account_entry

import (
	"sync"
	"time"

	"realtime/logger"
)

type AccountState int

const (
	UNMONITORED AccountState = iota
	MONITORED
)

type Entry struct {
	account_id     string
	last_scan_dt   int64
	last_update_dt int64
	scanner_seen   bool
	state          AccountState
	logger         logger.Logger
	rwlock         sync.RWMutex
}

func (h *Entry) AccountId() string {
	return h.account_id
}

func (h *Entry) SetState(state AccountState) {
	h.rwlock.Lock()
	defer h.rwlock.Unlock()

	h.state = state
}

func (h *Entry) State() AccountState {
	h.rwlock.RLock()
	defer h.rwlock.RUnlock()

	return h.state
}

func (h *Entry) ScannerSeen() bool {
	h.rwlock.RLock()
	defer h.rwlock.RUnlock()

	return h.scanner_seen
}

func (h *Entry) IsUpdated() bool {
	h.rwlock.RLock()
	defer h.rwlock.RUnlock()

	if h.last_update_dt >= h.last_scan_dt {
		return true
	}
	return false
}

func (h *Entry) SetLastUpdate() bool {
	h.rwlock.Lock()
	defer h.rwlock.Unlock()

	last_update := &h.last_update_dt
	h.last_update_dt = int64(time.Now().Unix())

	h.logger.Debugf("setting last content date from %d to %d", last_update, h.last_update_dt)
	return true
}

func (h *Entry) SetLastScan() bool {
	h.rwlock.Lock()
	defer h.rwlock.Unlock()

	last_scan := &h.last_scan_dt
	h.last_scan_dt = int64(time.Now().Unix())

	if h.scanner_seen == false && h.state == MONITORED {
		h.scanner_seen = true
	}

	h.logger.Debugf("setting last scan date from %d to %d", last_scan, h.last_scan_dt)
	h.logger.Debugf("setting last content date from %d to %d", 100, 10)
	return true
}

func New(account_id string) Entry {
	var account_entry Entry

	account_entry.account_id = account_id
	account_entry.state = UNMONITORED
	account_entry.scanner_seen = false
	account_entry.logger.Logprefix = "account " + account_id

	return account_entry
}
