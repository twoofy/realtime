package manager

import (
	"encoding/json"
	"net/http"

	"engines/github.com.blackjack.syslog"

	"realtime/account_entry"
	"realtime/account_store"
	"realtime/credential"
	"realtime/state"
)


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
	} else {
		syslog.Infof("json response %d, scan '%s', reason '%s' is cached, consider adding it to init() in package manager\n", response_code, scan_code, reason_code)
	}
	json_create, err_create := json.Marshal(j_response)
	if err_create != nil {
		syslog.Alertf("Unable to jsonMarshal(response %d, scan '%s', reason '%s', error '%s'\n", response_code, string(scan_code), string(reason_code))
		return nil
	}
	jsonResponses[j_response] = &json_create
	return jsonResponses[j_response]
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
	if c.Stale() == true {
		c.Update(credential)
	} else {
		if c.Changed(credential) {
			sendResponse(w, r, RESPONSE_UNAUTHORIZED, SCAN_UNDEFINED, ERROR_CREDENTIAL_INVALID)
			return
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
