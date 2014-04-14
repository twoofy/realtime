package manager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"engines/github.com.blackjack.syslog"
	"engines/github.com.bmizerany.pat"

	"realtime/account_entry"
	"realtime/account_store"
	"realtime/credential"
	"realtime/logger"
	"realtime/state"
)

type Manager interface {
	Log() *logger.Logger
	Name() string
	Type() ConnectorEnum
	Startup() bool
	Shutdown() bool
	State() *state.State
	Store() *account_store.Store
	Credential() *credential.Credential
}
type ConnectorEnum string

const (
	CONNECTOR ConnectorEnum = "connector"
	ROUTER    ConnectorEnum = "router"
)

type baseManager struct {
	store      *account_store.Store
	state      *state.State
	credential *credential.Credential
	name       string
	Logger     logger.Logger
}
type BaseConnector struct {
	baseManager
}

func (b *BaseConnector) InitBaseConnector(name string, store *account_store.Store, credential *credential.Credential) {
	b.initbaseManager(name, store, credential)
	b.Logger.Logprefix = fmt.Sprintf("manager %s, type %s", name, b.Type())
}

func (b *BaseConnector) Type() ConnectorEnum {
	return CONNECTOR
}

type BaseRouter struct {
	baseManager
	pat *pat.PatternServeMux
}

func (b *BaseRouter) InitBaseRouter(name string, store *account_store.Store, credential *credential.Credential, pat *pat.PatternServeMux) {
	b.initbaseManager(name, store, credential)
	path := "/" + name + "/:id"

	b.pat = pat

	b.pat.Put(path, http.HandlerFunc(b.HttpHandler))

	b.pat.Get(path, http.HandlerFunc(b.HttpHandler))
	b.Logger.Logprefix = fmt.Sprintf("manager %s, type %s ", name, b.Type())

}

func (b *BaseRouter) Type() ConnectorEnum {
	return ROUTER
}

func (b *baseManager) Credential() *credential.Credential {
	return b.credential
}
func (b *baseManager) Store() *account_store.Store {
	return b.store
}
func (b *baseManager) State() *state.State {
	return b.state
}
func (b *baseManager) Name() string {
	return b.name
}
func (b *baseManager) Log() *logger.Logger {
	return &b.Logger
}
func (b *baseManager) initbaseManager(name string, store *account_store.Store, credential *credential.Credential) {
	b.store = store
	b.state = state.NewState()
	b.credential = credential
	b.name = name
}

func RestartMonitor(managers []Manager) {
	reloadTimer := time.Tick(15 * time.Second)
	for {
		select {
		case <-reloadTimer:
			for _, manager := range managers {
				t := manager.Type()
				name := manager.Name()
				if t != CONNECTOR {
					manager.Log().Debugf("Skipping restart of %s %s\n", t, name)
					continue
				}
				store := manager.Store()
				s := manager.State()
				if store.NeedsRestart() && *s.State() == state.UP {
					manager.Log().Infof("Restarting %s %s\n", t, name)
					Stop(manager)
					Start(manager)
					manager.Store().SetRestart(false)
				} else {
					manager.Log().Debugf("No need to restart %s %s\n", t, name)
				}
			}
		}
	}
}

func Start(m Manager) *state.StateEnum {
	s := m.State()
	if *s.State() != state.DOWN {
		return nil
	}
	s.SetState(state.STARTUP)
	// m.Startup() needs to call SetState(state.UP) or will block on Wait()
	m.Startup()
	s.Wait()
	return s.State()
}

func Stop(m Manager) *state.StateEnum {
	s := m.State()
	if *s.State() != state.UP {
		return nil
	}
	s.SetState(state.SHUTDOWN)
	// m.Shutdown() needs to call SetState(state.DOWN) or will block on Wait()
	m.Shutdown()
	s.Wait()
	return s.State()
}

func (b *baseManager) HttpHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s := b.State()
	store := b.Store()
	c := b.Credential()

	if *s.State() != state.UP {
		sendResponse(w, r, RESPONSE_NOT_FOUND, SCAN_UNDEFINED, ERROR_ROUTE_DOWN)
		return
	}

	if r.Method == "PUT" {
		handlePut(w, r, s, store, c)
	} else if r.Method == "GET" {
		handleGet(w, r, s, store, c)
	} else if r.Method == "HEAD" {
		handleHead(w, r, s, store, c)
	} else {
		w.WriteHeader(int(RESPONSE_NOT_ALLOWED))
		w.Write(*makeJson(RESPONSE_NOT_ALLOWED, SCAN_UNDEFINED, ERROR_TRY_ANOTHER_METHOD))
	}
}

func handleGet(w http.ResponseWriter, r *http.Request, s *state.State, store *account_store.Store, c *credential.Credential) {
	account_id := string(r.URL.Query().Get(":id"))
	account, account_present := store.AccountEntry(account_id)
	if account_present {
		scanCode, reasonCode := scanCodeAndReason(s, account)
		sendResponse(w, r, RESPONSE_OK, scanCode, reasonCode)
	} else {
		sendResponse(w, r, RESPONSE_NOT_FOUND, SCAN_UNDEFINED, ERROR_ACCOUNT_NOT_MONITORED)
	}
}

