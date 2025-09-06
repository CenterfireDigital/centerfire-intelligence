#!/usr/bin/env python3

import json
import time
import uuid
from claude_http_client import CenterfireClient

def test_system_commander_general():
    """Test the newly promoted System Commander General with orchestration capabilities"""
    
    print("🎖️  SYSTEM COMMANDER GENERAL - ORCHESTRATION TEST")
    print("=" * 60)
    
    client = CenterfireClient()
    
    # Test 1: Basic shell pool functionality
    print("\n1. Testing Basic Shell Pool Management")
    print("-" * 40)
    
    result = client.call_agent("system", "execute_command", {
        "command": "pwd && echo 'Shell Alpha ready for orders'",
        "mode": "tmux",
        "purpose": "alpha_unit"
    })
    
    if result.success:
        print(f"✅ Shell Alpha established: {result.data.get('shell_id', 'Unknown')}")
        alpha_shell = result.data.get('shell_id')
    else:
        print(f"❌ Failed to establish Shell Alpha: {result.error}")
        return False
        
    # Test 2: Status command - General's situation report
    print("\n2. General's Situation Report")
    print("-" * 40)
    
    status_result = client.call_agent("system", "execute_command", {
        "command": "__status__"
    })
    
    if status_result.success:
        print("📊 Active Shell Status:")
        print(status_result.data.get('output', 'No status available'))
    else:
        print(f"❌ Status report failed: {status_result.error}")
    
    # Test 3: Parallel Operations - Multiple theaters of operation
    print("\n3. Testing Parallel Multi-Theater Operations")
    print("-" * 40)
    
    parallel_commands = [
        {
            "command": "echo 'Bravo Unit: Reconnaissance mission' && sleep 2 && echo 'Bravo: Target acquired'",
            "purpose": "bravo_recon"
        },
        {
            "command": "echo 'Charlie Unit: Supply line check' && sleep 1 && echo 'Charlie: Supplies secured'",
            "purpose": "charlie_supply"
        },
        {
            "command": "echo 'Delta Unit: Communications test' && sleep 3 && echo 'Delta: All frequencies clear'",
            "purpose": "delta_comms"
        }
    ]
    
    start_time = time.time()
    parallel_result = client.call_agent("system", "execute_command", {
        "mode": "parallel",
        "parallel": parallel_commands
    })
    execution_time = time.time() - start_time
    
    if parallel_result.success:
        print(f"✅ Parallel operations completed in {execution_time:.2f}s")
        print(f"📊 Active shells after operation: {parallel_result.data.get('active_shells', 0)}")
        
        # Show results from each unit
        results = parallel_result.data.get('results', [])
        for i, result in enumerate(results):
            unit_name = ['Bravo', 'Charlie', 'Delta'][i]
            status = '✅' if result['success'] else '❌'
            duration = result['duration_ms']
            print(f"  {status} {unit_name} Unit: {duration}ms - Shell {result['shell_id']}")
            
        print("\n📋 Combined Operations Report:")
        output_lines = parallel_result.data.get('output', '').split('\n')
        for line in output_lines:
            if line.strip():
                print(f"    {line}")
                
    else:
        print(f"❌ Parallel operations failed: {parallel_result.error}")
        return False
    
    # Test 4: Use specific shell (Alpha unit gets new orders)
    print("\n4. Issuing Orders to Specific Unit")
    print("-" * 40)
    
    if alpha_shell:
        specific_result = client.call_agent("system", "execute_command", {
            "command": "echo 'Alpha Unit receiving new orders...' && ls -la",
            "mode": "tmux",
            "shell_id": alpha_shell
        })
        
        if specific_result.success:
            print(f"✅ Alpha Unit executed orders successfully")
            print(f"   Shell ID: {specific_result.data.get('shell_id')}")
        else:
            print(f"❌ Alpha Unit failed to execute: {specific_result.error}")
    
    # Final status report
    print("\n5. Final Situation Report")
    print("-" * 40)
    
    final_status = client.call_agent("system", "execute_command", {
        "command": "__status__"
    })
    
    if final_status.success:
        print("🎖️  GENERAL'S FINAL REPORT:")
        print(final_status.data.get('output', 'No status available'))
    
    print("\n🎖️  SYSTEM COMMANDER GENERAL TEST COMPLETE")
    print("=" * 60)
    return True

if __name__ == "__main__":
    try:
        success = test_system_commander_general()
        if success:
            print("🎯 All tests passed! The General is ready for deployment.")
        else:
            print("⚠️  Some tests failed. Check logs for issues.")
    except Exception as e:
        print(f"💥 Test suite failed: {e}")