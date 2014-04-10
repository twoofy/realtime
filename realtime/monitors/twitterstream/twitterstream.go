package twitterstream

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"engines/github.com.bmizerany.pat"

	"engines/twitterstream"
	"realtime/account_entry"
	"realtime/account_store"
	"realtime/state"
)

type jsonEnum int

const (
	CREATED_DO_SCAN_NOT_MONITORED jsonEnum = iota
	CREATED_DO_SCAN_MONITORING_OFF
	CREATED_DO_SCAN_NEW_TWEET
	CREATED_DO_NOT_SCAN

	EXISTS_DO_SCAN_NOT_MONITORED
	EXISTS_DO_SCAN_MONITORING_OFF
	EXISTS_DO_SCAN_NEW_TWEET
	EXISTS_DO_NOT_SCAN

	NOT_FOUND

	INVALID_REQUEST_NOT_PARSABLE
	INVALID_REQUEST_INVALID_JSON

	INTERNAL_ERROR
)

type jsonResponse struct {
	Code    int
	Message string `json:",omitempty"`
	Error   string `json:",omitempty"`
	Reason  string `json:",omitempty"`
}

var jsonResponses = make(map[jsonEnum]*[]byte)

type jsonRequest struct {
	AppId               string `json:"app_id"`
	AppSecret           string `json:"app_secret"`
	ApiOauthToken       string `json:"api_oauth_token"`
	ApiOauthTokenSecret string `json:"api_oauth_token_secret"`
}

func init() {
	jsonResponses[CREATED_DO_SCAN_NOT_MONITORED] = makeJson(201, "yes", "", "not monitored")
	jsonResponses[CREATED_DO_SCAN_MONITORING_OFF] = makeJson(201, "yes", "", "monitoring turned off")
	jsonResponses[CREATED_DO_SCAN_NEW_TWEET] = makeJson(201, "yes", "", "new tweet has arrived")
	jsonResponses[CREATED_DO_NOT_SCAN] = makeJson(201, "no", "", "no new tweets")

	jsonResponses[EXISTS_DO_SCAN_NOT_MONITORED] = makeJson(200, "yes", "", "not monitored")
	jsonResponses[EXISTS_DO_SCAN_MONITORING_OFF] = makeJson(200, "yes", "", "monitoring turned off")
	jsonResponses[EXISTS_DO_SCAN_NEW_TWEET] = makeJson(200, "yes", "", "new tweet has arrived")
	jsonResponses[EXISTS_DO_NOT_SCAN] = makeJson(200, "no", "", "no new tweets")

	jsonResponses[INVALID_REQUEST_NOT_PARSABLE] = makeJson(400, "", "invalid json", "cannot parse")
	jsonResponses[INVALID_REQUEST_INVALID_JSON] = makeJson(400, "", "invalid json", "unexpected json")

	jsonResponses[NOT_FOUND] = makeJson(404, "", "not found", "route down")

	jsonResponses[INTERNAL_ERROR] = makeJson(500, "", "internal error", "please try again or contact tech support")
}

type Manager struct {
	store              *account_store.Store
	stream             *twitterstream.TwitterStream
	monitor            *state.MonitoredState
	router             *state.MonitoredState
	token              string
	token_secret       string
	oauth_token        string
	oauth_token_secret string
	restart            bool
}

func New(store *account_store.Store) *Manager {
	var m Manager
	m.monitor = state.New("twitterstream monitor")
	m.router = state.New("twitterstream router")
	m.setRoutes()
	m.store = store
	go m.restartMonitor()
	return &m
}

func (m *Manager) Credentials(j *jsonRequest) bool {
	if j.AppId == "" || j.AppSecret == "" || j.ApiOauthToken == "" || j.ApiOauthTokenSecret == "" {
		return false
	}
	if m.token != j.AppId || m.token_secret != j.AppSecret || m.oauth_token != j.ApiOauthToken || m.oauth_token_secret != j.ApiOauthTokenSecret {

		m.token = j.AppId
		m.token_secret = j.AppSecret
		m.oauth_token = j.ApiOauthToken
		m.oauth_token_secret = j.ApiOauthTokenSecret

		log.Println("Credentials have changed")
		m.restart = true
	}
	return true
}

func (m *Manager) setRoutes() {
	r := pat.New()

	r.Put("/twitterstream/:id", http.HandlerFunc(m.httpHandler))
	http.Handle("/", r)
}

func (m *Manager) httpHandler(w http.ResponseWriter, r *http.Request) {
	var json_request jsonRequest

	if m.router.State() != state.UP {
		w.Write(*jsonResponses[NOT_FOUND])
		return
	}

	dec := json.NewDecoder(r.Body)

	w.Header().Set("Content-Type", "application/json")
	err := dec.Decode(&json_request)
	if err == nil {
		store := m.store
		account_id := string(r.URL.Query().Get(":id"))
		account, account_present := store.AccountEntry(account_store.TWITTER_STREAM, account_id)
		if !m.Credentials(&json_request) {
			w.Write(*jsonResponses[INVALID_REQUEST_INVALID_JSON])
			return
		}
		if account_present {
			if m.monitor.State() != state.UP {
				w.Write(*jsonResponses[EXISTS_DO_SCAN_MONITORING_OFF])
			} else if account.State() == account_entry.UNMONITORED {
				w.Write(*jsonResponses[EXISTS_DO_SCAN_NOT_MONITORED])
			} else if account.IsUpdated() == true {
				w.Write(*jsonResponses[EXISTS_DO_SCAN_NEW_TWEET])
			} else {
				w.Write(*jsonResponses[EXISTS_DO_NOT_SCAN])
			}
			account.SetLastScan()
		} else {
			store.AddAccountEntry(account_store.TWITTER_STREAM, account_id)
			account, account_present := store.AccountEntry(account_store.TWITTER_STREAM, account_id)
			if !account_present {
				w.Write(*jsonResponses[INTERNAL_ERROR])
			} else if m.monitor.State() != state.UP {
				w.Write(*jsonResponses[CREATED_DO_SCAN_MONITORING_OFF])
			} else if account.State() == account_entry.UNMONITORED {
				w.Write(*jsonResponses[CREATED_DO_SCAN_NOT_MONITORED])
			} else if account.IsUpdated() == true {
				w.Write(*jsonResponses[CREATED_DO_SCAN_NEW_TWEET])
			} else {
				w.Write(*jsonResponses[CREATED_DO_NOT_SCAN])
			}
			account.SetLastScan()
			m.restart = true
		}
	} else {
		w.Write(*jsonResponses[INVALID_REQUEST_NOT_PARSABLE])
	}
}

