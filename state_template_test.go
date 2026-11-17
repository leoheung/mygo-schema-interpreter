package schemaui

import "testing"

func TestExpandStateTemplate(t *testing.T) {
	st := newStateStore()
	st.StringPtr("demo.input", "hello")
	st.StringPtr("demo.region", "euw")
	st.BoolPtr("demo.tos")
	*st.BoolPtr("demo.tos") = true

	tests := []struct {
		name, in, want string
	}{
		{"plain text", "no template", "no template"},
		{"single string", "Echo: {{demo.input}}", "Echo: hello"},
		{"two strings", "{{demo.input}} @ {{demo.region}}", "hello @ euw"},
		{"bool", "tos={{demo.tos}}", "tos=true"},
		{"missing key", "say {{unknown}}", "say "},
		{"empty string val", "got={{empty}}", "got="},
		{"nil store", "x={{k}}", "x="},
		{"braces but wrong format", "{{demo.input", "{{demo.input"},
		{"whitespace in key is trimmed", "{{ demo.input }}", "hello"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			if tc.name == "nil store" {
				got = expandStateTemplate(tc.in, nil)
			} else {
				got = expandStateTemplate(tc.in, st)
			}
			if got != tc.want {
				t.Errorf("expandStateTemplate(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