func handleHead(w http.ResponseWriter, r *http.Request, s *state.State, store *account_store.Store, c *credential.Credential) {
	account_id := string(r.URL.Query().Get(":id"))
	_, account_present := store.AccountEntry(account_id)
	if account_present {
		w.WriteHeader(int(RESPONSE_OK))
	} else {
		w.WriteHeader(int(RESPONSE_NOT_FOUND))
	}
}

func handlePut(w http.ResponseWriter, r *http.Request, s *state.State, store *account_store.Store, c *credential.Credential) {
	credential := credential.CredentialFromJson(r.Body)
	if credential == nil {
		sendResponse(w, r, RESPONSE_BAD_REQUEST, SCAN_UNDEFINED, ERROR_JSON_UNPARSABLE)
		return
	}
	if !credential.Valid() {
		sendResponse(w, r, RESPONSE_BAD_REQUEST, SCAN_UNDEFINED, ERROR_JSON_INVALID)
		return
	}
	if *s.State() == state.UP {
		if c.Stale() == true {
			c.Update(credential)
		} else {
			if *c != *credential {
				sendResponse(w, r, RESPONSE_UNAUTHORIZED, SCAN_UNDEFINED, ERROR_CREDENTIAL_INVALID)
				return
			}
		}
	}

	var account *account_entry.Entry
	var account_present bool

	var responseCode responseCodeEnum
	var scanCode scanCodeEnum
	var reasonCode reasonCodeEnum

	account_id := string(r.URL.Query().Get(":id"))
	account, account_present = store.AccountEntry(account_id)
	if account_present {
		responseCode = RESPONSE_OK
	} else {
		store.AddAccountEntry(account_id)
		account, account_present = store.AccountEntry(account_id)
		if account_present {
			responseCode = RESPONSE_CREATED
		} else {
			sendResponse(w, r, RESPONSE_INTERNAL_ERROR, SCAN_UNDEFINED, ERROR_ACCOUNT_CANNOT_STORE)
			return
		}
	}
	scanCode, reasonCode = scanCodeAndReason(s, account)
	if account.SetLastScan() == false {
		w.WriteHeader(int(RESPONSE_INTERNAL_ERROR))
		w.Write(*makeJson(RESPONSE_INTERNAL_ERROR, SCAN_UNDEFINED, ERROR_ACCOUNT_CANNOT_UPDATE_LASTSCAN))
		return
	}
	sendResponse(w, r, responseCode, scanCode, reasonCode)
}

type responseCodeEnum int

const (
	RESPONSE_OK             responseCodeEnum = http.StatusOK
	RESPONSE_CREATED        responseCodeEnum = http.StatusCreated
	RESPONSE_BAD_REQUEST    responseCodeEnum = http.StatusBadRequest
	RESPONSE_NOT_FOUND      responseCodeEnum = http.StatusNotFound
	RESPONSE_NOT_ALLOWED    responseCodeEnum = http.StatusMethodNotAllowed
	RESPONSE_UNAUTHORIZED   responseCodeEnum = http.StatusUnauthorized
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
	ERROR_CREDENTIAL_INVALID             reasonCodeEnum = "unexpected credential"
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

	makeJson(RESPONSE_UNAUTHORIZED, SCAN_UNDEFINED, ERROR_CREDENTIAL_INVALID)

	makeJson(RESPONSE_NOT_FOUND, SCAN_UNDEFINED, ERROR_ACCOUNT_NOT_MONITORED)
	makeJson(RESPONSE_NOT_FOUND, SCAN_UNDEFINED, ERROR_ROUTE_DOWN)

	makeJson(RESPONSE_NOT_ALLOWED, SCAN_UNDEFINED, ERROR_TRY_ANOTHER_METHOD)

	makeJson(RESPONSE_INTERNAL_ERROR, SCAN_UNDEFINED, ERROR_ACCOUNT_CANNOT_STORE)
	makeJson(RESPONSE_INTERNAL_ERROR, SCAN_UNDEFINED, ERROR_ACCOUNT_CANNOT_UPDATE_LASTSCAN)
}

func scanCodeAndReason(s *state.State, account *account_entry.Entry) (scanCodeEnum, reasonCodeEnum) {
	if *s.State() != state.UP {
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

func makeJson(response_code responseCodeEnum, scan_code scanCodeEnum, reason_code reasonCodeEnum) *[]byte {
	j_response := jsonResponse{Code: response_code, Message: string(scan_code), Reason: string(reason_code)}
	json_present, present := jsonResponses[j_response]
	if present {
		syslog.Debugf("json response %d, scan '%s', reason '%s' is cached\n", response_code, scan_code, reason_code)
		return json_present
	}
	json_create, err_create := json.Marshal(j_response)
	if err_create != nil {
		syslog.Alertf("Unable to jsonMarshal(response %d, scan '%s', reason '%s', error '%s'\n", response_code, string(scan_code), string(reason_code))
		return nil
	}
	jsonResponses[j_response] = &json_create
	return jsonResponses[j_response]
}
