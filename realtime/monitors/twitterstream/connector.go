package twitterstream

import (
	"log"
	"time"

	"engines/twitterstream"

	"realtime/account_entry"
	"realtime/account_store"
	"realtime/credential"
	"realtime/manager"
	"realtime/state"
)

type Connector struct {
	manager.BaseConnector
	stream *twitterstream.TwitterStream
}

func NewConnector(store *account_store.Store, credential *credential.Credential) *Connector {
	c := new(Connector)
	c.InitBaseConnector(NAME, store, credential)
	return c
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
	store := c.Store()

	// If something changes the Credential we do not want to create a race-condition here
	// so use what the function was instantiated with, to be extra-safe
	credential := c.Credential()
	AppId := credential.AppId()
	AppSecret := credential.AppSecret()
	ApiOauthToken := credential.ApiOauthToken()
	ApiOauthTokenSecret := credential.ApiOauthTokenSecret()

	c.Logger.Debugf("Filter initiated to state %s\n", c.State().State())
	if *c.State().State() != state.STARTUP {
		return
	}

	c.stream = twitterstream.New()
	slice := store.AccountSlice()

	c.State().SetState(state.UP)
	c.Logger.Debug("Filter is up")
	for {
		if *c.State().State() == state.SHUTDOWN {
			break
		}
		if c.stream.Up() == false {
			if len(slice) == 0 {
				c.Logger.Debug("no need to open connector yet")
				c.State().Sleep(10 * time.Second)
			} else {
				err := c.stream.Open(AppId, AppSecret, ApiOauthToken, ApiOauthTokenSecret, slice)
				if err != nil {
					c.Logger.Warningf("Attempted to open connection but failed: %s - sleeping for 60 seconds\n", err)
					c.State().Sleep(60 * time.Second)
				} else {
					c.Logger.Info("connector opened")
					for _, account_id := range slice {
						account, account_present := store.AccountEntry(account_id)
						if account_present {
							account.SetState(account_entry.MONITORED)
						}
					}
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
		if resp.ScanUserIdStr != "" {
			account_id := resp.ScanUserIdStr
			c.Logger.Debugf("New content for account %s\n", account_id)
			account, present := store.AccountEntry(account_id)
			if present {
				account.SetLastUpdate()
				continue
			}
			if resp.RetweetUserIdStr != "" {
				retweet_account_id := resp.RetweetUserIdStr
				_, retweet_present := store.AccountEntry(retweet_account_id)
				if retweet_present {
					c.Logger.Debugf("Skipping %s because it is a retweet of a monitored account %s\n", account_id, retweet_account_id)
				} else {
					create := false
					for _, user_mention := range resp.UserMentions {
						if user_mention.IdStr == account_id {
							create = true
							c.Logger.Debugf("Skipping %s because it was found in user mentions for account %s\n", account_id, retweet_account_id)
							break
						}
					}
					if create {
						c.Logger.Warningf("Initializing non-existant store for %s  - this should not happen, content %+v\n", account_id, resp)
						log.Fatalf("WTF: %s\n", resp.Rawsource)
						account = store.AddAccountEntry(account_id)
						account.SetLastUpdate()
					}
				}
				continue
			}
		}
		c.Logger.Debugf("Do not know how to handle incoming content %v", *resp)
	}
	c.Logger.Info("Shutting down filter()")
	c.stream.Close()
	c.State().SetState(state.DOWN)
	return
}
