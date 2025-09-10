package main

import (
	"fmt"
	"log"
	"net/smtp"
	"strings"
	"time"
)

func main() {

	// Mailpit SMTP server configuration
	smtpHost := "localhost"
	smtpPort := "1025"

	// Test email configurations
	testEmails := []struct {
		from    string
		to      []string
		subject string
		body    string
	}{
		{
			from:    "no-reply@example.com",
			to:      []string{"user1@example.com"},
			subject: "Test Email 1 - Plain Text",
			body:    "This is a plain text test email sent to Mailpit for PostgreSQL testing.",
		},
		{
			from:    "no-reply@example.com",
			to:      []string{"user2@example.com"},
			subject: "Test Email 2 - HTML Content",
			body:    "<html><body><h1>HTML Test Email</h1><p>This is an <b>HTML</b> test email with <i>formatting</i>.</p><p>Sent at: " + time.Now().Format(time.RFC3339) + "</p></body></html>",
		},
		{
			from:    "no-reply@example.com",
			to:      []string{"user3@example.com"},
			subject: "Test Email 3 - Multiple Recipients",
			body:    "This email tests multiple recipients functionality.",
		},
		{
			from:    "no-reply@example.com",
			to:      []string{"user4@example.com"},
			subject: "Test Email 4 - Long Content",
			body:    generateLongContent(),
		},
		{
			from:    "no-reply@example.com",
			to:      []string{"user5@example.com", "user6@example.com", "user7@example.com"},
			subject: "Test Email 5 - Multiple To's",
			body:    "This email tests sending to multiple recipients at once.",
		},
	}

	// Send each test email
	for i, email := range testEmails {
		fmt.Printf("Sending email %d/%d: %s\n", i+1, len(testEmails), email.subject)

		err := sendEmail(smtpHost, smtpPort, email.from, email.to, email.subject, email.body)
		if err != nil {
			log.Printf("Error sending email %d: %v\n", i+1, err)
		} else {
			fmt.Printf(" Email %d sent successfully\n", i+1)
		}

		// Small delay between emails
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("\nAll test emails sent!")

}

func sendEmail(smtpHost, smtpPort, from string, to []string, subject, body string) error {
	// Create the email message
	message := fmt.Sprintf("From: %s\r\n", from)
	message += fmt.Sprintf("To: %s\r\n", strings.Join(to, ","))
	message += fmt.Sprintf("Subject: %s\r\n", subject)
	message += "MIME-Version: 1.0\r\n"

	// Check if body contains HTML
	if len(body) > 0 && body[0:1] == "<" {
		message += "Content-Type: text/html; charset=UTF-8\r\n"
	} else {
		message += "Content-Type: text/plain; charset=UTF-8\r\n"
	}

	message += "\r\n" + body

	// Connect to the SMTP server
	addr := smtpHost + ":" + smtpPort

	conn, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %v", err)
	}
	defer conn.Close()

	// Set sender
	if err := conn.Mail(from); err != nil {
		return fmt.Errorf("failed to set sender: %v", err)
	}

	// Set recipients
	for _, recipient := range to {
		if err := conn.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient %s: %v", recipient, err)
		}
	}

	// Send the email
	writer, err := conn.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %v", err)
	}

	_, err = writer.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to write message: %v", err)
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("failed to close writer: %v", err)
	}

	return nil
}

func generateLongContent() string {
	content := "This is a test email with longer content to test database storage capabilities.\n\n"
	content += "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.\n\n"
	content += "Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.\n\n"
	content += "Sed ut perspiciatis unde omnis iste natus error sit voluptatem accusantium doloremque laudantium, totam rem aperiam.\n\n"
	content += "Nemo enim ipsam voluptatem quia voluptas sit aspernatur aut odit aut fugit, sed quia consequuntur magni dolores eos qui ratione voluptatem sequi nesciunt.\n\n"
	content += "This email was generated at: " + time.Now().Format(time.RFC3339) + "\n"
	content += "Test ID: " + fmt.Sprintf("%d", time.Now().Unix()) + "\n"

	return content
}
