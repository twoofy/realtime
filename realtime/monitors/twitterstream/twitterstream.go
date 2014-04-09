package twitterstream

import (
	"encoding/json"
	"log"
	"net/http"

	"engines/github.com.bmizerany.pat"

	"engines/twitterstream"
	"realtime/account_entry"
	"realtime/account_store"
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

	INVALID_REQUEST

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

	jsonResponses[INVALID_REQUEST] = makeJson(400, "", "invalid json", "cannot continue")

	jsonResponses[INTERNAL_ERROR] = makeJson(500, "", "internal error", "please try again or contact tech support")

}

type Manager struct {
	store *account_store.Store
	stream *twitterstream.TwitterStream
}

func New(store *account_store.Store) (*Manager) {
	var m Manager
	m.setRoutes()
	m.stream = twitterstream.New()
	m.store = store
	return &m
}


func (m *Manager) setRoutes() {
	r := pat.New()

	r.Put("/twitterstream/:id", http.HandlerFunc(m.httpHandler))
	http.Handle("/", r)
}

func (m *Manager) httpHandler(w http.ResponseWriter, r *http.Request) {
	var json_request jsonRequest


	dec := json.NewDecoder(r.Body)

	w.Header().Set("Content-Type", "application/json")
	err := dec.Decode(&json_request)
	if err == nil {
		store := m.store
		account_id := string(r.URL.Query().Get(":id"))
		account, account_present := store.AccountEntry(account_store.TWITTER_STREAM, account_id)
		m.stream.Credentials(json_request.AppId, json_request.AppSecret, json_request.ApiOauthToken, json_request.ApiOauthTokenSecret)
		if account_present {
			if m.stream.State() != twitterstream.UP {
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
			} else if m.stream.State() != twitterstream.UP {
				w.Write(*jsonResponses[CREATED_DO_SCAN_MONITORING_OFF])
			} else if account.State() == account_entry.UNMONITORED {
				w.Write(*jsonResponses[CREATED_DO_SCAN_NOT_MONITORED])
			} else if account.IsUpdated() == true {
				w.Write(*jsonResponses[CREATED_DO_SCAN_NEW_TWEET])
			} else {
				w.Write(*jsonResponses[CREATED_DO_NOT_SCAN])
			}
			account.SetLastScan()
		}
	} else {
		w.Write(*jsonResponses[INVALID_REQUEST])
	}
}

func (m *Manager) Start() {
	store := m.store
	log.Println("handleTwitterFilter called")
	for {
		if m.stream.State() != twitterstream.UP {
			slice, slice_present := store.AccountSlice(account_store.TWITTER_STREAM)
			if slice_present {
				m.stream.Filter(slice)
				for _, user_id := range m.stream.UserIds {
					account, account_present := store.AccountEntry(account_store.TWITTER_STREAM, user_id)
					if account_present {
						account.SetState(account_entry.MONITORED)
					}
				}
			}
		}
		tweet_resp, err := m.stream.UnmarshalNext()
		if err != nil {
			log.Printf("UnmarshalNext error %s\n", err)
			for _, user_id := range m.stream.UserIds {
				account, account_present := store.AccountEntry(account_store.TWITTER_STREAM, user_id)
				if account_present {
					account.SetState(account_entry.UNMONITORED)
				}
			}
			m.stream.Close()
		} else if tweet_resp.ScanUserIdStr != nil {
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
		} else {
			log.Printf("WTF: %v\n", *tweet_resp)
			// WTF!!!!
		}
	}
}

func (m *Manager) Stop() {
}

func makeJson(code int, message string, err_message string, reason string) *[]byte {
	json, err := json.Marshal(jsonResponse{Code: code, Message: message, Error: err_message, Reason: reason})
	if err != nil {
		log.Fatalf("Unable to jsonMarshal(code %d, message '%s', err '%s', reason '%s'\n", code, message, err_message, reason)
	}
	return &json
}


