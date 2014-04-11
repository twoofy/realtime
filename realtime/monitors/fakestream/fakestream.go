package fakestream

import (
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"time"

	"engines/github.com.bmizerany.pat"

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
	REASON_DO_SCAN_FIRST_SCAN         reasonCodeEnum = "first scan since monitor started"
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
	name               string
	store              *account_store.Store
	stream             chan string
	monitor            *state.MonitoredState
	router             *state.MonitoredState
	token              string
	token_secret       string
	oauth_token        string
	oauth_token_secret string
	restart            bool
	userIds            []string
}

func New(store *account_store.Store, r *pat.PatternServeMux) *Manager {
	var m Manager
	m.name = string(account_store.FAKE_STREAM)
	m.monitor = state.New(m.name + " monitor")
	m.router = state.New(m.name + " router")
	m.setRoutes(r)
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

func (m *Manager) setRoutes(r *pat.PatternServeMux) {
	path := "/" + m.name + "/:id"
	r.Put(path, http.HandlerFunc(m.httpHandler))
	r.Get(path, http.HandlerFunc(m.httpHandler))
}

func (m *Manager) httpHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	if m.router.State() != state.UP {
		sendResponse(w, r, RESPONSE_NOT_FOUND, SCAN_UNDEFINED, ERROR_ROUTE_DOWN)
		return
	}

	if r.Method != "GET" && r.Method != "HEAD" && r.Method != "PUT" {
		return
	}

	if r.Method == "PUT" {
		m.handlePut(w, r)
	} else if r.Method == "GET" {
		m.handleGet(w, r)
	} else if r.Method == "HEAD" {
		m.handleHead(w, r)
	} else {
		w.WriteHeader(int(RESPONSE_NOT_ALLOWED))
		w.Write(*makeJson(RESPONSE_NOT_ALLOWED, SCAN_UNDEFINED, ERROR_TRY_ANOTHER_METHOD))
	}
}

func (m *Manager) handleGet(w http.ResponseWriter, r *http.Request) {
	account_id := string(r.URL.Query().Get(":id"))
	account, account_present := m.store.AccountEntry(account_store.FAKE_STREAM, account_id)
	if account_present {
		scanCode, reasonCode := m.scanCodeAndReason(account)
		sendResponse(w, r, RESPONSE_OK, scanCode, reasonCode)
	} else {
		sendResponse(w, r, RESPONSE_NOT_FOUND, SCAN_UNDEFINED, ERROR_ACCOUNT_NOT_MONITORED)
	}
}

func (m *Manager) handleHead(w http.ResponseWriter, r *http.Request) {
	account_id := string(r.URL.Query().Get(":id"))
	_, account_present := m.store.AccountEntry(account_store.FAKE_STREAM, account_id)
	if account_present {
		w.WriteHeader(int(RESPONSE_OK))
	} else {
		w.WriteHeader(int(RESPONSE_NOT_FOUND))
	}
}

func (m *Manager) handlePut(w http.ResponseWriter, r *http.Request) {
	var json_request jsonRequest
	dec := json.NewDecoder(r.Body)

	err := dec.Decode(&json_request)

	if err != nil {
		sendResponse(w, r, RESPONSE_BAD_REQUEST, SCAN_UNDEFINED, ERROR_JSON_UNPARSABLE)
		return
	}
	if !m.Credentials(&json_request) {
		sendResponse(w, r, RESPONSE_BAD_REQUEST, SCAN_UNDEFINED, ERROR_JSON_INVALID)
		return
	}

	var account *account_entry.Entry
	var account_present bool

	var responseCode responseCodeEnum
	var scanCode scanCodeEnum
	var reasonCode reasonCodeEnum

	store := m.store
	account_id := string(r.URL.Query().Get(":id"))
	account, account_present = store.AccountEntry(account_store.FAKE_STREAM, account_id)
	if account_present {
		responseCode = RESPONSE_OK
	} else {
		store.AddAccountEntry(account_store.FAKE_STREAM, account_id)
		account, account_present = store.AccountEntry(account_store.FAKE_STREAM, account_id)
		if account_present {
			responseCode = RESPONSE_CREATED
			m.restart = true
		} else {
			sendResponse(w, r, RESPONSE_INTERNAL_ERROR, SCAN_UNDEFINED, ERROR_ACCOUNT_CANNOT_STORE)
			return
		}
	}
	scanCode, reasonCode = m.scanCodeAndReason(account)
	if account.SetLastScan() == false {
		w.WriteHeader(int(RESPONSE_INTERNAL_ERROR))
		w.Write(*makeJson(RESPONSE_INTERNAL_ERROR, SCAN_UNDEFINED, ERROR_ACCOUNT_CANNOT_UPDATE_LASTSCAN))
		return
	}
	sendResponse(w, r, responseCode, scanCode, reasonCode)
}

