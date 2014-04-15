package manager

import (
	"net/http"
	"fmt"

	"engines/github.com.bmizerany.pat"
	"realtime/account_store"
	"realtime/credential"
)

type BaseRouter struct {
	baseManager
	pat *pat.PatternServeMux
}

func (b *BaseRouter) InitBaseRouter(name string, store *account_store.Store, credential *credential.Credential, pat *pat.PatternServeMux) {
	b.initbaseManager(name, store, credential)
	path := "/" + name + "/:id"

	b.pat = pat

	b.pat.Put(path, http.HandlerFunc(b.HttpHandler))

	b.pat.Get(path, http.HandlerFunc(b.HttpHandler))
	b.Logger.Logprefix = fmt.Sprintf("manager %s, type %s ", name, b.Type())

}

func (b *BaseRouter) Type() ConnectorEnum {
	return ROUTER
}
