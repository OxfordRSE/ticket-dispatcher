// Parse an email and print out metadata if a ticket number is detected
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/mail"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	ticketDomain    string
	githubProject   string
	whitelistDomain string
	s3Client        *s3.Client
)

func loadConfig() {
	// read env vars
	ticketDomain = os.Getenv("TICKET_DISPATCHER_DOMAIN")
	whitelistDomain = os.Getenv("WHITELIST_DOMAIN")
	githubProject = os.Getenv("GITHUB_PROJECT")

	if ticketDomain == "" {
		log.Fatalf("TICKET_DISPATCHER_DOMAIN is not set, example: issues.example.com")
	}

	if whitelistDomain == "" {
		log.Fatalf("WHITELIST_DOMAIN is unset, set to a domain that is allowed to send emails")
	}

	if githubProject == "" {
		fmt.Println("GITHUB_PROJECT not set, will not comment on issues, only writing metadata")
	}
}

func initS3() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("failed to load aws config: %v", err)
	}
	s3Client = s3.NewFromConfig(cfg)
}

func handler(ctx context.Context, s3Event events.S3Event) error {
	quoteConfig := os.Getenv("SHOW_QUOTED_TEXT")
	removeQuotes := quoteConfig == ""
	for _, rec := range s3Event.Records {
		bucket := rec.S3.Bucket.Name
		key := rec.S3.Object.Key
		log.Printf("processing s3://%s/%s", bucket, key)

		objOut, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: &bucket,
			Key:    &key,
		})
		if err != nil {
			log.Printf("failed get object: %v", err)
			continue
		}
		raw, err := io.ReadAll(objOut.Body)
		objOut.Body.Close()
		if err != nil {
			log.Printf("failed read object body: %v", err)
			continue
		}
		msg, err := mail.ReadMessage(bytes.NewReader(raw))

		msgId := msg.Header.Get("Message-ID")
		toHeader := msg.Header.Get("To")
		ccHeader := msg.Header.Get("Cc")
		fromHeader := msg.Header.Get("From")
		subject := msg.Header.Get("Subject")
		auth := msg.Header.Get("Authentication-Results")

		issue := extractIssueNumber(toHeader, ccHeader)
		senderDomain := extractSenderDomain(fromHeader)

		if !strings.Contains(auth, "spf=pass") && !strings.Contains(auth, "dkim=pass") {
			log.Fatalf("%s authentication failure, possibly spoofed", msgId)
		}
		if !strings.HasSuffix(senderDomain, whitelistDomain) {
			log.Fatalf("sender does not have a '%s' email address", whitelistDomain)
		}
		if issue == "" {
			log.Fatalf("no issue number found in To: or Cc:")
		}
		log.Printf("%s | From: %s; To: %s; Subject: %s\n", msgId, fromHeader, toHeader, subject)
		body, err := extractBodyAsMarkdown(msg)
		if err != nil {
			log.Fatalf("error in extracting message body")
		} else {
			header := fmt.Sprintf("From: %s\n\n", fromHeader)
			err := postIssueComment(issue, msgId, header+hideQuotedPart(body, removeQuotes))
			if err != nil {
				log.Printf("postIssueComment err=%v", err)
			}
		}
		os.Exit(0)
	}
	return nil
}

func main() {
	loadConfig()
	initS3()
	lambda.Start(handler)
}

func debugMain() {
	if len(os.Args) < 2 {
		fmt.Println("usage: ./ticket-dispatcher <filename>")
		return
	}
	file, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer file.Close()

	msg, err := mail.ReadMessage(file)
	if err != nil {
		log.Fatalf("error parsing email: %v", err)
	}
	body, err := extractBodyAsMarkdown(msg)
	if err != nil {
		log.Fatalf("error extracting body: %v", err)
	} else {
		fmt.Println(body)
	}
}
