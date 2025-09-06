#!/usr/bin/env python3
"""
Test System Commander directly via Redis (not through HTTP gateway)
"""

import redis
import json
import time
import uuid

def test_system_commander():
    r = redis.Redis(host='localhost', port=6380, decode_responses=True)
    
    request_id = f"test_{int(time.time() * 1000)}_{uuid.uuid4().hex[:8]}"
    
    # Create subscription first
    pubsub = r.pubsub()
    pubsub.subscribe('agent.system.response')
    
    # Send command request
    request = {
        'command': 'ls -la',
        'client_id': 'claude_code',
        'request_id': request_id,
        'tty': False
    }
    
    print(f"🔍 Sending command: {request['command']}")
    print(f"   Request ID: {request_id}")
    
    # Publish request
    r.publish('agent.system.request', json.dumps(request))
    
    # Wait for response
    timeout = time.time() + 10  # 10 second timeout
    
    for message in pubsub.listen():
        if message['type'] == 'message':
            try:
                response = json.loads(message['data'])
                if response.get('request_id') == request_id:
                    print(f"✅ Got response:")
                    print(f"   Success: {response.get('success')}")
                    print(f"   Exit Code: {response.get('exit_code')}")
                    print(f"   Output:\n{response.get('output')}")
                    if response.get('error'):
                        print(f"   Error: {response.get('error')}")
                    pubsub.unsubscribe()
                    pubsub.close()
                    return response
            except json.JSONDecodeError:
                continue
        
        if time.time() > timeout:
            print("❌ Timeout waiting for response")
            pubsub.unsubscribe()
            pubsub.close()
            return None

if __name__ == "__main__":
    test_system_commander()