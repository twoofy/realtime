package fakestream

import (
	"realtime/account_store"
)

const (
	PROPERTY account_store.Property = account_store.FAKE_STREAM
	NAME     string                 = string(PROPERTY)
)
