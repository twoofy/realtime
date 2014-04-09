package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"realtime/account_entry"
	"realtime/account_store"

	"realtime/monitors/twitterstream"
)


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

func main() {
	// handle control-c and kill
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	store := account_store.New()

	twitter_manager := twitterstream.New(store)
	go twitter_manager.Start()

	go http.ListenAndServe(":8080", nil)


	reloadTimer := time.Tick(60 * time.Second)
	for {
		select {
		case <-c:
			log.Println("Got close signal")
			twitter_manager.Stop()
			os.Exit(1)
		case <-reloadTimer:
			restart := false
			accounts, present := store.AccountEntries(account_store.TWITTER_STREAM)
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
			if restart {
				fmt.Println("Restarting twitterstream")
				twitter_manager.Stop()
			} else {
				fmt.Println("No need to restart twitterstream")
			}
		}
	}
	select {}
}
