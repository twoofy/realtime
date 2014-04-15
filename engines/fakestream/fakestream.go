package fakestream

import (
	"math/rand"
	"sync"
	"time"
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

type FakeStream struct {
	rwlock  sync.RWMutex
	open    bool
	userIds []string
}

func (stream *FakeStream) Close() {
	stream.rwlock.Lock()
	defer stream.rwlock.Unlock()
	if stream.Up() {
		stream.open = false
	}
}

func (stream *FakeStream) UnmarshalNext() (*FakeResponse, error) {
	rand.Seed(time.Now().Unix())
	secs := time.Duration((rand.Intn(9)+1)*(1000/len(stream.userIds))) * time.Millisecond

	reloadTimer := time.Tick(secs)
	//log.Printf("Sleeping for %s\n", secs)
	for {
		select {
		case <-reloadTimer:
			j_response := FakeResponse{IdStr: stream.userIds[rand.Intn(len(stream.userIds))]}
			return &j_response, nil
		}
	}
}

type FakeResponse struct {
	Id    int64
	IdStr string
}

func (stream *FakeStream) Up() bool {
	return stream.open
}

func (stream *FakeStream) Open(token string, token_secret string, oauth_token string, oauth_token_secret string, userIds []string) error {
	if len(userIds) == 0 {
		time.Sleep(1 * time.Second)
		return nil
	}
	stream.userIds = userIds
	stream.open = true

	return nil
}

func New() *FakeStream {
	streamPtr := new(FakeStream)
	return streamPtr
}
