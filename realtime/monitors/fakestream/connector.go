package fakestream

import (
	"time"

	"engines/fakestream"

	"realtime/account_entry"
	"realtime/account_store"
	"realtime/credential"
	"realtime/manager"
	"realtime/state"
)

type Connector struct {
	manager.BaseConnector
	stream *fakestream.FakeStream
}

func NewConnector(store *account_store.Store, credential *credential.Credential) *Connector {
	var c Connector
	c.InitBaseConnector(NAME, store, credential)
	return &c
}

func (c *Connector) Startup() bool {
	go c.filter()
	return true
}

func (c *Connector) Shutdown() bool {
	c.stream.Close()
	c.Credential().SetStale()
	return true
}

func (c *Connector) filter() {
	c_state := c.State()
	store := c.Store()

	// If something changes the Credential we do not want to create a race-condition here
	// so use what the function was instantiated with, to be extra-safe
	credential := c.Credential()
	AppId := credential.AppId()
	AppSecret := credential.AppSecret()
	ApiOauthToken := credential.ApiOauthToken()
	ApiOauthTokenSecret := credential.ApiOauthTokenSecret()

	c.Logger.Debugf("Filter initiated to state %s\n", *c_state.State())
	if *c_state.State() != state.STARTUP {
		return
	}

	c.stream = fakestream.New()
	slice, slice_present := store.AccountSlice()

	c_state.SetState(state.UP)
	c.Logger.Debug("Filter is up")
	for {
		if *c_state.State() == state.SHUTDOWN {
			break
		}
		if c.stream.Up() == false {
			if !slice_present {
				c.Logger.Debug("connector not open yet")
				c_state.Sleep(1 * time.Second)
			}
			err := c.stream.Open(AppId, AppSecret, ApiOauthToken, ApiOauthTokenSecret, slice)
			if err != nil {
				c.Logger.Warningf("Attempted to open connection but failed: %s - sleeping for 60 seconds\n", err)
				c_state.Sleep(60 * time.Second)
			}
			c.Logger.Info("connector opened")
			for _, account_id := range slice {
				account, account_present := store.AccountEntry(account_id)
				if account_present {
					account.SetState(account_entry.MONITORED)
				}
			}
			continue
		}
		resp, err := c.stream.UnmarshalNext()
		// stream is down to get resp == nil and err == nil
		if resp == nil && err == nil {
			continue
		}
		if err != nil {
			c.Logger.Warningf("UnmarshalNext error %s\n", err)
			for _, account_id := range slice {
				account, account_present := store.AccountEntry(account_id)
				if account_present {
					account.SetState(account_entry.UNMONITORED)
				}
			}
			c.stream.Close()
			continue
		}
		if resp.IdStr != "" {
			account_id := resp.IdStr
			c.Logger.Debugf("New content for account %s\n", account_id)
			account, present := store.AccountEntry(account_id)
			if present {
				account.SetLastUpdate()
				continue
			} else {
				c.Logger.Warningf("Initializing non-existant store for %s  - this should not happen, content %+v\n", account_id, resp)
				account = store.AddAccountEntry(account_id)
				account.SetLastUpdate()
				continue
			}
		}
		c.Logger.Debugf("Do not know how to handle incoming content %v", *resp)
	}
	c.Logger.Info("Shutting down filter()")
	c.stream.Close()
	c_state.SetState(state.DOWN)
	return
}
