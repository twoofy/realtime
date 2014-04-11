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

type responseCodeEnum int

const (
	RESPONSE_OK             responseCodeEnum = http.StatusOK
	RESPONSE_CREATED        responseCodeEnum = http.StatusCreated
	RESPONSE_BAD_REQUEST    responseCodeEnum = http.StatusBadRequest
	RESPONSE_NOT_FOUND      responseCodeEnum = http.StatusNotFound
	RESPONSE_NOT_ALLOWED    responseCodeEnum = http.StatusMethodNotAllowed
	RESPONSE_INTERNAL_ERROR responseCodeEnum = http.StatusInternalServerError
)

type scanCodeEnum string

const (
	SCAN_UNDEFINED scanCodeEnum = ""
	SCAN_YES       scanCodeEnum = "yes"
	SCAN_NO        scanCodeEnum = "no"
)

type reasonCodeEnum string

const (
	REASON_DO_SCAN_NOT_MONITORED      reasonCodeEnum = "not monitored"
	REASON_DO_SCAN_MONITORING_OFF     reasonCodeEnum = "monitoring turned off"
	REASON_DO_SCAN_NEW_CONTENT        reasonCodeEnum = "new content has arrived"
	REASON_DO_NOT_SCAN_NO_NEW_CONTENT reasonCodeEnum = "no new content"

	ERROR_JSON_UNPARSABLE                reasonCodeEnum = "cannot parse"
	ERROR_JSON_INVALID                   reasonCodeEnum = "unexpected json"
	ERROR_ACCOUNT_NOT_MONITORED          reasonCodeEnum = "account is not monitored"
	ERROR_ROUTE_DOWN                     reasonCodeEnum = "route down"
	ERROR_TRY_ANOTHER_METHOD             reasonCodeEnum = "try another method"
	ERROR_ACCOUNT_CANNOT_STORE           reasonCodeEnum = "could not store account for monitoring"
	ERROR_ACCOUNT_CANNOT_UPDATE_LASTSCAN reasonCodeEnum = "unable to update last scan date"
)

type jsonResponse struct {
	Code    responseCodeEnum
	Message string `json:",omitempty"`
	Reason  string `json:",omitempty"`
}

var jsonResponses = make(map[jsonResponse]*[]byte)

func init() {
	makeJson(RESPONSE_OK, SCAN_YES, REASON_DO_SCAN_NOT_MONITORED)
	makeJson(RESPONSE_OK, SCAN_YES, REASON_DO_SCAN_MONITORING_OFF)
	makeJson(RESPONSE_OK, SCAN_YES, REASON_DO_SCAN_NEW_CONTENT)
	makeJson(RESPONSE_OK, SCAN_NO, REASON_DO_NOT_SCAN_NO_NEW_CONTENT)

	makeJson(RESPONSE_CREATED, SCAN_YES, REASON_DO_SCAN_NOT_MONITORED)
	makeJson(RESPONSE_CREATED, SCAN_YES, REASON_DO_SCAN_MONITORING_OFF)
	makeJson(RESPONSE_CREATED, SCAN_YES, REASON_DO_SCAN_NEW_CONTENT)
	makeJson(RESPONSE_CREATED, SCAN_NO, REASON_DO_NOT_SCAN_NO_NEW_CONTENT)

	makeJson(RESPONSE_BAD_REQUEST, SCAN_UNDEFINED, ERROR_JSON_UNPARSABLE)
	makeJson(RESPONSE_BAD_REQUEST, SCAN_UNDEFINED, ERROR_JSON_INVALID)

	makeJson(RESPONSE_NOT_FOUND, SCAN_UNDEFINED, ERROR_ACCOUNT_NOT_MONITORED)
	makeJson(RESPONSE_NOT_FOUND, SCAN_UNDEFINED, ERROR_ROUTE_DOWN)

	makeJson(RESPONSE_NOT_ALLOWED, SCAN_UNDEFINED, ERROR_TRY_ANOTHER_METHOD)

	makeJson(RESPONSE_INTERNAL_ERROR, SCAN_UNDEFINED, ERROR_ACCOUNT_CANNOT_STORE)
	makeJson(RESPONSE_INTERNAL_ERROR, SCAN_UNDEFINED, ERROR_ACCOUNT_CANNOT_UPDATE_LASTSCAN)
}

type jsonRequest struct {
	AppId               string `json:"app_id"`
	AppSecret           string `json:"app_secret"`
	ApiOauthToken       string `json:"api_oauth_token"`
	ApiOauthTokenSecret string `json:"api_oauth_token_secret"`
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
	r.Get("/twitterstream/:id", http.HandlerFunc(m.httpHandler))
	http.Handle("/", r)
}

