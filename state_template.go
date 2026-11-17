package schemaui

import (
	"strconv"
	"strings"
)

// expandStateTemplate replaces every {{stateKey}} in s with the value of
// that key in the state store. {{missing}} expands to the empty string. A
// bare string with no template markers is returned untouched.
//
// The store keeps strings and bools in separate maps; a bool key renders
// as "true" or "false". An integer key renders as its decimal form.
func expandStateTemplate(s string, st *StateStore) string {
	if !strings.Contains(s, "{{") {
		return s
	}
	var sb strings.Builder
	i := 0
	for i < len(s) {
		start := strings.Index(s[i:], "{{")
		if start < 0 {
			sb.WriteString(s[i:])
			break
		}
		sb.WriteString(s[i : i+start])
		end := strings.Index(s[i+start:], "}}")
		if end < 0 {
			// Unmatched opener — treat as literal
			sb.WriteString(s[i+start:])
			break
		}
		key := strings.TrimSpace(s[i+start+2 : i+start+end])
		sb.WriteString(stateKeyString(key, st))
		i = i + start + end + 2
	}
	return sb.String()
}

// stateKeyString resolves a single template key to its store value.
// Missing key → "". Bool key → "true"/"false".
func stateKeyString(key string, st *StateStore) string {
	if st == nil || key == "" {
		return ""
	}
	// Try string map first (text input, select), then bool (checkbox).
	if v, ok := st.stringValue(key); ok {
		return v
	}
	if v, ok := st.boolValue(key); ok {
		return strconv.FormatBool(v)
	}
	return ""
}

// stringValue is the mutex-protected read of a string key.
func (s *StateStore) stringValue(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.strings[key]; ok {
		return *p, true
	}
	return "", false
}

// boolValue is the mutex-protected read of a bool key.
func (s *StateStore) boolValue(key string) (bool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.bools[key]; ok {
		return *p, true
	}
	return false, false
}
