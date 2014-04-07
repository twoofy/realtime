package main

import (
  "net/http"
  "encoding/json"
  "log"
  "twitter_real_time/deps/nexgate/twitterstream"
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
)

type jsonResponse struct {
  Code int
  Message string `json:",omitempty"`
  Error string `json:",omitempty"`
  Reason string `json:",omitempty"`
}
var jsonResponses = make(map[jsonEnum]*[]byte)

type jsonRequest struct {
  ApiToken string `json:"api_token"`
  ApiTokenSecret string `json:"api_token_secret"`
  ApiOauthToken string `json:"api_oauth_token"`
  ApiOauthTokenSecret string `json:"api_oauth_token_secret"`
  UserToken string `json:"user_token"`
  UserTokenSecret string `json:"user_token_secret"`
  UserOauthToken string `json:"user_oauth_token"`
  UserOauthTokenSecret string `json:"user_oauth_token_secret"`
}

var twitterStream *twitterstream.TwitterStream

func init() {
  twitterStream = twitterstream.New()

  jsonResponses[CREATED_DO_SCAN_NOT_MONITORED] = makeJson(201, "yes", "", "not monitored")
  jsonResponses[CREATED_DO_SCAN_MONITORING_OFF] = makeJson(201, "yes", "", "monitoring turned off")
  jsonResponses[CREATED_DO_SCAN_NEW_TWEET] = makeJson(201, "yes", "", "new tweet has arrived")
  jsonResponses[CREATED_DO_NOT_SCAN] = makeJson(201, "no", "", "no new tweets")

  jsonResponses[EXISTS_DO_SCAN_NOT_MONITORED] = makeJson(200, "yes", "", "not monitored")
  jsonResponses[EXISTS_DO_SCAN_MONITORING_OFF] = makeJson(200, "yes", "", "monitoring turned off")
  jsonResponses[EXISTS_DO_SCAN_NEW_TWEET] = makeJson(200, "yes", "", "new tweet has arrived")
  jsonResponses[EXISTS_DO_NOT_SCAN] = makeJson(200, "no", "", "no new tweets")

  jsonResponses[INVALID_REQUEST] = makeJson(400, "", "invalid json", "cannot continue")
}

func makeJson(code int, message string, err_message string, reason string) (*[]byte) {
  json, err := json.Marshal(jsonResponse{Code : code, Message: message, Error: err_message, Reason: reason})
  if err != nil {
    log.Fatalf("Unable to jsonMarshal(code %d, message '%s', err '%s', reason '%s'\n", code, message, err_message, reason)
  }
  return &json
}

func twitterHttpHandler(w http.ResponseWriter, r *http.Request) {
  var json_request jsonRequest
  dec := json.NewDecoder(r.Body)

  w.Header().Set("Content-Type", "application/json")
  err := dec.Decode(&json_request)
  if err == nil {
    account_id := string(r.URL.Query().Get(":id"))
    account, present := Account_Store[TWITTER_STREAM][account_id]
    twitterStream.Credentials(json_request.ApiToken, json_request.ApiTokenSecret, json_request.ApiOauthToken, json_request.ApiOauthTokenSecret)
    if present {
      if twitterStream.State() != twitterstream.UP {
        w.Write(*jsonResponses[EXISTS_DO_SCAN_MONITORING_OFF])
      } else if account.State() == UNMONITORED {
        w.Write(*jsonResponses[EXISTS_DO_SCAN_NOT_MONITORED])
      } else if account.IsUpdated() == true {
        w.Write(*jsonResponses[EXISTS_DO_SCAN_NEW_TWEET])
      } else {
        w.Write(*jsonResponses[EXISTS_DO_NOT_SCAN])
      }
    } else {
			init_Account_Store_Entry(TWITTER_STREAM, account_id)
      account = Account_Store[TWITTER_STREAM][account_id]
      if twitterStream.State() != twitterstream.UP {
        w.Write(*jsonResponses[CREATED_DO_SCAN_MONITORING_OFF])
      } else if account.State() == UNMONITORED {
        w.Write(*jsonResponses[CREATED_DO_SCAN_NOT_MONITORED])
      } else if account.IsUpdated() == true {
        w.Write(*jsonResponses[CREATED_DO_SCAN_NEW_TWEET])
      } else {
        w.Write(*jsonResponses[CREATED_DO_NOT_SCAN])
      }
    }
    Account_Store[TWITTER_STREAM][account_id].setLastScan()
  } else {
    w.Write(*jsonResponses[INVALID_REQUEST])
  }
}

func handleTwitterFilter() {
log.Println("handleTwitterFilter called")
	for {
    if twitterStream.State() != twitterstream.UP {
	    twitterStream.Filter(Account_StoreSlice[TWITTER_STREAM])
      for _, user_id := range twitterStream.UserIds {
        Account_Store[TWITTER_STREAM][user_id].setState(MONITORED)
      }
    }
		tweet_resp, err := twitterStream.UnmarshalNext()
		if err != nil {
			log.Printf("UnmarshalNext error %s\n", err)
      for _, user_id := range twitterStream.UserIds {
        Account_Store[TWITTER_STREAM][user_id].setState(UNMONITORED)
      }
      twitterStream.Close()
		} else if (tweet_resp.ScanUserIdStr != nil) {
			account_id := *tweet_resp.ScanUserIdStr
			log.Printf("Account Store contents %v\n", Account_Store)
			log.Println("Tweet from twitterstream")
			log.Printf("UserId %s\n", account_id)
			_, present := Account_Store[TWITTER_STREAM][account_id]
			if !present {
			  retweet_account_id := *tweet_resp.RetweetUserIdStr
        _, retweet_present := Account_Store[TWITTER_STREAM][retweet_account_id]
        if retweet_present {
          log.Printf("Skipping %s because it is a retweet of a monitored account %s\n", account_id, retweet_account_id)
          continue;
        } else {
				  log.Printf("Initializing non-existant Account_Store for %s because it was sent from twitterstream\n", account_id)
				  init_Account_Store_Entry(TWITTER_STREAM, account_id)
        }
			}
			Account_Store[TWITTER_STREAM][account_id].setLastUpdate()
		} else {
			log.Printf("WTF: %v\n", *tweet_resp)
			// WTF!!!!
		}
	}
}

