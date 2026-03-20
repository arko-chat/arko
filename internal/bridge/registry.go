package bridge

import (
	"github.com/arko-chat/arko/internal/errors"
)

var global NativeBridge

func Register(b NativeBridge) {
	global = b
}

func Get() NativeBridge {
	return global
}

func Safe() (NativeBridge, error) {
	if global == nil {
		return nil, errors.BridgeNotInitialized()
	}
	return global, nil
}
