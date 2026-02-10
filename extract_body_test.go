package main

import (
	"encoding/base64"
	"net/mail"
	"strings"
	"testing"
)

// helper to build a mail.Message from raw RFC822 text
func mustMessage(t *testing.T, raw string) *mail.Message {
	t.Helper()
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("failed to parse message: %v\nraw:\n%s", err, raw)
	}
	return msg
}

func TestExtractBodyAsMarkdown_SinglePartPlain(t *testing.T) {
	raw := "Content-Type: text/plain; charset=utf-8\r\n\r\nHello world\n"
	msg := mustMessage(t, raw)

	got, err := extractBodyAsMarkdown(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "Hello world"
	if got != want {
		t.Fatalf("unexpected body: got=%q want=%q", got, want)
	}
}

func TestExtractBodyAsMarkdown_MissingContentType(t *testing.T) {
	// No Content-Type header -> treat as plain text
	raw := "Subject: test\r\n\r\nThis is a message with no content-type.\n"
	msg := mustMessage(t, raw)

	got, err := extractBodyAsMarkdown(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "This is a message") {
		t.Fatalf("unexpected body: %q", got)
	}
}

func TestExtractBodyAsMarkdown_HTMLSinglePart(t *testing.T) {
	raw := "Content-Type: text/html; charset=utf-8\r\n\r\n<p>Hello <b>World</b></p>\r\n"
	msg := mustMessage(t, raw)

	got, err := extractBodyAsMarkdown(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// htmlToPlain output formatting can vary slightly; assert key substrings exist
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "World") {
		t.Fatalf("html conversion seems wrong: %q", got)
	}
}

func TestExtractBodyAsMarkdown_MultipartPrefersPlain(t *testing.T) {
	raw := "Content-Type: multipart/alternative; boundary=BOUNDARY42\r\n\r\n" +
		"--BOUNDARY42\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n\r\n" +
		"Plain body text\r\n" +
		"--BOUNDARY42\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n\r\n" +
		"<p>HTML body</p>\r\n" +
		"--BOUNDARY42--\r\n"

	msg := mustMessage(t, raw)
	got, err := extractBodyAsMarkdown(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Plain body text" {
		t.Fatalf("unexpected multipart result: %q", got)
	}
}

func TestExtractBodyAsMarkdown_QuotedPrintableDecoded(t *testing.T) {
	// "Hello=\r\nWorld" should decode to "HelloWorld" (soft line break)
	raw := "Content-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\nHello=\r\nWorld\r\n"
	msg := mustMessage(t, raw)
	got, err := extractBodyAsMarkdown(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "HelloWorld" {
		t.Fatalf("quoted-printable not decoded: got=%q", got)
	}
}

func TestExtractBodyAsMarkdown_Base64Decoded(t *testing.T) {
	payload := "Hi base64"
	enc := base64.StdEncoding.EncodeToString([]byte(payload))
	raw := "Content-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\n" + enc + "\r\n"
	msg := mustMessage(t, raw)
	got, err := extractBodyAsMarkdown(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != payload {
		t.Fatalf("base64 not decoded: got=%q want=%q", got, payload)
	}
}

func TestHideQuotedPart_Behavior(t *testing.T) {
	visible := "Thanks for your note."
	quoted := "> On Tue, Alice <alice@example.com> wrote:\n> Hello\n> More\n> End\n"
	md := visible + "\n\n" + quoted

	// keep quotes inside <details>
	got := hideQuotedPart(md, false)
	if !strings.Contains(got, "<details>") || !strings.Contains(got, visible) {
		t.Fatalf("expected details wrapper with visible content; got: %q", got)
	}

	// remove quotes entirely
	got2 := hideQuotedPart(md, true)
	if !strings.Contains(got2, visible) {
		t.Fatalf("expected visible content when removing quotes; got: %q", got2)
	}
	// when removing quotes we expect no "<details>"
	if strings.Contains(got2, "<details>") {
		t.Fatalf("did not expect details when removeQuotes=true: %q", got2)
	}
}
