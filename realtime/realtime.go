package main

import _ "net/http/pprof"
import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"engines/github.com.blackjack.syslog"
	"engines/github.com.bmizerany.pat"

	"realtime/account_store"
	"realtime/credential"
	"realtime/manager"
	"realtime/monitors/twitterstream"
	"realtime/monitors/fakestream"
)

var port *string = flag.String("port", "", "Please enter the port for the client to listen on. Port is required.")

func main() {
	flag.Parse()
	if *port == "" {
		log.Println("Port is required.")
		os.Exit(1)
	}
	syslog.Openlog("realtime", syslog.LOG_PID, syslog.LOG_DEBUG)
	syslog.Noticef("Initialized on port %s", *port)
	syslog.Debugf("Initialized on port %s", *port)

	// handle control-c and kill
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	r := pat.New()
	//management := manager.NewHttpManagement()
	//management.SetRoutes(r)

	twitter_store := account_store.New(true)
	twitter_credential := credential.NewCredential()
	twitter_connector := twitterstream.NewConnector(twitter_store, twitter_credential)
	twitter_router := twitterstream.NewRouter(twitter_store, twitter_credential, r)

	fake_store := account_store.New(true)
  fake_credential := credential.NewCredential()
	fake_manager := fakestream.NewConnector(fake_store, fake_credential)
  fake_router := fakestream.NewRouter(fake_store, fake_credential, r)

	http.Handle("/", r)
	go http.ListenAndServe(":"+*port, nil)
  go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
  }()

	//monitoredArr := []monitors.Managed{twitter_manager, fake_manager}
	monitoredArr := []manager.Manager{twitter_connector, twitter_router, fake_manager, fake_router}
	go manager.RestartMonitor(monitoredArr)

	for _, m := range monitoredArr {
		manager.Start(m)
	}

	for {
		select {
		case <-c:
			syslog.Notice("Exiting")
			for _, m := range monitoredArr {
				manager.Stop(m)
			}
			os.Exit(1)
		}
	}
}
