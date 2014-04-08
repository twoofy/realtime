package main

import (
  "os"
  "os/signal"
  "syscall"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

  "engines/github.com.bmizerany.pat"
  "realtime/account_store"
  "realtime/account_entry"
)

type AccountState int

const (
	UNMONITORED AccountState = iota
	MONITORED
)

var Credentials = map[string]string{}

var credPath = flag.String("config", "config.json", "Path to configuration file containing the application's credentials.")

var Account_Store = account_store.New()

func init() {
	log.Println("In intialize")
	//test code should be removed. This should be dynamic eventually

	//init_Account_Store_Entry(account_store.TWITTER_STREAM, "2183242184")
	//init_Account_Store_Entry(account_store.TWITTER_STREAM, "14681605")
	//init_Account_Store_Entry(account_store.TWITTER_STREAM, "25365536")
	//init_Account_Store_Entry(account_store.TWITTER_STREAM, "139162440")

	// End code that should be removed.
}

func ScanRequestHandler(w http.ResponseWriter, r *http.Request) {
	//params := mux.Vars(r)
	w.Write([]byte("Yes"))
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
        accounts, present := Account_Store.AccountEntries(account_store.TWITTER_STREAM)
        if present {
          for _, account := range accounts {
            account_id := account.AccountId()
            state := account.State()
            log.Printf("Account %s in state %d\n", account_id, state)
            if state == account_entry.UNMONITORED {
              restart = true
              break
            }
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
