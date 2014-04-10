package state

import (
	"log"
	"sync"
)

type monitoredEnum string
const (
	DOWN monitoredEnum = "DOWN"
	STARTUP monitoredEnum = "STARTUP"
	UP monitoredEnum = "UP"
	SHUTDOWN monitoredEnum = "SHUTDOWN"
)

type MonitoredState struct {
	name string
	state monitoredEnum
	rwlock sync.RWMutex
	wg sync.WaitGroup
}

func New(name string) (*MonitoredState) {
	var state MonitoredState

	state.name = name
	state.state = DOWN

	return &state
}

func (state *MonitoredState) Wait() {
	state.wg.Wait()
}


func (state *MonitoredState) State() (monitoredEnum) {
	state.rwlock.RLock()
	defer state.rwlock.RUnlock()
	return state.state
}

func (state *MonitoredState) SetState(new_state monitoredEnum) (bool) {
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
		log.Println("Add SHUTDOWN WG for %s\n", state.name)
		state.wg.Add(1)
	} else if state.state == SHUTDOWN && new_state == DOWN {
		log.Println("Done SHUTDOWN WG for %s\n", state.name)
		state.wg.Done()
	} else {
		log.Println("Cannot change for %s state from '%s' to '%s'\n", state.name, state.state, new_state)
		return false
	}
	log.Printf("Changing state for %s from '%s' to '%s'\n", state.name, state.state, new_state)
	state.state = new_state
	return true
}

