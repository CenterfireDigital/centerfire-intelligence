#!/usr/bin/env python3
"""
System Commander Client - Interface to AGT-SYSTEM-COMMANDER-1 via HTTP Gateway
Allows Claude Code to execute system commands through the secure agent architecture.
"""

import json
import time
from claude_http_client import CenterfireClient, AgentResponse
from typing import Optional

class SystemCommanderClient:
    """
    Client for AGT-SYSTEM-COMMANDER-1 with TTY and session support
    """
    
    def __init__(self, client_id: str = "claude_code"):
        self.client = CenterfireClient()
        self.client_id = client_id
        
    def execute_command(self, 
                       command: str, 
                       tty: bool = False, 
                       session: str = None,
                       timeout: int = 30) -> dict:
        """
        Execute a system command via AGT-SYSTEM-COMMANDER-1
        
        Args:
            command: Command to execute
            tty: Use TTY mode with tmux session
            session: Specific tmux session name (optional)
            timeout: Command timeout
            
        Returns:
            dict with success, output, error, exit_code
        """
        request_id = f"syscmd_{int(time.time() * 1000)}"
        
        params = {
            'command': command,
            'client_id': self.client_id,
            'request_id': request_id,
            'tty': tty
        }
        
        if session:
            params['session'] = session
            
        # Call system commander via HTTP gateway
        result = self.client.call_agent('system', 'execute_command', params)
        
        if result.success:
            return {
                'success': True,
                'output': result.data.get('output', ''),
                'error': result.data.get('error', ''),
                'exit_code': result.data.get('exit_code', 0),
                'session_name': result.data.get('session_name'),
                'transport': result.transport_used
            }
        else:
            return {
                'success': False,
                'output': '',
                'error': result.error or 'System commander request failed',
                'exit_code': -1,
                'transport': result.transport_used
            }
    
    def bash(self, command: str, tty: bool = False) -> dict:
        """Convenience method for bash command execution"""
        return self.execute_command(command, tty=tty)
    
    def interactive_session(self, session_name: str = None) -> str:
        """Start or get interactive TTY session"""
        if not session_name:
            session_name = f"claude_session_{int(time.time())}"
            
        # Initialize session with a simple command
        result = self.execute_command("echo 'Session initialized'", tty=True, session=session_name)
        
        if result['success']:
            return result.get('session_name', session_name)
        else:
            raise Exception(f"Failed to create session: {result['error']}")

def run_command(command: str, tty: bool = False) -> dict:
    """Quick function to run a command via system commander"""
    client = SystemCommanderClient()
    return client.execute_command(command, tty=tty)

if __name__ == "__main__":
    # Demo usage
    print("🖥️  System Commander Client Demo")
    print("=" * 40)
    
    client = SystemCommanderClient()
    
    # Test basic command
    print("\n📋 Testing basic command (ls -la):")
    result = client.bash("ls -la")
    print(f"Success: {result['success']}")
    print(f"Exit Code: {result['exit_code']}")
    print(f"Output:\n{result['output']}")
    
    # Test TTY command  
    print("\n🖥️  Testing TTY command (pwd):")
    result = client.bash("pwd", tty=True)
    print(f"Success: {result['success']}")
    print(f"Session: {result.get('session_name')}")
    print(f"Output:\n{result['output']}")
    
    print(f"\n✅ System Commander Client operational via {result.get('transport', 'unknown')} transport")