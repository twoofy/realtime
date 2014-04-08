package account_store

import (
	"log"
  "sync"

  "realtime/account_entry"
)

type Property int

const (
  TWITTER_STREAM Property = iota
)

type Store struct {
  account_entries map[Property]map[string]*account_entry.Entry
  account_slice map[Property][]string
  rwlock sync.RWMutex
}

var StoreSlice = make(map[Property][]string)


func New() (*Store) {
  var account_store Store

  account_store.account_entries = make(map[Property]map[string]*account_entry.Entry)
  account_store.account_slice = make(map[Property][]string)

  log.Println("New Store created")
  return &account_store
}

func (account_store *Store) AddAccountEntry(property Property, account_id string) (*account_entry.Entry) {

  account_store.rwlock.Lock()
  defer account_store.rwlock.Unlock()

  Store := account_store.account_entries
  StoreSlice := account_store.account_slice

  _, present := Store[property]
  if ! present {
    Store[property] = make(map[string]*account_entry.Entry)
  }
  mc, present := Store[property][account_id]
  if present {
    return mc
  }
  account_entry := account_entry.New(account_id)
  account_entry.SetLastScan()

  Store[property][account_id] = &account_entry
  StoreSlice[property] = append(StoreSlice[property], account_id)
  return &account_entry
}

func (account_store *Store) AccountSlice(property Property) ([]string, bool) {
  _, present := account_store.account_entries[property]
  if ! present {
    return nil, false
  }
  return account_store.account_slice[property], true
}

func (account_store *Store) AccountEntries(property Property) (map[string]*account_entry.Entry, bool) {
  _, present := account_store.account_entries[property]
  if ! present {
    return nil, false
  }
  return account_store.account_entries[property], true
}

func (account_store *Store) AccountEntry(property Property, account_id string) (*account_entry.Entry, bool) {
  _, property_present := account_store.account_entries[property]
  if ! property_present {
    return nil, false
  }

  _, account_present := account_store.account_entries[property][account_id]
  if ! account_present {
    return nil, false
  }
  return account_store.account_entries[property][account_id], true
}
