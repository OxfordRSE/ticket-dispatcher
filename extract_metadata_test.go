package main

import (
	"net/mail"
	"strings"
	"testing"
)

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
			gotIssue, gotRepo := extractIssueNumberFrom(tc.to)
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

func TestExtractIssueNumberFromMessage(t *testing.T) {
	setupTests(t)
	tests := []struct {
		name      string
		raw       string
		wantIssue string
		wantRepo  string
	}{
		{
			name: "bcc header retained by the sending MTA",
			raw: "From: John Doe <johndoe@example.com>\r\n" +
				"To: Jane Doe <janedoe@example.com>\r\n" +
				"Bcc: 123@issues.example.com\r\n" +
				"\r\nhello\r\n",
			wantIssue: "123",
		},
		{
			name: "bcc recovered from the SES received trace",
			raw: "Received: from mail.example.com (mail.example.com [203.0.113.1])\r\n" +
				" by inbound-smtp.eu-west-2.amazonaws.com with SMTP id abc123\r\n" +
				" for 456+other-repo@issues.example.com; Tue, 08 Sep 2026 09:00:00 +0000 (UTC)\r\n" +
				"From: John Doe <johndoe@example.com>\r\n" +
				"To: Jane Doe <janedoe@example.com>\r\n" +
				"\r\nhello\r\n",
			wantIssue: "456",
			wantRepo:  "other-repo",
		},
		{
			name: "bcc recovered from X-Original-To",
			raw: "From: John Doe <johndoe@example.com>\r\n" +
				"To: Jane Doe <janedoe@example.com>\r\n" +
				"X-Original-To: <789@issues.example.com>\r\n" +
				"\r\nhello\r\n",
			wantIssue: "789",
		},
		{
			name: "To takes precedence over an unrelated received trace",
			raw: "Received: from mail.example.com (mail.example.com [203.0.113.1])\r\n" +
				" by inbound-smtp.eu-west-2.amazonaws.com with SMTP id abc123\r\n" +
				" for 999@issues.example.com; Tue, 08 Sep 2026 09:00:00 +0000 (UTC)\r\n" +
				"From: John Doe <johndoe@example.com>\r\n" +
				"To: 123@issues.example.com\r\n" +
				"\r\nhello\r\n",
			wantIssue: "123",
		},
		{
			name: "no ticket address anywhere",
			raw: "Received: from mail.example.com (mail.example.com [203.0.113.1])\r\n" +
				" by inbound-smtp.eu-west-2.amazonaws.com with SMTP id abc123\r\n" +
				" for janedoe@example.com; Tue, 08 Sep 2026 09:00:00 +0000 (UTC)\r\n" +
				"From: John Doe <johndoe@example.com>\r\n" +
				"To: Jane Doe <janedoe@example.com>\r\n" +
				"\r\nhello\r\n",
			wantIssue: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := mail.ReadMessage(strings.NewReader(tc.raw))
			if err != nil {
				t.Fatalf("error parsing message: %v", err)
			}
			gotIssue, gotRepo := extractIssueNumber(msg.Header)
			if gotIssue != tc.wantIssue {
				t.Errorf("extractIssueNumber issue mismatch:\n--- got ---\n%q\n--- want ---\n%q\n", gotIssue, tc.wantIssue)
			}
			if gotRepo != tc.wantRepo {
				t.Errorf("extractIssueNumber repo mismatch:\n--- got ---\n%q\n--- want ---\n%q\n", gotRepo, tc.wantRepo)
			}
		})
	}
}

func TestReceivedFor(t *testing.T) {
	tests := []struct {
		received string
		want     string
	}{
		{
			received: "from mail.example.com (mail.example.com [203.0.113.1]) by inbound-smtp.eu-west-2.amazonaws.com with SMTP id abc123 for 123@issues.example.com; Tue, 08 Sep 2026 09:00:00 +0000 (UTC)",
			want:     "123@issues.example.com",
		},
		{
			received: "by mx.example.com with SMTP id abc123 for <123@issues.example.com>; Tue, 08 Sep 2026 09:00:00 +0000",
			want:     "123@issues.example.com",
		},
		{
			received: "from mail.example.com by mx.example.com with ESMTPS id abc123; Tue, 08 Sep 2026 09:00:00 +0000",
			want:     "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if got := receivedFor(tc.received); got != tc.want {
				t.Errorf("receivedFor mismatch:\n--- got ---\n%q\n--- want ---\n%q\n", got, tc.want)
			}
		})
	}
}
