package twitterstream

import (
  "net/url"
  "strings"
  "log"
  "time"
	"encoding/json"
  "errors"

  "engines/github.com.garyburd.go-oauth/oauth"
)

const (
  FilterUrl = "https://stream.twitter.com/1.1/statuses/filter.json"
)

type State int
const (
  DOWN State = iota
  WAITING
  STARTING
  UP
  CLOSING
)

type TwitterStream struct {
	token string
  token_secret string
  oauth_token string
  oauth_token_secret string
  state State
  UserIds []string
  *Stream
}

func (stream *TwitterStream) State() (State) {
  return stream.state
}

func (stream *TwitterStream) Credentials(token string, token_secret string, oauth_token string, oauth_token_secret string) {
  if stream.token != token || stream.token_secret != token_secret || stream.oauth_token != oauth_token || stream.oauth_token_secret != oauth_token_secret {
    stream.Close()
    stream.token = token
    stream.token_secret = token_secret
    stream.oauth_token = oauth_token
    stream.oauth_token_secret = oauth_token_secret
  }
}

func (stream *TwitterStream) Close() {
  stream.state = CLOSING
  if stream.Stream != nil {
log.Printf("Closing stream %s", stream.Stream)
    stream.Stream.Close()
    stream.Stream = nil
  }
  stream.state = DOWN
}

func (stream *TwitterStream) UnmarshalNext() (*TweetResponse, error) {
  var t TweetResponse
  if stream.state != UP {
    time.Sleep(1*time.Second)
    return nil, errors.New("stream not up")
  }
  if stream.Err() != nil {
    return nil, stream.Err()
  }
  tweet, err := stream.Stream.Next()
	if err != nil {
    return nil, err
  } 
	log.Printf("received from twitter: %s\n", tweet)
	
	if err := json.Unmarshal(tweet, &t.Tweet); err != nil {
    return nil, err
	}
  if t.Tweet.RetweetedStatus.User.IdString != nil {
    t.RetweetUserId = t.Tweet.RetweetedStatus.User.Id
    t.RetweetUserIdStr = t.Tweet.RetweetedStatus.User.IdString
  }

	if t.Tweet.InReplyToUserIdStr != nil {
		t.ScanUserId = t.Tweet.InReplyToUserId
		t.ScanUserIdStr = t.Tweet.InReplyToUserIdStr
	} else {
		t.ScanUserId = t.Tweet.User.Id
		t.ScanUserIdStr = t.Tweet.User.IdString
	}
	
  return &t, nil
}

type User struct {
  Id *int64 `json:"id"`
  IdString *string `json:"id_str"`
}

type RetweetedStatus struct {
  Id *int64 `json:"id"`
  IdString *string `json:"id_str"`
  User User `json:"user"`
}


type Reweet struct {
  Id *int64 `json:"id"`
  IdString *string `json:"id_str"`
}

type UnmarshalledTweet struct {
	User User `json:"user"`

	InReplyToUserId *int64 `json:"in_reply_to_user_id"`
	InReplyToUserIdStr *string `json:"in_reply_to_user_id_str"`

  RetweetedStatus RetweetedStatus `json:"retweeted_status"`
}

type TweetResponse struct {
	Tweet UnmarshalledTweet
	ScanUserId *int64
	ScanUserIdStr *string

	RetweetUserId *int64
	RetweetUserIdStr *string
}

func (stream *TwitterStream) Filter(userIds []string) {
  stream.Close()

  stream.UserIds = userIds
  

  params := url.Values{"follow": {strings.Join(userIds, ",")}}
  wait := 1
  maxWait := 600
  if len(userIds) == 0 {
    time.Sleep(1*time.Second)
    stream.state = WAITING
    log.Println("Nothing to filter, not opening connection")
    return
  }
  stream.state = STARTING
  for {
log.Println("Opening new twitterstream")
    s, err := Open(
      &oauth.Client{
        Credentials: oauth.Credentials{
            Token:  stream.token,
            Secret: stream.token_secret,
        },
      },
      &oauth.Credentials{
        Token:  stream.oauth_token,
        Secret: stream.oauth_token_secret,
      },
      FilterUrl,
      params,
    )
    if err != nil {
      log.Printf("tracking failed: %s", err)
      wait = wait << 1
      log.Printf("waiting for %d seconds before reconnect", min(wait, maxWait))
      time.Sleep(time.Duration(min(wait, maxWait)) * time.Second)
      continue
    } else {
      wait = 1
    }
log.Println("Connected to Twitter Stream!\n")
    stream.Stream = s
    stream.state = UP
    return
  }
}


func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}

func New() (*TwitterStream) {
	var stream TwitterStream
  stream.state = DOWN
	return &stream
}

