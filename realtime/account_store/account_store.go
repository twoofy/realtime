package account_store

import (
	"engines/github.com.blackjack.syslog"
	"sync"

	"realtime/account_entry"
)

type Property string

const (
	TWITTER_STREAM Property = "twitterstream"
	FAKE_STREAM    Property = "fakestream"
)

type Store struct {
	Property          Property
	account_entries   map[string]*account_entry.Entry
	account_slice     []string
	restart_on_change bool
	restart           bool
	count             int64
	rwlock            sync.RWMutex
}

func New(restart_on_change bool) *Store {
	account_store := new(Store)

	account_store.account_entries = make(map[string]*account_entry.Entry)
	account_store.restart_on_change = restart_on_change
	account_store.restart = false

	syslog.Debugf("new store created restart on change: %t", restart_on_change)
	return account_store
}

func (account_store *Store) AddAccountEntry(account_id string) *account_entry.Entry {

	account_store.rwlock.Lock()
	defer account_store.rwlock.Unlock()

	Store := account_store.account_entries

	mc, present := Store[account_id]
	if present {
		return mc
	}
	account_entry := account_entry.New(account_id)
	account_entry.SetLastScan()

	Store[account_id] = &account_entry
	account_store.account_slice = append(account_store.account_slice, account_id)
	account_store.restart = true
	account_store.count += 1
	return &account_entry
}

func (account_store *Store) RemoveAccountEntry(account_id string) *account_entry.Entry {

	account_store.rwlock.Lock()
	defer account_store.rwlock.Unlock()

	Store := account_store.account_entries

	mc, present := Store[account_id]
	if !present {
		return nil
	}

	delete(Store, account_id)

	var new_slice []string
	for _, str := range account_store.account_slice {
		if str == account_id {
			continue
		}
		new_slice = append(new_slice, str)
	}

	account_store.account_slice = new_slice

	account_store.restart = true
	account_store.count -= 1
	return mc
}

func (account_store *Store) Count() int64 {
	account_store.rwlock.RLock()
	defer account_store.rwlock.RUnlock()
	return account_store.count
}

func (account_store *Store) NeedsRestart() bool {
	if account_store.restart_on_change == false {
		return false
	}
	account_store.rwlock.RLock()
	defer account_store.rwlock.RUnlock()
	return account_store.restart
}

func (account_store *Store) SetRestart(state bool) {
	if account_store.restart_on_change == false {
		return
	}
	if account_store.restart == state {
		return
	}
	account_store.rwlock.Lock()
	defer account_store.rwlock.Unlock()
	account_store.restart = state
}

func (account_store *Store) AccountSlice() ([]string, bool) {
	account_store.rwlock.RLock()
	defer account_store.rwlock.RUnlock()

	return account_store.account_slice, true
}

func (account_store *Store) AccountEntries() (map[string]*account_entry.Entry, bool) {
	account_store.rwlock.RLock()
	defer account_store.rwlock.RUnlock()

	return account_store.account_entries, true
}

func (account_store *Store) AccountEntry(account_id string) (*account_entry.Entry, bool) {
	account_store.rwlock.RLock()
	defer account_store.rwlock.RUnlock()

	_, account_present := account_store.account_entries[account_id]
	if !account_present {
		return nil, false
	}
	return account_store.account_entries[account_id], true
}