func (m *Manager) scanCodeAndReason(account *account_entry.Entry) (scanCodeEnum, reasonCodeEnum) {
	if m.monitor.State() != state.UP {
		return SCAN_YES, REASON_DO_SCAN_MONITORING_OFF
	} else if account.State() == account_entry.UNMONITORED {
		return SCAN_YES, REASON_DO_SCAN_NOT_MONITORED
	} else if account.ScannerSeen() == false {
		return SCAN_YES, REASON_DO_SCAN_FIRST_SCAN
	} else if account.IsUpdated() == true {
		return SCAN_YES, REASON_DO_SCAN_NEW_CONTENT
	} else {
		return SCAN_NO, REASON_DO_NOT_SCAN_NO_NEW_CONTENT
	}
}

func sendResponse(w http.ResponseWriter, r *http.Request, responseCode responseCodeEnum, scanCode scanCodeEnum, reasonCode reasonCodeEnum) {
	json_bytes := makeJson(responseCode, scanCode, reasonCode)
	w.WriteHeader(int(responseCode))
	w.Write(*json_bytes)
}

func (m *Manager) Up() bool {
	if m.stream == nil {
		return false
	}
	return true
}

func (m *Manager) Open(token string, token_secret string, oauth_token string, oauth_token_secret string, userIds []string) error {
	if len(userIds) == 0 {
		time.Sleep(1 * time.Second)
		log.Println("Nothing to filter, not opening connection")
		return nil
	}
	m.userIds = userIds
	m.stream = make(chan string)

	return nil
}

type fakeJson struct {
	Id string
}

func (m *Manager) UnmarshalNext() (*fakeJson, error) {
	rand.Seed(time.Now().Unix())
	secs := time.Duration(rand.Intn(9)+1) * time.Second

	reloadTimer := time.Tick(secs)
	log.Printf("Sleeping for %s\n", secs)
	for {
		select {
		case <-reloadTimer:
			j_response := fakeJson{Id: m.userIds[rand.Intn(len(m.userIds))]}
			return &j_response, nil
		}
	}
}

func (m *Manager) Close() {
	m.stream = nil
}

func (m *Manager) filter() {
	if m.monitor.State() != state.STARTUP {
		return
	}
	store := m.store

	slice, slice_present := store.AccountSlice(account_store.FAKE_STREAM)

	m.monitor.SetState(state.UP)
	for {
		if m.monitor.State() == state.SHUTDOWN {
			break
		}
		if m.Up() == false {
			if !slice_present {
				log.Println(m.name + " not open yet")
				m.monitor.Sleep(1 * time.Second)
			}
			err := m.Open(m.token, m.token_secret, m.oauth_token, m.oauth_token_secret, slice)
			if err != nil {
				log.Printf("Attempted to open connection but failed: %s - sleeping for 60 seconds\n", err)
				m.monitor.Sleep(60 * time.Second)
			}
			log.Println("Connection opened %b", m.Up())
			for _, account_id := range slice {
				account, account_present := store.AccountEntry(account_store.FAKE_STREAM, account_id)
				if account_present {
					account.SetState(account_entry.MONITORED)
				}
			}
			continue
		}
		resp, err := m.UnmarshalNext()
		// stream is down to get resp == nil and err == nil
		if resp == nil && err == nil {
			continue
		}
		if err != nil {
			log.Printf("UnmarshalNext error %s\n", err)
			for _, account_id := range slice {
				account, account_present := store.AccountEntry(account_store.FAKE_STREAM, account_id)
				if account_present {
					account.SetState(account_entry.UNMONITORED)
				}
			}
			m.Close()
			continue
		}
		if resp.Id != "" {
			account_id := resp.Id
			log.Printf("Account Store contents %v\n", store)
			log.Printf("Content from %s\n", m.name)
			log.Printf("UserId %s\n", account_id)
			account, present := store.AccountEntry(account_store.FAKE_STREAM, account_id)
			if !present {
				log.Printf("Account id %s is not present\n", account_id)
			} else {
				account.SetLastUpdate()
			}
			continue
		}
		log.Printf("WTF: %v\n", *resp)
	}
	log.Println("Shutting down filter()")
	m.Close()
	m.monitor.SetState(state.DOWN)
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
				m.restart = false
			} else {
				log.Println("no need to restart " + m.name + " monitor")
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
	m.Close()
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
