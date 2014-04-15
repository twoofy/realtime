package manager

import (
	"time"

	"realtime/account_store"
	"realtime/credential"
	"realtime/logger"
	"realtime/state"
)

type Manager interface {
	Log() *logger.Logger
	Name() string
	Type() ConnectorEnum
	Startup() bool
	Shutdown() bool
	State() *state.State
	Store() *account_store.Store
	Credential() *credential.Credential
}

type ConnectorEnum string

const (
	CONNECTOR ConnectorEnum = "connector"
	ROUTER    ConnectorEnum = "router"
)

func RestartMonitor(managers []Manager) {
	reloadTimer := time.Tick(15 * time.Second)
	for {
		select {
		case <-reloadTimer:
			for _, manager := range managers {
				t := manager.Type()
				name := manager.Name()
				if t != CONNECTOR {
					manager.Log().Debugf("Skipping restart of %s %s\n", t, name)
					continue
				}
				store := manager.Store()
				s := manager.State()
				if store.NeedsRestart() && *s.State() == state.UP {
					manager.Log().Infof("Restarting %s %s\n", t, name)
					Stop(manager)
					Start(manager)
					manager.Store().SetRestart(false)
				} else {
					manager.Log().Debugf("No need to restart %s %s\n", t, name)
				}
			}
		}
	}
}

func Start(m Manager) *state.StateEnum {
	if *m.State().State() != state.DOWN {
		m.Log().Info("not starting because its not down")
		return nil
	}
	m.State().SetState(state.STARTUP)
	m.Log().Infof("state set to %s", *m.State().State())
	// m.Startup() needs to call SetState(state.UP) or will block on Wait()
	m.Startup()
	m.State().Wait()
	m.Log().Infof("state set to %s", *m.State().State())
	return m.State().State()
}

func Stop(m Manager) *state.StateEnum {
	if *m.State().State() != state.UP {
		m.Log().Info("not shutting down its not up")
		return nil
	}
	m.State().SetState(state.SHUTDOWN)
	m.Log().Infof("state set to %s", *m.State().State())
	// m.Shutdown() needs to call SetState(state.DOWN) or will block on Wait()
	m.Shutdown()
	m.State().Wait()
	m.Log().Infof("state set to %s", *m.State().State())
	return m.State().State()
}
