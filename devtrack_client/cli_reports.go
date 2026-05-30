package main

import (
	"fmt"
	"os"
)

// handlePreviewReport previews today's email report via the server.
func (cli *CLI) handlePreviewReport() error {
	date := ""
	if len(os.Args) > 2 {
		date = os.Args[2]
	}

	fmt.Println("Generating daily report preview...")
	fmt.Println()

	client := NewHTTPTriggerClient()
	output, err := client.ReportPreview(date)
	if err != nil {
		return fmt.Errorf("preview-report: %w (is the server running?)", err)
	}
	fmt.Print(output)
	return nil
}

// handleSendReport emails the report via the server.
func (cli *CLI) handleSendReport() error {
	if len(os.Args) < 3 {
		fmt.Println("Usage: devtrack send-report <email> [date]")
		return fmt.Errorf("missing email argument")
	}

	email := os.Args[2]
	date := ""
	if len(os.Args) > 3 {
		date = os.Args[3]
	}

	fmt.Printf("Sending report to %s...\n", email)
	fmt.Println()

	client := NewHTTPTriggerClient()
	output, err := client.ReportSend(email, date)
	if err != nil {
		return fmt.Errorf("send-report: %w (is the server running?)", err)
	}
	fmt.Print(output)
	return nil
}

// handleSaveReport saves the report to disk via the server.
func (cli *CLI) handleSaveReport() error {
	date := ""
	if len(os.Args) > 2 {
		date = os.Args[2]
	}

	fmt.Println("Saving report to file...")
	fmt.Println()

	client := NewHTTPTriggerClient()
	output, err := client.ReportSave(date)
	if err != nil {
		return fmt.Errorf("save-report: %w (is the server running?)", err)
	}
	fmt.Print(output)
	return nil
}
