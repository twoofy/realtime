package manager

import (
	"realtime/account_store"
	"realtime/credential"
	"realtime/logger"
	"realtime/state"
)

type baseManager struct {
	store      *account_store.Store
	state      *state.State
	credential *credential.Credential
	name       string
	Logger     logger.Logger
}

func (b *baseManager) Credential() *credential.Credential {
	return b.credential.Credential()
}
func (b *baseManager) Store() *account_store.Store {
	return b.store
}
func (b *baseManager) State() *state.State {
	return b.state
}
func (b *baseManager) Name() string {
	return b.name
}
func (b *baseManager) Log() *logger.Logger {
	return &b.Logger
}
func (b *baseManager) initbaseManager(name string, store *account_store.Store, credential *credential.Credential) {
	b.store = store
	b.state = state.NewState()
	b.credential = credential
	b.name = name
}
