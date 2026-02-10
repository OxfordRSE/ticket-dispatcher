// Extracts body of an email and formats it into format suitable
// for posting to a GitHub issue thread
package main

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"regexp"
	"strings"

	"golang.org/x/net/html/charset"
)

// extractBodyAsMarkdown parses an RFC822 message (net/mail.Message) and returns
// the best-effort Markdown:
//   - prefer text/plain (used as-is, trimmed)
//   - else transform text/html -> markdown
//
// Attachments (Content-Disposition: attachment) are skipped.
func extractBodyAsMarkdown(msg *mail.Message) (string, error) {
	ct := msg.Header.Get("Content-Type")
	cte := msg.Header.Get("Content-Transfer-Encoding")
	mediatype, params, err := mime.ParseMediaType(ct)
	if err != nil {
		// If no/invalid content-type assume simple text/plain
		buf := new(strings.Builder)
		_, _ = io.Copy(buf, msg.Body)
		return strings.TrimSpace(buf.String()), nil
	}

	if strings.HasPrefix(mediatype, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return "", fmt.Errorf("multipart without boundary")
		}
		mr := multipart.NewReader(msg.Body, boundary)
		// Collect first text/plain, else first text/html
		var firstHTML string
		for {
			part, perr := mr.NextPart()
			if perr == io.EOF {
				break
			}
			if perr != nil {
				return "", perr
			}
			// skip attachments
			if disp := strings.ToLower(part.Header.Get("Content-Disposition")); strings.HasPrefix(disp, "attachment") {
				continue
			}
			pct := part.Header.Get("Content-Type")
			pcte := part.Header.Get("Content-Transfer-Encoding")
			ptype, _, _ := mime.ParseMediaType(pct)
			switch ptype {
			case "text/plain":
				b, e := readAndDecodePart(part, pct, pcte)
				if e != nil {
					return "", e
				}
				return strings.TrimSpace(string(b)), nil
			case "text/html":
				b, e := readAndDecodePart(part, pct, pcte)
				if e != nil {
					return "", e
				}
				firstHTML = string(b)
			default:
				return "", errors.New("no text part found")
			}
		}
		// If we saw HTML but no plain text, convert HTML -> markdown
		if firstHTML != "" {
			return htmlToPlain(firstHTML)
		}
		// no useful body found
		return "", nil
	}

	// not multipart: single part message
	bodyBytes, err := readAndDecodePart(msg.Body, ct, cte)
	if err != nil {
		return "", err
	}
	ptype, _, _ := mime.ParseMediaType(ct)
	if ptype == "text/html" {
		return htmlToPlain(string(bodyBytes))
	}
	// default: text/plain or other -> return as text
	return strings.TrimSpace(string(bodyBytes)), nil
}

// readAndDecodePart reads from the raw part Reader (r) and decodes:
//   - Content-Transfer-Encoding: quoted-printable, base64
//   - Charset -> UTF-8 conversion based on Content-Type header
//
// contentType should be the raw Content-Type header value for charset parsing.
func readAndDecodePart(r io.Reader, contentType, cteHeader string) ([]byte, error) {
	// Step 1: decode Content-Transfer-Encoding (cte)
	// cteHeader is typically part.Header.Get("Content-Transfer-Encoding")
	cte := strings.ToLower(strings.TrimSpace(cteHeader))
	var decodedReader io.Reader = r

	switch cte {
	case "quoted-printable":
		decodedReader = quotedprintable.NewReader(r)
	case "base64":
		decodedReader = base64.NewDecoder(base64.StdEncoding, r)
	default:
		// 7bit, 8bit, binary, or absent -> use as-is
		decodedReader = r
	}

	// Step 2: read into a buffer (we'll wrap with charset converter next)
	bufReader := bufio.NewReader(decodedReader)
	rawBytes, err := io.ReadAll(bufReader)
	if err != nil {
		return nil, err
	}

	// Step 3: charset conversion to UTF-8 using contentType
	_, params, _ := mime.ParseMediaType(contentType)
	charsetLabel := strings.ToLower(strings.TrimSpace(params["charset"]))
	if charsetLabel == "" || charsetLabel == "utf-8" || charsetLabel == "us-ascii" {
		return rawBytes, nil
	}

	// Use charset.NewReaderLabel which returns a reader that converts to UTF-8.
	// We create a reader around the raw bytes.
	cr, err := charset.NewReaderLabel(charsetLabel, strings.NewReader(string(rawBytes)))
	if err != nil {
		// If conversion fails, return the raw bytes rather than fail hard.
		return rawBytes, nil
	}
	convBytes, err := io.ReadAll(cr)
	if err != nil {
		return rawBytes, nil
	}
	return convBytes, nil
}

// hideQuotedPart scans plain/markdown text for quoted email context and,
// if found, moves it into a collapsible <details> block.
func hideQuotedPart(md string, removeQuotes bool) string {
	if strings.TrimSpace(md) == "" {
		return md
	}

	pats := []*regexp.Regexp{
		regexp.MustCompile(`(?i)^On .+ wrote:`),             // On ... wrote:
		regexp.MustCompile(`(?i)^\**From:\s*.+@.+`),         // From: someone <email>
		regexp.MustCompile(`(?i)^Sent:\s*`),                 // Sent:
		regexp.MustCompile(`(?i)^\**To:\s*`),                // To:
		regexp.MustCompile(`(?i)^\**Subject:\s*`),           // Subject:
		regexp.MustCompile(`(?i)^-+ ?Original Message ?-+`), // -----Original Message-----
		regexp.MustCompile(`(?i)^Begin forwarded message:`), // Begin forwarded message:
		regexp.MustCompile(`(?m)^--\s*$`),                   // signature separator
	}

	lines := strings.Split(md, "\n")
	n := len(lines)

	// helper to test if current line looks like start of quoted block of > lines
	isQuoteBlock := func(i int) bool {
		// require at least 3 consecutive lines starting with >
		if i >= n {
			return false
		}
		count := 0
		for j := i; j < n && count < 3; j++ {
			if strings.HasPrefix(strings.TrimSpace(lines[j]), ">") {
				count++
			} else if strings.TrimSpace(lines[j]) == "" {
				// allow blank lines in between quoted blocks
				continue
			} else {
				break
			}
		}
		return count >= 3
	}

	// Find split index
	split := -1
	for i, ln := range lines {
		trim := strings.TrimSpace(ln)
		if trim == "" {
			continue
		}
		if isQuoteBlock(i) {
			split = i
			break
		}
		for _, re := range pats {
			if re.MatchString(trim) {
				split = i
				break
			}
		}
		if split != -1 {
			break
		}
	}

	if split == -1 {
		return md
	}

	visible := strings.TrimRight(strings.Join(lines[:split], "\n"), "\n")
	quoted := strings.TrimLeft(strings.Join(lines[split:], "\n"), "\n")

	// Wrap the quoted part in details
	details := "<details>\n<summary>Show quoted email</summary>\n\n" +
		strings.TrimRight(quoted, "\n") + "\n\n</details>"

	// If visible body is empty (e.g., purely quoted), we still show a short header
	if strings.TrimSpace(visible) == "" {
		// show a short intro and then details
		return details
	}

	// Remove quotes entirely as message threads can get long
	// if removeQuotes = false, then display the context as a <details>/<summary> enclosure
	if removeQuotes {
		// remove quotes entirely
		return visible + "\n"
	} else {
		// Otherwise show visible then details
		return visible + "\n\n" + details
	}
}