func (m *Manager) httpHandler(w http.ResponseWriter, r *http.Request) {

	if m.router.State() != state.UP {
		updateAndSendResponse(w, r, RESPONSE_NOT_FOUND, SCAN_UNDEFINED, ERROR_ROUTE_DOWN, nil)
		return
	}

	var json_request jsonRequest
	dec := json.NewDecoder(r.Body)

	err := dec.Decode(&json_request)

	if err != nil {
		updateAndSendResponse(w, r, RESPONSE_BAD_REQUEST, SCAN_UNDEFINED, ERROR_JSON_UNPARSABLE, nil)
		return
	}
	if !m.Credentials(&json_request) {
		updateAndSendResponse(w, r, RESPONSE_BAD_REQUEST, SCAN_UNDEFINED, ERROR_JSON_INVALID, nil)
		return
	}
	var account *account_entry.Entry
	var account_present bool

	var responseCode responseCodeEnum
	var scanCode scanCodeEnum
	var reasonCode reasonCodeEnum

	store := m.store
	account_id := string(r.URL.Query().Get(":id"))
	account, account_present = store.AccountEntry(account_store.TWITTER_STREAM, account_id)
	if account_present {
		responseCode = RESPONSE_OK
	} else {
		store.AddAccountEntry(account_store.TWITTER_STREAM, account_id)
		account, account_present = store.AccountEntry(account_store.TWITTER_STREAM, account_id)
		if account_present {
			responseCode = RESPONSE_CREATED
			m.restart = true
		} else {
			updateAndSendResponse(w, r, RESPONSE_INTERNAL_ERROR, SCAN_UNDEFINED, ERROR_ACCOUNT_CANNOT_STORE, nil)
			return
		}
	}
	if m.monitor.State() != state.UP {
		scanCode = SCAN_YES
		reasonCode = REASON_DO_SCAN_MONITORING_OFF
	} else if account.State() == account_entry.UNMONITORED {
		scanCode = SCAN_YES
		reasonCode = REASON_DO_SCAN_NOT_MONITORED
	} else if account.IsUpdated() == true {
		scanCode = SCAN_YES
		reasonCode = REASON_DO_SCAN_NEW_CONTENT
	} else {
		scanCode = SCAN_NO
		reasonCode = REASON_DO_NOT_SCAN_NO_NEW_CONTENT
	}
	updateAndSendResponse(w, r, responseCode, scanCode, reasonCode, nil)
}

func updateAndSendResponse(w http.ResponseWriter, r *http.Request, responseCode responseCodeEnum, scanCode scanCodeEnum, reasonCode reasonCodeEnum, account *account_entry.Entry) {

	w.Header().Set("Content-Type", "application/json")

	if r.Method != "GET" && r.Method != "HEAD" && r.Method != "PUT" {
		w.WriteHeader(int(RESPONSE_NOT_ALLOWED))
		w.Write(*makeJson(RESPONSE_NOT_ALLOWED, SCAN_UNDEFINED, ERROR_TRY_ANOTHER_METHOD))
		return
	}
	json_bytes := makeJson(responseCode, scanCode, reasonCode)
	if account != nil && r.Method == "PUT" {
		if account.SetLastScan() == true {
			w.WriteHeader(int(responseCode))
			w.Write(*json_bytes)
		} else {
			w.WriteHeader(int(RESPONSE_INTERNAL_ERROR))
			w.Write(*makeJson(RESPONSE_INTERNAL_ERROR, SCAN_UNDEFINED, ERROR_ACCOUNT_CANNOT_UPDATE_LASTSCAN))
			return
		}
	} else {
		w.WriteHeader(int(responseCode))
		if r.Method != "HEAD" {
			w.Write(*json_bytes)
		}
	}
}

func (m *Manager) filter() {
	if m.monitor.State() != state.STARTUP {
		return
	}
	store := m.store

	m.stream = twitterstream.New()
	slice, slice_present := store.AccountSlice(account_store.TWITTER_STREAM)

	m.monitor.SetState(state.UP)
	for {
		if m.monitor.State() == state.SHUTDOWN {
			break
		}
		if m.stream.Up() == false {
			if !slice_present {
				log.Println("twitterstream not open yet")
				m.monitor.Sleep(1 * time.Second)
			}
			err := m.stream.Open(m.token, m.token_secret, m.oauth_token, m.oauth_token_secret, slice)
			if err != nil {
				log.Printf("Attempted to open connection but failed: %s - sleeping for 60 seconds\n", err)
				m.monitor.Sleep(60 * time.Second)
			}
			log.Println("Connection opened %b", m.stream.Up())
			for _, account_id := range slice {
				account, account_present := store.AccountEntry(account_store.TWITTER_STREAM, account_id)
				if account_present {
					account.SetState(account_entry.MONITORED)
				}
			}
			continue
		}
		tweet_resp, err := m.stream.UnmarshalNext()
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
			m.stream.Close()
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
	log.Println("Shutting down filter()")
	m.stream.Close()
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
	m.stream.Close()
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

func makeJson(response_code responseCodeEnum, scan_code scanCodeEnum, reason_code reasonCodeEnum) *[]byte {
	j_response := jsonResponse{Code: response_code, Message: string(scan_code), Reason: string(reason_code)}
	json_present, present := jsonResponses[j_response]
	if present {
		log.Printf("json response %d, scan '%s', reason '%s' is cached\n", response_code, scan_code, reason_code)
		return json_present
	}
	json_create, err_create := json.Marshal(j_response)
	if err_create != nil {
		log.Printf("Unable to jsonMarshal(response %d, scan '%s', reason '%s', error '%s'\n", response_code, string(scan_code), string(reason_code))
		return nil
	}
	jsonResponses[j_response] = &json_create
	return jsonResponses[j_response]
}
