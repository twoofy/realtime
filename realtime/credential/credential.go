package credential

import (
	"encoding/json"
	"io"
	"sync"
)

type JsonCredential struct {
	AppId               string `json:"app_id"`
	AppSecret           string `json:"app_secret"`
	ApiOauthToken       string `json:"api_oauth_token"`
	ApiOauthTokenSecret string `json:"api_oauth_token_secret"`
}
type Credential struct {
	app_id string
	app_secret string
	api_oauth_token string
	api_oauth_token_secret string
	stale               bool
	rwlock              sync.RWMutex
}

func NewCredential() *Credential {
	var c Credential
	c.stale = true
	return &c
}

func (credential *JsonCredential) Valid() bool {
	if credential.AppId == "" || credential.AppSecret == "" || credential.ApiOauthToken == "" || credential.ApiOauthTokenSecret == "" {
		return false
	}
	return true
}

func CredentialFromJson(r io.ReadCloser) *JsonCredential {
	var c JsonCredential
	dec := json.NewDecoder(r)

	err := dec.Decode(&c)

	if err != nil {
		return nil
	}
	return &c
}

func (c *Credential) Changed(new_credential *JsonCredential) bool {
	c.rwlock.RLock()
	defer c.rwlock.RUnlock()

	if c.app_id == new_credential.AppId && c.app_secret == new_credential.AppSecret && c.api_oauth_token == new_credential.ApiOauthToken && c.api_oauth_token_secret == new_credential.ApiOauthTokenSecret {
		return false
	}
	return true
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
func (c *Credential) Credential() *Credential {
	c.rwlock.RLock()
	defer c.rwlock.RUnlock()
	return c
}

func (c *Credential) Update(new_credential *JsonCredential) {
	if false && c.stale == false {
		return
	}
	if false && ! c.Changed(new_credential) {
		return
	}
	c.rwlock.Lock()
	defer c.rwlock.Unlock()
	c.app_id = new_credential.AppId
	c.app_secret = new_credential.AppSecret
	c.api_oauth_token = new_credential.ApiOauthToken
	c.api_oauth_token_secret = new_credential.ApiOauthTokenSecret
	c.stale = false

	return
}

func (c *Credential) AppId() string {
	c.rwlock.RLock()
	defer c.rwlock.RUnlock()

	return c.app_id
}
func (c *Credential) AppSecret() string {
	c.rwlock.RLock()
	defer c.rwlock.RUnlock()

	return c.app_secret
}
func (c *Credential) ApiOauthToken() string {
	c.rwlock.RLock()
	defer c.rwlock.RUnlock()

	return c.api_oauth_token
}
func (c *Credential) ApiOauthTokenSecret() string {
	c.rwlock.RLock()
	defer c.rwlock.RUnlock()

	return c.api_oauth_token_secret
}
