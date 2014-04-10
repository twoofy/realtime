package state

import (
	"log"
	"sync"
	"time"
)

type monitoredEnum string

const (
	DOWN     monitoredEnum = "DOWN"
	STARTUP  monitoredEnum = "STARTUP"
	UP       monitoredEnum = "UP"
	SHUTDOWN monitoredEnum = "SHUTDOWN"
)

type MonitoredState struct {
	name     string
	state    monitoredEnum
	rwlock   sync.RWMutex
	wg       sync.WaitGroup
	sleeping chan bool
}

func New(name string) *MonitoredState {
	var state MonitoredState

	state.name = name
	state.state = DOWN

	return &state
}

func (state *MonitoredState) Wait() {
	state.wg.Wait()
}

func (state *MonitoredState) State() monitoredEnum {
	state.rwlock.RLock()
	defer state.rwlock.RUnlock()
	return state.state
}

func (state *MonitoredState) Sleep(dur time.Duration) {
	timer := time.NewTimer(dur)
	state.sleeping = make(chan bool)
	select {
	case <-state.sleeping:
		timer.Stop()
	case <-timer.C:
	}
	state.sleeping = nil
}

func (state *MonitoredState) SetState(new_state monitoredEnum) bool {
	if new_state == state.state {
		return true
	}
	if state.sleeping != nil {
		state.sleeping <- true
	}
	state.rwlock.Lock()
	defer state.rwlock.Unlock()
	if state.state == "" && new_state == DOWN {
		state.state = DOWN
		return true
	} else if state.state == DOWN && new_state == STARTUP {
		log.Printf("Add STARTUP WG for %s\n", state.name)
		state.wg.Add(1)
	} else if state.state == STARTUP && new_state == UP {
		log.Printf("Done STARTUP WG for %s\n", state.name)
		state.wg.Done()
	} else if state.state == UP && new_state == SHUTDOWN {
		log.Printf("Add SHUTDOWN WG for %s\n", state.name)
		state.wg.Add(1)
	} else if state.state == SHUTDOWN && new_state == DOWN {
		log.Printf("Done SHUTDOWN WG for %s\n", state.name)
		state.wg.Done()
	} else {
		log.Println("Cannot change for %s state from '%s' to '%s'\n", state.name, state.state, new_state)
		return false
	}
	log.Printf("Changing state for %s from '%s' to '%s'\n", state.name, state.state, new_state)
	state.state = new_state
	return true
}
