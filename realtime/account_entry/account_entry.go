package account_entry

import (
	"fmt"
	"log"
	"sync"
	"time"
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
	state          AccountState
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

func (h *Entry) IsUpdated() bool {
	h.rwlock.RLock()
	defer h.rwlock.RUnlock()

	if h.last_update_dt >= h.last_scan_dt {
		return true
	}
	return false
}

func (h *Entry) SetLastUpdate() {
	h.rwlock.Lock()
	defer h.rwlock.Unlock()

	last_update := h.last_update_dt
	fmt.Printf("This is the last update %+v\n", last_update)
	h.last_update_dt = int64(time.Now().Unix())

	log.Printf("Account Store contents %v", h)
}

func (h *Entry) SetLastScan() {
	h.rwlock.Lock()
	defer h.rwlock.Unlock()

	last_scan := h.last_scan_dt
	fmt.Printf("This is the last scan %+v\n", last_scan)
	h.last_scan_dt = int64(time.Now().Unix())

	log.Println("Account Store contents %v", h)
}

func New(account_id string) Entry {
	var account_entry Entry

	account_entry.account_id = account_id
	account_entry.state = UNMONITORED

	return account_entry
}
