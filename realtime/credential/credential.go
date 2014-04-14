package credential

import (
	"encoding/json"
	"io"
	"sync"
)

type Credential struct {
	AppId               string `json:"app_id"`
	AppSecret           string `json:"app_secret"`
	ApiOauthToken       string `json:"api_oauth_token"`
	ApiOauthTokenSecret string `json:"api_oauth_token_secret"`
	stale               bool
	rwlock              sync.RWMutex
}

func NewCredential() *Credential {
	var c Credential
	c.stale = true
	return &c
}

func (credential *Credential) Valid() bool {
	if credential.AppId == "" || credential.AppSecret == "" || credential.ApiOauthToken == "" || credential.ApiOauthTokenSecret == "" {
		return false
	}
	return true
}

func CredentialFromJson(r io.ReadCloser) *Credential {
	var c Credential
	dec := json.NewDecoder(r)

	err := dec.Decode(&c)

	if err != nil {
		return nil
	}
	return &c
}

func (c *Credential) Stale() bool {
	c.rwlock.RLock()
	defer c.rwlock.RUnlock()
	return c.stale
}

func (c *Credential) SetStale() {
	c.rwlock.RLock()
	if c.stale == true {
		c.rwlock.RUnlock()
		return
	}
	c.rwlock.RUnlock()

	c.rwlock.Lock()
	c.stale = true
	c.rwlock.Unlock()
}

func (c *Credential) Update(new_credential *Credential) *Credential {
	c.rwlock.RLock()
	if c.AppId == new_credential.AppId && c.AppSecret == new_credential.AppSecret && c.ApiOauthToken == new_credential.ApiOauthToken && c.ApiOauthTokenSecret == new_credential.ApiOauthTokenSecret && c.stale == false {
		c.rwlock.RUnlock()
		return c
	}
	c.rwlock.RUnlock()
	c.rwlock.Lock()
	defer c.rwlock.Unlock()

	c.AppId = new_credential.AppId
	c.AppSecret = new_credential.AppSecret
	c.ApiOauthToken = new_credential.ApiOauthToken
	c.ApiOauthTokenSecret = new_credential.ApiOauthTokenSecret
	c.stale = false

	return c
}
