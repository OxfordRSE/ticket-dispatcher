package main

import (
	"net/mail"
	"strings"
	"unicode"
)

// extractIssueNumber scans To and Cc headers and returns the first numeric local-part found,
// along with an optional repo suffix extracted from a + tag (e.g. 123+myrepo@domain → "123", "myrepo").
func extractIssueNumber(toHeader, ccHeader string) (issueNumber, repoSuffix string) {
	// Combine headers; ParseAddressList handles comma-separated lists
	headers := []string{toHeader, ccHeader}

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

// splitLocalPart splits an email local part on the first '+'.
// "123+myrepo" → ("123", "myrepo"); "123" → ("123", "").
func splitLocalPart(local string) (num, repo string) {
	if i := strings.IndexByte(local, '+'); i >= 0 {
		return local[:i], local[i+1:]
	}
	return local, ""
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
