package state

import (
	"sync"
	"time"
)

type StateEnum string

const (
	DOWN     StateEnum = "DOWN"
	STARTUP  StateEnum = "STARTUP"
	UP       StateEnum = "UP"
	SHUTDOWN StateEnum = "SHUTDOWN"
)

type State struct {
	state    StateEnum
	rwlock   sync.RWMutex
	wg       sync.WaitGroup
	sleeping chan bool
	restart  bool
}

func NewState() *State {
	var state State

	state.state = DOWN

	return &state
}

func (state *State) Wait() {
	state.wg.Wait()
}

func (state *State) State() *StateEnum {
	state.rwlock.RLock()
	defer state.rwlock.RUnlock()
	return &state.state
}

func (state *State) Sleeping() bool {
	state.rwlock.RLock()
	defer state.rwlock.RUnlock()
	return state.sleeping != nil
}

func (state *State) Sleep(dur time.Duration) {
	timer := time.NewTimer(dur)
	state.rwlock.Lock()
	state.sleeping = make(chan bool)
	state.rwlock.Unlock()
	select {
	case <-state.sleeping:
		timer.Stop()
	case <-timer.C:
	}
	state.rwlock.Lock()
	state.sleeping = nil
	state.rwlock.Unlock()
}

func (state *State) SetState(new_state StateEnum) bool {
	if new_state == state.state {
		return true
	}
	if state.Sleeping() {
		state.sleeping <- true
	}
	state.rwlock.Lock()
	defer state.rwlock.Unlock()
	if state.state == "" && new_state == DOWN {
		state.state = DOWN
		return true
	} else if state.state == DOWN && new_state == STARTUP {
		//log.Printf("Add STARTUP WG for %s\n", state.name)
		state.wg.Add(1)
	} else if state.state == STARTUP && new_state == UP {
		//log.Printf("Done STARTUP WG for %s\n", state.name)
		state.wg.Done()
	} else if state.state == UP && new_state == SHUTDOWN {
		//log.Printf("Add SHUTDOWN WG for %s\n", state.name)
		state.wg.Add(1)
	} else if state.state == SHUTDOWN && new_state == DOWN {
		//log.Printf("Done SHUTDOWN WG for %s\n", state.name)
		state.wg.Done()
	} else {
		//log.Printf("Cannot change for %s state from '%s' to '%s'\n", state.name, state.state, new_state)
		return false
	}
	//log.Printf("Changing state for %s from '%s' to '%s'\n", state.name, state.state, new_state)
	state.state = new_state
	return true
}
