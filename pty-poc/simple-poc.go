// Simple PTY Proxy - Proof of Concept
// Demonstrates command interception without raw terminal mode

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/creack/pty"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run simple-poc.go <command> [args...]")
		fmt.Println("Example: go run simple-poc.go echo 'Hello World'")
		os.Exit(1)
	}

	// Create command to execute
	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	
	// Start command with PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		panic(err)
	}
	defer ptmx.Close()

	fmt.Printf("[PTY PROXY]: Starting command: %s\n", strings.Join(os.Args[1:], " "))

	// Read output from PTY with interception capability
	scanner := bufio.NewScanner(ptmx)
	for scanner.Scan() {
		line := scanner.Text()
		
		// INTERCEPTION POINT: This is where orchestrator would
		// capture tool calls, modify context, route to different LLMs, etc.
		if strings.Contains(line, "{") {
			fmt.Printf("[INTERCEPTED]: Potential JSON detected: %s\n", line)
		}
		if strings.Contains(line, "claude") || strings.Contains(line, "Claude") {
			fmt.Printf("[INTERCEPTED]: Claude reference detected: %s\n", line)
		}
		
		// Pass through to stdout (this is where real output would go)
		fmt.Printf("[OUTPUT]: %s\n", line)
	}

	// Wait for command to finish
	err = cmd.Wait()
	if err != nil {
		fmt.Printf("[PTY PROXY]: Command exited with error: %v\n", err)
	} else {
		fmt.Printf("[PTY PROXY]: Command completed successfully\n")
	}
}