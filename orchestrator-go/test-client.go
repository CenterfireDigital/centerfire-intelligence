// Test client to simulate agent connecting to orchestrator socket
package main

import (
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	// Connect to the naming agent socket
	conn, err := net.Dial("unix", "/tmp/orchestrator-naming.sock")
	if err != nil {
		log.Fatalf("Failed to connect to orchestrator socket: %v", err)
	}
	defer conn.Close()

	fmt.Println("✅ Connected to orchestrator naming socket")

	// Send test message
	testMessage := `{"id":"socket-test-001","action":"test","data":{"message":"Hello from socket client"}}`
	_, err = conn.Write([]byte(testMessage))
	if err != nil {
		log.Fatalf("Failed to write to socket: %v", err)
	}

	fmt.Println("📤 Sent test message to orchestrator")

	// Read response (if any)
	buffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buffer)
	if err != nil {
		fmt.Printf("⏰ Timeout or error reading response: %v\n", err)
	} else {
		fmt.Printf("📥 Received response: %s\n", string(buffer[:n]))
	}

	// Keep connection alive for a moment to simulate agent behavior
	time.Sleep(2 * time.Second)
	fmt.Println("🔌 Disconnecting from orchestrator")
}