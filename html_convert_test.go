package main

import (
	"testing"
)

func TestHtmlToPlain(t *testing.T) {
	t.Setenv("TICKET_DISPATCHER_DOMAIN", "issues.example.com")
	t.Setenv("WHITELIST_DOMAIN", "example.ac.uk")
	t.Setenv("GITHUB_PROJECT", "example/repo")
	loadConfig()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "simple paragraph and entity unescape",
			in:   `<p>Hello &amp; welcome</p>`,
			want: "Hello & welcome",
		},
		{
			name: "heading and paragraph",
			in:   `<h2>Title</h2><p>First para</p><p>Second para</p>`,
			want: "## Title\n\nFirst para\n\nSecond para",
		},
		{
			name: "bold and italic and inline code",
			in:   `This is <b>bold</b> and <i>italic</i> and <code>inline()</code>`,
			want: "This is **bold** and *italic* and `inline()`",
		},
		{
			name: "link with href different from text",
			in:   `See <a href="https://example.com">project</a> updates.`,
			want: "See project (https://example.com) updates.",
		},
		{
			name: "unordered and ordered lists",
			in: `<ul>
<li>one</li>
<li>two</li>
</ul>
<ol>
<li>first</li>
<li>second</li>
</ol>`,
			want: "- one\n- two\n\n1. first\n2. second",
		},
		{
			name: "image with alt text",
			in:   `<p>Look: <img src="https://img.example/x.png" alt="logo"></p>`,
			want: "Look: ![logo](https://img.example/x.png)",
		},
		{
			name: "normalize multiple blank lines",
			in:   `<p>A</p><div></div><p>B</p><p></p><p>C</p>`,
			want: "A\n\nB\n\nC",
		},
		{
			name: "links where href equals text",
			in:   `Visit <a href="https://example.com">https://example.com</a> now.`,
			want: "Visit https://example.com now.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := htmlToPlain(tc.in)
			if err != nil {
				t.Fatalf("htmlToPlain returned error: %v", err)
			}
			if got != tc.want {
				t.Errorf("htmlToPlain mismatch:\n--- got ---\n%q\n--- want ---\n%q\n", got, tc.want)
			}
		})
	}
}
