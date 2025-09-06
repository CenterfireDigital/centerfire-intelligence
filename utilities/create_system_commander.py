#!/usr/bin/env python3
"""
Create AGT-SYSTEM-COMMANDER-1 using the HTTP client
"""
from claude_http_client import CenterfireClient

def create_system_commander():
    client = CenterfireClient()
    
    # Allocate agent identity via AGT-NAMING-1 (use allocate_module instead)
    result = client.call_agent('naming', 'allocate_module', {
        'domain': 'SYSTEM-COMMANDER',
        'description': 'Secure system command execution with contract-based authorization'
    })
    
    if result.success:
        print(f"✅ System Commander Identity Allocated:")
        print(f"   Agent ID: {result.data.get('slug')}")  
        print(f"   Directory: {result.data.get('directory')}")
        print(f"   CID: {result.data.get('cid')}")
        
        # Request directory structure via AGT-STRUCT-1
        struct_result = client.call_agent('struct', 'create_structure', {
            'name': result.data.get('slug'),
            'type': 'agent',
            'description': 'Secure system command execution with contract validation'
        })
        
        if struct_result.success:
            print(f"✅ Directory Structure Created")
            return result.data
        else:
            print(f"❌ Structure Creation Failed: {struct_result.error}")
            return None
    else:
        print(f"❌ Agent Allocation Failed: {result.error}")
        return None

if __name__ == "__main__":
    create_system_commander()