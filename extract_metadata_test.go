package main

import "testing"

func setupTests(t *testing.T) {
	t.Setenv("TICKET_DISPATCHER_DOMAIN", "issues.example.com")
	t.Setenv("WHITELIST_DOMAIN", "example.com")
	t.Setenv("GITHUB_PROJECT", "example/repo")
	loadConfig()
}
func TestExtractIssueNumber(t *testing.T) {
	setupTests(t)
	tests := []struct {
		to   string
		want string
	}{{
		to:   "John Doe <johndoe@example.com>",
		want: "",
	},
		{to: "John Doe <johndoe@example.com>, 123@issues.example.com",
			want: "123",
		},
	}

	for _, tc := range tests {
		t.Run(tc.to, func(t *testing.T) {
			got := extractIssueNumber(tc.to, "")
			if got != tc.want {
				t.Errorf("extractIssueNumber mismatch:\n--- got ---\n%q\n--- want ---\n%q\n", got, tc.want)
			}
		})
	}
}

func TestExtractSenderDomain(t *testing.T) {
	setupTests(t)
	tests := []struct {
		from string
		want string
	}{
		{from: "John Doe <john.doe@example.com", want: "example.com"},
		{from: "jane.doe@example.com", want: "example.com"},
		{from: "rincewind@unseen.ac.uk", want: "unseen.ac.uk"},
	}
	for _, tc := range tests {
		t.Run(tc.from, func(t *testing.T) {
			got := extractSenderDomain(tc.from)
			if got != tc.want {
				t.Errorf("extractSenderDomain mismatch:\n--- got ---\n%q\n--- want ---\n%q\n", got, tc.want)
			}
		})
	}
}
