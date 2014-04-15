package twitterstream

import (
	"encoding/json"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

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
	*Stream
	rwlock sync.RWMutex
}

func (stream *TwitterStream) Close() {
	stream.rwlock.Lock()
	defer stream.rwlock.Unlock()
	if stream.Up() {
		log.Printf("Closing twitterstream")
		stream.Stream.Close()
		stream.Stream = nil
	}
}

func (stream *TwitterStream) UnmarshalNext() (*TweetResponse, error) {
	var t TweetResponse
	if stream.Up() == false {
		return nil, nil
	}
	if stream.Err() != nil {
		return nil, stream.Err()
	}
	tweet, err := stream.Stream.Next()
	if err != nil {
		return nil, err
	}
	t.Rawsource = tweet

	if err := json.Unmarshal(tweet, &t.Tweet); err != nil {
		return nil, err
	}
	if t.Tweet.RetweetedStatus.User.IdString != "" {
		t.RetweetUserId = t.Tweet.RetweetedStatus.User.Id
		t.RetweetUserIdStr = t.Tweet.RetweetedStatus.User.IdString

		for _, ent := range t.Tweet.RetweetedStatus.Entities.UserMentions {
			t.UserMentions = append(t.UserMentions, TweetUserMention{Id: ent.Id, IdStr: ent.IdString})
		}
	} else {
		for _, ent := range t.Tweet.Entities.UserMentions {
			t.UserMentions = append(t.UserMentions, TweetUserMention{Id: ent.Id, IdStr: ent.IdString})
		}
	}

	if t.Tweet.InReplyToUserIdStr != "" {
		t.ScanUserId = t.Tweet.InReplyToUserId
		t.ScanUserIdStr = t.Tweet.InReplyToUserIdStr
	} else {
		t.ScanUserId = t.Tweet.User.Id
		t.ScanUserIdStr = t.Tweet.User.IdString

	}

	return &t, nil
}

type User struct {
	Id       int64  `json:"id"`
	IdString string `json:"id_str"`
}

type RetweetedStatus struct {
	Id       int64  `json:"id"`
	IdString string `json:"id_str"`
	User     User   `json:"user"`
	Entities `json:"entities"`
}

type UserMention struct {
	Id       int64  `json:"id"`
	IdString string `json:"id_str"`
}

type UserMentions struct {
	UserMention []UserMention `json:"user_mentions"`
}

type Entities struct {
	UserMentions []UserMention `json:"user_mentions"`
}

type UnmarshalledTweet struct {
	User User `json:"user"`

	InReplyToUserId    int64  `json:"in_reply_to_user_id"`
	InReplyToUserIdStr string `json:"in_reply_to_user_id_str"`

	RetweetedStatus RetweetedStatus `json:"retweeted_status"`
	Entities        `json:"entities"`
}

type TweetUserMention struct {
	Id    int64
	IdStr string
}

type TweetResponse struct {
	Tweet         UnmarshalledTweet
	ScanUserId    int64
	ScanUserIdStr string

	RetweetUserId    int64
	RetweetUserIdStr string

	UserMentions []TweetUserMention
	Rawsource    []byte
}

func (stream *TwitterStream) Up() bool {
	if stream.Stream == nil {
		return false
	}
	return true
}

func (stream *TwitterStream) Open(token string, token_secret string, oauth_token string, oauth_token_secret string, userIds []string) error {
	stream.rwlock.Lock()
	defer stream.rwlock.Unlock()

	params := url.Values{"follow": {strings.Join(userIds, ",")}}
	if len(userIds) == 0 {
		time.Sleep(1 * time.Second)
		return nil
	}
	s, err := Open(
		&oauth.Client{
			Credentials: oauth.Credentials{
				Token:  token,
				Secret: token_secret,
			},
		},
		&oauth.Credentials{
			Token:  oauth_token,
			Secret: oauth_token_secret,
		},
		FilterUrl,
		params,
	)
	if err == nil {
		stream.Stream = s
	} else {
		stream.Stream = nil
	}
	return err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func New() *TwitterStream {
	var stream TwitterStream
	return &stream
}