func (m *Manager) filter() {
	if m.monitor.State() != state.STARTUP {
		return
	}
	store := m.store

	stream := twitterstream.New()
	slice, slice_present := store.AccountSlice(account_store.TWITTER_STREAM)

	m.monitor.SetState(state.UP)
	for {
		if m.monitor.State() == state.SHUTDOWN {
			break
		}
		if stream.Up() == false {
			if !slice_present {
				log.Println("twitterstream not open yet")
				m.monitor.Sleep(1 * time.Second)
				continue
			}
			err := stream.Open(m.token, m.token_secret, m.oauth_token, m.oauth_token_secret, slice)
			if err != nil {
				log.Printf("Attempted to open connection but failed: %s - sleeping for 60 seconds\n", err)
				m.monitor.Sleep(60 * time.Second)
				continue
			}
			for _, account_id := range slice {
				account, account_present := store.AccountEntry(account_store.TWITTER_STREAM, account_id)
				if account_present {
					account.SetState(account_entry.MONITORED)
				}
			}
		}
		tweet_resp, err := stream.UnmarshalNext()
		// stream is down to get tweet_resp == nil and err == nil
		if tweet_resp == nil && err == nil {
			continue
		}
		if err != nil {
			log.Printf("UnmarshalNext error %s\n", err)
			for _, account_id := range slice {
				account, account_present := store.AccountEntry(account_store.TWITTER_STREAM, account_id)
				if account_present {
					account.SetState(account_entry.UNMONITORED)
				}
			}
			stream.Close()
			continue
		}
		if tweet_resp.ScanUserIdStr != nil {
			account_id := *tweet_resp.ScanUserIdStr
			log.Printf("Account Store contents %v\n", store)
			log.Println("Tweet from twitterstream")
			log.Printf("UserId %s\n", account_id)
			account, present := store.AccountEntry(account_store.TWITTER_STREAM, account_id)
			if !present {
				retweet_account_id := *tweet_resp.RetweetUserIdStr
				_, retweet_present := store.AccountEntry(account_store.TWITTER_STREAM, retweet_account_id)
				if retweet_present {
					log.Printf("Skipping %s because it is a retweet of a monitored account %s\n", account_id, retweet_account_id)
					continue
				} else {
					log.Printf("Initializing non-existant store for %s because it was sent from twitterstream\n", account_id)
					account = store.AddAccountEntry(account_store.TWITTER_STREAM, account_id)
				}
			}
			account.SetLastUpdate()
			continue
		}
		log.Printf("WTF: %v\n", *tweet_resp)
	}
	stream.Close()
	m.monitor.SetState(state.DOWN)
	return
}

func (m *Manager) restartMonitor() {
	reloadTimer := time.Tick(15 * time.Second)
	for {
		select {
		case <-reloadTimer:
			if m.restart && m.monitor.State() == state.UP {
				log.Println("Restarting twitterstream monitor")
				m.StopMonitor()
				m.StartMonitor()
				m.restart = false
			} else {
				log.Println("no need to restart twitterstream monitor")
			}
		}
	}
}

func (m *Manager) StartMonitor() (bool, error) {
	if m.monitor.State() != state.DOWN {
		return false, errors.New("Monitor not down")
	}
	m.monitor.SetState(state.STARTUP)
	go m.filter()
	m.monitor.Wait()
	return true, nil
}

func (m *Manager) StopMonitor() (bool, error) {
	if m.monitor.State() != state.UP {
		return false, errors.New("Monitor not up")
	}
	m.monitor.SetState(state.SHUTDOWN)
	m.monitor.Wait()
	return true, nil
}

func (m *Manager) StartRoute() (bool, error) {
	if m.router.State() != state.DOWN {
		return false, errors.New("HTTP not down")
	}
	m.router.SetState(state.STARTUP)
	m.router.SetState(state.UP)
	m.router.Wait()
	return true, nil
}

func (m *Manager) StopRoute() (bool, error) {
	if m.router.State() != state.UP {
		return false, errors.New("HTTP not up")
	}
	m.router.SetState(state.SHUTDOWN)
	m.router.SetState(state.DOWN)
	m.router.Wait()
	return true, nil
}

func makeJson(code int, message string, err_message string, reason string) *[]byte {
	json, err := json.Marshal(jsonResponse{Code: code, Message: message, Error: err_message, Reason: reason})
	if err != nil {
		log.Fatalf("Unable to jsonMarshal(code %d, message '%s', err '%s', reason '%s'\n", code, message, err_message, reason)
	}
	return &json
}
