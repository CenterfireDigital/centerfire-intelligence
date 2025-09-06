package main

import "fmt"

func testNaming() {
    agent := NewAgent()
    
    // Test allocation
    request := map[string]interface{}{
        "action": "allocate_capability",
        "params": map[string]interface{}{
            "domain": "AUTH",
            "purpose": "User authentication system",
        },
    }
    
    result := agent.HandleRequest(request)
    fmt.Printf("Naming Test Result: %+v\n", result)
}