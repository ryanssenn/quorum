package core

import "sync"

type Engine struct {
	store   map[string]string
	muStore sync.Mutex
}

func NewEngine() *Engine {
	return &Engine{store: make(map[string]string)}
}

func (e *Engine) Get(key string) string {
	e.muStore.Lock()
	defer e.muStore.Unlock()
	val, ok := e.store[key]

	if !ok {
		return "key not found"
	}

	return val
}

func (e *Engine) Put(key string, value string) {
	e.muStore.Lock()
	defer e.muStore.Unlock()
	e.store[key] = value
}

// Snapshot returns a copy of the full key-value state. Used to capture the
// state machine when compacting the log into a snapshot.
func (e *Engine) Snapshot() map[string]string {
	e.muStore.Lock()
	defer e.muStore.Unlock()
	out := make(map[string]string, len(e.store))
	for k, v := range e.store {
		out[k] = v
	}
	return out
}

// Restore replaces the entire key-value state, e.g. after loading or receiving
// a snapshot.
func (e *Engine) Restore(state map[string]string) {
	e.muStore.Lock()
	defer e.muStore.Unlock()
	e.store = make(map[string]string, len(state))
	for k, v := range state {
		e.store[k] = v
	}
}
