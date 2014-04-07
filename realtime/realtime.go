package main

import (
  "os"
  "os/signal"
  "syscall"
	//"encoding/json"
	"flag"
	"fmt"
  //"dev.socialiqnetworks.com/twitterstream"
	//"github.com/darkhelmet/twitterstream"
	"log"
	"net/http"
	"sync"
	"time"

  "twitter_real_time/deps/github.com/bmizerany/pat"
)

type AccountState int
type Property int

const (
	UNMONITORED AccountState = iota
	MONITORED
)

const (
	TWITTER_STREAM Property = iota
)

type Account_Entry struct {
	account_id string
	last_scan_dt int64
	last_update_dt int64
	state	AccountState
	rwlock sync.RWMutex
}

func (h *Account_Entry) AccountId() (string) {
	return h.account_id
}

func (h *Account_Entry) setState(state AccountState) {
	h.rwlock.Lock()
	defer h.rwlock.Unlock()

	h.state = state
}

func (h *Account_Entry) State() (AccountState) {
	h.rwlock.RLock()
	defer h.rwlock.RUnlock()

	return h.state
}

func (h *Account_Entry) IsUpdated() (bool) {
	h.rwlock.RLock()
	defer h.rwlock.RUnlock()

  if h.last_update_dt >= h.last_scan_dt {
    return true
  }
  return false
}

func (h *Account_Entry) setLastUpdate() {
	h.rwlock.Lock()
	defer h.rwlock.Unlock()

	last_update := h.last_update_dt
	fmt.Printf("This is the last update %+v\n", last_update)
	h.last_update_dt = int64(time.Now().Unix())

	log.Printf("Account Store contents %v", h)
}

func (h *Account_Entry) setLastScan() {
	h.rwlock.Lock()
	defer h.rwlock.Unlock()

	last_scan := h.last_scan_dt
	fmt.Printf("This is the last scan %+v\n", last_scan)
	h.last_scan_dt = int64(time.Now().Unix())

	log.Println("Account Store contents %v", h)
}

var Credentials = map[string]string{}

var credPath = flag.String("config", "config.json", "Path to configuration file containing the application's credentials.")

var Account_Store = make(map[Property]map[string]*Account_Entry)
var Account_StoreSlice = make(map[Property][]string)

var streams = make(map[Property]chan string)

func init() {
	log.Println("In intialize")
	//test code should be removed. This should be dynamic eventually

	//init_Account_Store_Entry(TWITTER_STREAM, "2183242184")
	//init_Account_Store_Entry(TWITTER_STREAM, "14681605")
	//init_Account_Store_Entry(TWITTER_STREAM, "25365536")
	//init_Account_Store_Entry(TWITTER_STREAM, "139162440")

	// End code that should be removed.
}

func ScanRequestHandler(w http.ResponseWriter, r *http.Request) {
	//params := mux.Vars(r)
	w.Write([]byte("Yes"))
}

func init_Account_Store_Entry(property Property, account_id string) *Account_Entry {
	_, present := Account_Store[property]
	if ! present {
		Account_Store[property] = make(map[string]*Account_Entry)
	}
	mc, present := Account_Store[property][account_id]
	if present {
		return mc
	}
	var account_entry Account_Entry
	account_entry.account_id = account_id
	account_entry.state = UNMONITORED
	Account_Store[property][account_id] = &account_entry

	account_entry.setLastScan()

	Account_StoreSlice[property] = append(Account_StoreSlice[property], account_id)
	return &account_entry
}

func scannerListen() {
  m := pat.New()
  m.Put("/twitterstream/:id", http.HandlerFunc(twitterHttpHandler))
  http.Handle("/", m)
  http.ListenAndServe(":8080", nil)
}

func main() {
  // handle control-c and kill
  c := make(chan os.Signal, 1) 
  signal.Notify(c, os.Interrupt, syscall.SIGTERM)

  go scannerListen()

	go handleTwitterFilter()

	reloadTimer := time.Tick(60 * time.Second)
	for {
		select {
      case <-c:
log.Println("Got close signal")
        twitterStream.Close()
        os.Exit(1)
			case <-reloadTimer:
        restart := false
        for _, account := range Account_Store[TWITTER_STREAM] {
          log.Printf("Account %s in state %d\n", account.account_id, account.state)
          if account.state == UNMONITORED {
            restart = true
            break
          }
        }
        if(restart) {
fmt.Println("Restarting twitterstream")
          twitterStream.Close()
        } else {
fmt.Println("No need to restart twitterstream")
        }
		}
	}
	select { }
}
