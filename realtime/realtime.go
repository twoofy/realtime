package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

  "engines/github.com.bmizerany.pat"

	"realtime/account_store"
	"realtime/monitors/twitterstream"
	"realtime/monitors/fakestream"
)

var port *string = flag.String("port", "", "Please enter the port for the client to listen on. Port is required.")

func init() {
	log.Println("In intialize")

}

func ScanRequestHandler(w http.ResponseWriter, r *http.Request) {
	//params := mux.Vars(r)
	w.Write([]byte("Yes"))
}

func main() {
	flag.Parse()
	log.Printf("This is the passed in port %s\n", *port)
	if *port == "" {
		log.Println("Port is required.")
		os.Exit(1)
	}

	// handle control-c and kill
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	store := account_store.New()
  r := pat.New()

	twitter_manager := twitterstream.New(store, r)
	fake_manager := fakestream.New(store, r)

  http.Handle("/", r)
	go http.ListenAndServe(":"+*port, nil)

	twitter_manager.StartMonitor()
	twitter_manager.StartRoute()

	fake_manager.StartMonitor()
	fake_manager.StartRoute()

	for {
		select {
		case <-c:
			log.Println("Got close signal")
			twitter_manager.StopMonitor()
			twitter_manager.StopRoute()
			os.Exit(1)
		}
	}
	select {}
}
