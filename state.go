package schemaui

import "sync"

// StateStore keeps the local state of schema-driven input widgets: the text
// of a text input, the check of a checkbox, the choice of a select. It is
// addressed by the node's stable key, so a widget keeps its value across
// frames and schema updates as long as its key (or tree position) is stable.
//
// Drawing reads and writes on the UI thread; actions may read from other
// goroutines. Both go through the mutex.
type StateStore struct {
	mu      sync.RWMutex
	strings map[string]*string
	bools   map[string]*bool
}

func newStateStore() *StateStore {
	return &StateStore{
		strings: make(map[string]*string),
		bools:   make(map[string]*bool),
	}
}

// StringPtr returns the string state at key, creating it with init when the
// key is new. The pointer stays valid for the store's lifetime, which is what
// a mygo text input or select needs to bind to.
func (s *StateStore) StringPtr(key, init string) *string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.strings[key]; ok {
		return p
	}
	v := init
	s.strings[key] = &v
	return &v
}

// BoolPtr returns the bool state at key, creating it false when the key is
// new. The pointer stays valid for the store's lifetime.
func (s *StateStore) BoolPtr(key string) *bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.bools[key]; ok {
		return p
	}
	v := false
	s.bools[key] = &v
	return &v
}

// String returns a copy of the string state at key, or "" when unset.
func (s *StateStore) String(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.strings[key]; ok {
		return *p
	}
	return ""
}

// Bool returns the bool state at key, or false when unset.
func (s *StateStore) Bool(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.bools[key]; ok {
		return *p
	}
	return false
}
