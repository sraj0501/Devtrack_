package main

import (
	"fmt"
	"os"
	"os/exec"
)

// handlePreviewReport previews today's email report
func (cli *CLI) handlePreviewReport() error {
	if err := requiresManagedMode("preview-report"); err != nil {
		return err
	}
	date := ""
	if len(os.Args) > 2 {
		date = os.Args[2]
	}

	fmt.Println("📊 Generating daily report preview...")
	fmt.Println()

	scriptPath, err := GetEmailReporterPath()
	if err != nil {
		return fmt.Errorf("cannot generate report: %w", err)
	}
	config, _ := LoadEnvConfig()
	projectRoot := ""
	if config != nil {
		projectRoot = config.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = os.Getenv("PROJECT_ROOT")
	}

	args := []string{"run", "--directory", projectRoot, "python", scriptPath, "preview"}
	if date != "" {
		args = append(args, date)
	}

	cmd := exec.Command("uv", args...)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Failed to generate report: %v\n", err)
		return err
	}

	return nil
}

// handleSendReport sends email report
func (cli *CLI) handleSendReport() error {
	if err := requiresManagedMode("send-report"); err != nil {
		return err
	}
	if len(os.Args) < 3 {
		fmt.Println("❌ Usage: devtrack send-report <email> [date]")
		return fmt.Errorf("missing email argument")
	}

	email := os.Args[2]
	date := ""
	if len(os.Args) > 3 {
		date = os.Args[3]
	}

	fmt.Printf("📧 Sending report to %s...\n", email)
	fmt.Println()

	scriptPath, err := GetEmailReporterPath()
	if err != nil {
		return fmt.Errorf("cannot send report: %w", err)
	}
	config, _ := LoadEnvConfig()
	projectRoot := ""
	if config != nil {
		projectRoot = config.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = os.Getenv("PROJECT_ROOT")
	}

	args := []string{"run", "--directory", projectRoot, "python", scriptPath, "send", email}
	if date != "" {
		args = append(args, date)
	}

	cmd := exec.Command("uv", args...)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Failed to send report: %v\n", err)
		return err
	}

	return nil
}

// handleSaveReport saves report to file
func (cli *CLI) handleSaveReport() error {
	if err := requiresManagedMode("save-report"); err != nil {
		return err
	}
	date := ""
	if len(os.Args) > 2 {
		date = os.Args[2]
	}

	fmt.Println("💾 Saving report to file...")
	fmt.Println()

	scriptPath, err := GetEmailReporterPath()
	if err != nil {
		return fmt.Errorf("cannot save report: %w", err)
	}
	config, _ := LoadEnvConfig()
	projectRoot := ""
	if config != nil {
		projectRoot = config.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = os.Getenv("PROJECT_ROOT")
	}

	args := []string{"run", "--directory", projectRoot, "python", scriptPath, "save"}
	if date != "" {
		args = append(args, date)
	}

	cmd := exec.Command("uv", args...)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Failed to save report: %v\n", err)
		return err
	}

	return nil
}
