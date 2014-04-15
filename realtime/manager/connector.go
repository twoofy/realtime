package manager

import (
	"fmt"

	"realtime/account_store"
	"realtime/credential"
)

type BaseConnector struct {
	baseManager
}

func (b *BaseConnector) InitBaseConnector(name string, store *account_store.Store, credential *credential.Credential) {
	b.initbaseManager(name, store, credential)
	b.Logger.Logprefix = fmt.Sprintf("manager %s, type %s", name, b.Type())
}

func (b *BaseConnector) Type() ConnectorEnum {
	return CONNECTOR
}
