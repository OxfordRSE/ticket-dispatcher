package main

import (
	"net/mail"
	"net/textproto"
	"strings"
	"unicode"
)

// recipientHeaders are the header fields scanned for a ticket address, in order of
// preference. Bcc is normally stripped before delivery, so an address that was only
// Bcc'd is usually recovered from the delivery headers or the Received trace instead.
var recipientHeaders = []string{"To", "Cc", "Bcc", "X-Original-To", "Delivered-To", "Envelope-To", "X-Envelope-To"}

// extractIssueNumber scans the recipient headers of a message, including the envelope
// recipients recorded in the Received trace, and returns the first numeric local-part
// found at the ticket domain, along with an optional repo suffix extracted from a + tag
// (e.g. 123+myrepo@domain → "123", "myrepo").
func extractIssueNumber(h mail.Header) (issueNumber, repoSuffix string) {
	var headers []string
	for _, name := range recipientHeaders {
		headers = append(headers, h[textproto.CanonicalMIMEHeaderKey(name)]...)
	}
	for _, received := range h["Received"] {
		if addr := receivedFor(received); addr != "" {
			headers = append(headers, addr)
		}
	}
	return extractIssueNumberFrom(headers...)
}

// extractIssueNumberFrom scans address-list header values in order and returns the first
// ticket address found.
func extractIssueNumberFrom(headers ...string) (issueNumber, repoSuffix string) {
	// ParseAddressList handles comma-separated lists
	for _, h := range headers {
		if h == "" {
			continue
		}
		addrs, err := mail.ParseAddressList(h)
		if err != nil {
			// fallback: naive split
			parts := strings.FieldsFunc(h, func(r rune) bool {
				return r == ',' || r == '<' || r == '>' || r == ' ' || r == '\n' || r == '\t'
			})
			for _, p := range parts {
				if strings.Contains(p, "@") {
					stringParts := strings.SplitN(p, "@", 2)
					num, repo := splitLocalPart(stringParts[0])
					if isDigits(num) && stringParts[1] == ticketDomain {
						return num, repo
					}
				}
			}
			continue
		}

		for _, a := range addrs {
			if a.Address == "" {
				continue
			}
			parts := strings.SplitN(a.Address, "@", 2)
			if len(parts) != 2 {
				continue
			}
			local := parts[0]
			domain := parts[1]
			num, repo := splitLocalPart(local)
			if isDigits(num) && domain == ticketDomain {
				return num, repo
			}
		}
	}
	return "", ""
}

// receivedFor returns the envelope recipient from the "for" clause of a Received header,
// or "" if it has none. Mail sent only to a Bcc'd address leaves no trace in the message
// headers, but the receiving MTA records the RCPT TO address here; SES, for instance,
// writes "by inbound-smtp.<region>.amazonaws.com with SMTP id <id> for <addr>; <date>".
func receivedFor(received string) string {
	// Trace fields end at the ';' that introduces the date
	if i := strings.IndexByte(received, ';'); i >= 0 {
		received = received[:i]
	}
	fields := strings.Fields(received)
	for i, f := range fields {
		if strings.EqualFold(f, "for") && i+1 < len(fields) {
			return strings.Trim(fields[i+1], "<>,")
		}
	}
	return ""
}

// splitLocalPart splits an email local part on the first '+'.
// "123+myrepo" → ("123", "myrepo"); "123" → ("123", "").
func splitLocalPart(local string) (num, repo string) {
	if i := strings.IndexByte(local, '+'); i >= 0 {
		return local[:i], local[i+1:]
	}
	return local, ""
}

// checks against whitelisted domains to see if sender allowed
func senderDomainAllowed(domain string) bool {
	domain = strings.ToLower(domain)
	for _, d := range whitelistDomains {
		if domain == d || strings.HasSuffix(domain, "."+d) {
			return true
		}
	}
	return false
}

// extractSenderDomain parses the From header and returns the domain (lowercased) or empty string.
func extractSenderDomain(fromHeader string) string {
	if fromHeader == "" {
		return ""
	}
	addr, err := mail.ParseAddress(fromHeader)
	if err != nil {
		// fallback regex-ish parse
		if strings.Contains(fromHeader, "@") {
			parts := strings.Split(fromHeader, "@")
			last := parts[len(parts)-1]
			last = strings.Trim(last, " \t\r\n<>\"")
			return strings.ToLower(last)
		}
		return ""
	}
	parts := strings.SplitN(addr.Address, "@", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.ToLower(parts[1])
}

func passesEmailAuth(h mail.Header) bool {
	v := strings.ToLower(h.Get("Authentication-Results"))
	return strings.Contains(v, "spf=pass") || strings.Contains(v, "dkim=pass")
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
