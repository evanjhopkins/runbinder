package taskconfig

import (
	"strings"
	"testing"
)

func TestParseCommandForms(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "scalar",
			yaml: "namespace: example\ncommand: echo hello\ncron: '* * * * *'\n",
			want: "echo hello",
		},
		{
			name: "sequence",
			yaml: "namespace: example\ncommand:\n  - echo one\n  - echo two\nschedule:\n  time_of_day: ['12:00:00']\n",
			want: "echo one && echo two",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := Parse([]byte(test.yaml))
			if err != nil {
				t.Fatal(err)
			}
			if got := cfg.Command.Shell(); got != test.want {
				t.Fatalf("command = %q, want %q", got, test.want)
			}
		})
	}
}

func TestParseRejectsInvalidDefinitions(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{"unknown field", "namespace: x\ncommand: echo x\ncron: '* * * * *'\ncrno: nope\n", "field crno not found"},
		{"missing schedule", "namespace: x\ncommand: echo x\n", "at least one"},
		{"zero interval", "namespace: x\ncommand: echo x\nschedule:\n  window_interval:\n    start: '10:00:00'\n    stop: '11:00:00'\n    interval_sec: 0\n", "greater than zero"},
		{"backward window", "namespace: x\ncommand: echo x\nschedule:\n  window_interval:\n    start: '11:00:00'\n    stop: '10:00:00'\n    interval_sec: 60\n", "must not be before"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse([]byte(test.yaml))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestCanonicalIsStable(t *testing.T) {
	first, err := Parse([]byte("namespace: x\ncommand: echo x\ncron: '* * * * *'\n"))
	if err != nil {
		t.Fatal(err)
	}
	definition, hash, err := first.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	second, err := Parse([]byte(definition))
	if err != nil {
		t.Fatal(err)
	}
	_, secondHash, err := second.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if hash != secondHash {
		t.Fatalf("hash changed after canonical round trip: %s != %s", hash, secondHash)
	}
}
