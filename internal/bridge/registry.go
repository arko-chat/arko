package bridge

import "fmt"

var global NativeBridge

// Register is called once from native (Swift/Kotlin) before Start().
func Register(b NativeBridge) {
	global = b
}

// Get returns the registered bridge. Panics if Register was never called.
func Get() NativeBridge {
	if global == nil {
		panic("bridge: no NativeBridge registered — call bridge.Register() before bridge.Start()")
	}
	return global
}

// Safe returns the bridge and an error instead of panicking.
func Safe() (NativeBridge, error) {
	if global == nil {
		return nil, fmt.Errorf("bridge: no NativeBridge registered")
	}
	return global, nil
}
