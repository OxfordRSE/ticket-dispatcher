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
		to        string
		wantIssue string
		wantRepo  string
	}{
		{
			to:        "John Doe <johndoe@example.com>",
			wantIssue: "",
			wantRepo:  "",
		},
		{
			to:        "John Doe <johndoe@example.com>, 123@issues.example.com",
			wantIssue: "123",
			wantRepo:  "",
		},
		{
			to:        "123+myrepo@issues.example.com",
			wantIssue: "123",
			wantRepo:  "myrepo",
		},
		{
			to:        "John Doe <johndoe@example.com>, 456+other-repo@issues.example.com",
			wantIssue: "456",
			wantRepo:  "other-repo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.to, func(t *testing.T) {
			gotIssue, gotRepo := extractIssueNumber(tc.to, "")
			if gotIssue != tc.wantIssue {
				t.Errorf("extractIssueNumber issue mismatch:\n--- got ---\n%q\n--- want ---\n%q\n", gotIssue, tc.wantIssue)
			}
			if gotRepo != tc.wantRepo {
				t.Errorf("extractIssueNumber repo mismatch:\n--- got ---\n%q\n--- want ---\n%q\n", gotRepo, tc.wantRepo)
			}
		})
	}
}

func TestSenderDomainAllowed(t *testing.T) {
	tests := []struct {
		whitelist string
		domain    string
		want      bool
	}{
		{whitelist: "example.com", domain: "example.com", want: true},
		{whitelist: "example.com", domain: "sub.example.com", want: true},
		{whitelist: "example.com", domain: "EXAMPLE.COM", want: true},
		{whitelist: "EXAMPLE.COM", domain: "example.com", want: true},
		{whitelist: "example.com", domain: "notexample.com", want: false},
		{whitelist: "example.com", domain: "other.org", want: false},
		{whitelist: "example.com,other.org", domain: "other.org", want: true},
		{whitelist: "example.com, other.org", domain: "sub.other.org", want: true},
		{whitelist: "example.com,other.org", domain: "unrelated.net", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.whitelist+"|"+tc.domain, func(t *testing.T) {
			t.Setenv("TICKET_DISPATCHER_DOMAIN", "issues.example.com")
			t.Setenv("WHITELIST_DOMAIN", tc.whitelist)
			t.Setenv("GITHUB_PROJECT", "example/repo")
			loadConfig()
			got := senderDomainAllowed(tc.domain)
			if got != tc.want {
				t.Errorf("senderDomainAllowed(%q) with whitelist %q: got %v, want %v", tc.domain, tc.whitelist, got, tc.want)
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
