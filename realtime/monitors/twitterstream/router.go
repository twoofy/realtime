package twitterstream

import (
	"engines/github.com.bmizerany.pat"

	"realtime/account_store"
	"realtime/credential"
	"realtime/manager"
	"realtime/state"
)

type Router struct {
	manager.BaseRouter
	pat *pat.PatternServeMux
}

func NewRouter(store *account_store.Store, credential *credential.Credential, pat *pat.PatternServeMux) *Router {
	r := new(Router)
	r.InitBaseRouter(NAME, store, credential, pat)
	return r
}

func (r *Router) Startup() bool {
	s := r.State()

	s.SetState(state.UP)
	return true
}

func (r *Router) Shutdown() bool {
	s := r.State()

	s.SetState(state.DOWN)
	return true
}
