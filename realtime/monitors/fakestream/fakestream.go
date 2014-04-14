package fakestream

import (
	"errors"
	"net/http"
	"time"

	"engines/github.com.blackjack.syslog"
	"engines/github.com.bmizerany.pat"

	"engines/fakestream"
	"realtime/account_entry"
	"realtime/account_store"
	"realtime/logger"
	"realtime/state"
)

type Manager struct {
	monitors.Manager
	name               string
	stream             *fakestream.FakeStream
	token              string
	token_secret       string
	oauth_token        string
	oauth_token_secret string
	restart            bool
	logger             logger.Logger
}

func New(store *account_store.Store, r *pat.PatternServeMux) *Manager {
	var m Manager
	m.name = string(account_store.FAKE_STREAM)
	m.Monitor = state.NewState()
	m.Router = state.NewState()
	m.setRoutes(r)
	m.Store = store
	m.Logger.Logprefix = "store " + m.name
	go m.restartMonitor()
	return &m
}

func (m *Manager) setRoutes(r *pat.PatternServeMux) {
	path := "/" + m.name + "/:id"
	r.Put(path, http.HandlerFunc(m.HttpHandler))
	r.Get(path, http.HandlerFunc(m.HttpHandler))
}

func (m *Manager) filter() {
	if m.Monitor.State() != state.STARTUP {
		return
	}
	store := m.Store

	m.stream = fakestream.New()
	slice, slice_present := store.AccountSlice(account_store.FAKE_STREAM)

	m.Monitor.SetState(state.UP)
	for {
		if m.Monitor.State() == state.SHUTDOWN {
			break
		}
		if m.stream.Up() == false {
			if !slice_present {
				m.Logger.Debugf(m.name + " not open yet")
				m.Monitor.Sleep(1 * time.Second)
			}
			err := m.stream.Open(m.token, m.token_secret, m.oauth_token, m.oauth_token_secret, slice)
			if err != nil {
				m.Logger.Noticef("Attempted to open connection but failed: %s - sleeping for 60 seconds\n", err)
				m.Monitor.Sleep(60 * time.Second)
			}
			m.Logger.Infof("Connection opened %t", m.stream.Up())
			for _, account_id := range slice {
				account, account_present := store.AccountEntry(account_store.FAKE_STREAM, account_id)
				if account_present {
					account.SetState(account_entry.MONITORED)
				}
			}
			continue
		}
		resp, err := m.stream.UnmarshalNext()
		// stream is down to get resp == nil and err == nil
		if resp == nil && err == nil {
			continue
		}
		if err != nil {
			m.Logger.Debugf("UnmarshalNext error %s\n", err)
			for _, account_id := range slice {
				account, account_present := store.AccountEntry(account_store.FAKE_STREAM, account_id)
				if account_present {
					account.SetState(account_entry.UNMONITORED)
				}
			}
			m.stream.Close()
			continue
		}
		if resp.Id != "" {
			account_id := resp.Id
			m.Logger.Debugf("Incoming content for %s\n", account_id)
			account, present := store.AccountEntry(account_store.FAKE_STREAM, account_id)
			if !present {
				log.Warnf("Account id %s is not present in store, but it should be\n", account_id)
			} else {
				account.SetLastUpdate()
			}
			continue
		}
		m.Logger.Debugf("Do not know how to handle incoming content %v", *resp)
	}
	m.Logger.Info("Shutting down filter()")
	m.stream.Close()
	m.Monitor.SetState(state.DOWN)
	return
}

func (m *Manager) restartMonitor() {
	reloadTimer := time.Tick(15 * time.Second)
	for {
		select {
		case <-reloadTimer:
			if m.restart && m.monitor.State() == state.UP {
				log.Println("Restarting " + m.name + " monitor")
				m.StopMonitor()
				m.StartMonitor()
			} else {
				log.Println("no need to restart " + m.name + " monitor")
			}
		}
	}
}

func (m *Manager) StartMonitor() (bool, error) {
	if m.Monitor.State() != state.DOWN {
		return false, errors.New("Monitor not down")
	}
	m.Monitor.SetState(state.STARTUP)
	go m.filter()
	m.Monitor.Wait()
	m.restart = false
	return true, nil
}

func (m *Manager) StopMonitor() (bool, error) {
	if m.Monitor.State() != state.UP {
		return false, errors.New("Monitor not up")
	}
	m.Monitor.SetState(state.SHUTDOWN)
	m.stream.Close()
	m.Monitor.Wait()
	return true, nil
}

func (m *Manager) StartRouter() (bool, error) {
	if m.Router.State() != state.DOWN {
		return false, errors.New("HTTP not down")
	}
	m.Router.SetState(state.STARTUP)
	m.Router.SetState(state.UP)
	m.Router.Wait()
	return true, nil
}

func (m *Manager) StopRouter() (bool, error) {
	if m.Router.State() != state.UP {
		return false, errors.New("HTTP not up")
	}
	m.Router.SetState(state.SHUTDOWN)
	m.Router.SetState(state.DOWN)
	m.Router.Wait()
	return true, nil
}
