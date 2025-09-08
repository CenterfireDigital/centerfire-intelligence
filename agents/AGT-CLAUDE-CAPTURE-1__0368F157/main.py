#!/usr/bin/env python3
"""
AGT-CLAUDE-CAPTURE-1: Claude Code Session Capture Agent

Captures Claude Code sessions and streams them to Redis for consumption
by the conversation streaming pipeline. Designed as a development tool
to capture architectural discussions that would otherwise be lost.

Usage:
    python main.py [--mode=hook|standalone]
"""

import asyncio
import json
import logging
import os
import sys
import time
import uuid
from datetime import datetime, timezone
from typing import Dict, Any, Optional, List
from dataclasses import dataclass, asdict
from pathlib import Path

import redis
import psutil
import aiofiles
from watchdog.observers import Observer
from watchdog.events import FileSystemEventHandler

@dataclass
class ConversationTurn:
    """Single conversation turn (user input + assistant response)"""
    session_id: str
    agent_id: str
    timestamp: str
    turn_count: int
    user: str
    assistant: str

@dataclass
class SessionEvent:
    """Session lifecycle event"""
    event_type: str  # start, message, heartbeat, end
    session_id: str
    timestamp: str
    data: Optional[Dict[str, Any]] = None

class ClaudeCaptureAgent:
    """Claude Code session capture and streaming agent"""
    
    def __init__(self):
        self.agent_id = "AGT-CLAUDE-CAPTURE-1"
        self.cid = "cid:centerfire:agent:0368F157" 
        self.redis_client = None
        self.active_sessions: Dict[str, Dict] = {}
        self.conversation_buffer: List[str] = []
        self.session_id = None
        
        # Lazy disk buffering for Redis failures
        self.redis_healthy = True
        self.disk_buffer_active = False
        self.buffer_file_path = Path("./redis_failover_buffer.jsonl")
        self.last_redis_check = 0
        self.redis_check_interval = 30  # Check Redis health every 30 seconds
        
        # Redis channels and streams
        self.request_channel = "agent.claude-capture.request"
        self.response_channel = "agent.claude-capture.response"
        self.conversation_stream = "centerfire:semantic:conversations"
        self.session_stream = "claude:sessions:stream"
        
        # Setup logging
        logging.basicConfig(
            level=logging.INFO,
            format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
        )
        self.logger = logging.getLogger(self.agent_id)
        
    async def start(self):
        """Start the Claude capture agent"""
        self.logger.info(f"🚀 Starting {self.agent_id}...")
        
        # Connect to Redis
        await self._connect_redis()
        
        # Register with AGT-MANAGER-1
        await self._register_with_manager()
        
        # Start background tasks
        tasks = [
            self._listen_for_requests(),
            self._monitor_claude_processes(),
            self._capture_current_session()
        ]
        
        self.logger.info(f"✅ {self.agent_id} ready for Claude Code session capture")
        
        # Run all tasks concurrently
        try:
            await asyncio.gather(*tasks)
        except KeyboardInterrupt:
            self.logger.info(f"🛑 {self.agent_id} shutting down...")
            await self._unregister_with_manager()
    
    async def _connect_redis(self):
        """Connect to Redis"""
        try:
            self.redis_client = redis.Redis(
                host="localhost", 
                port=6380, 
                decode_responses=True
            )
            # Test connection
            self.redis_client.ping()
            self.logger.info("📡 Connected to Redis successfully")
        except Exception as e:
            self.logger.error(f"❌ Failed to connect to Redis: {e}")
            raise
    
    async def _register_with_manager(self):
        """Register with AGT-MANAGER-1"""
        session_data = {
            "pid": os.getpid(),
            "agent_type": "persistent",
            "capabilities": ["session_capture", "conversation_streaming", "claude_monitoring"],
            "channels": [self.request_channel],
            "language": "python"
        }
        
        registration_data = {
            "request_type": "register_running",
            "agent_name": self.agent_id,
            "session_data": session_data
        }
        
        try:
            self.redis_client.publish("centerfire:agent:manager", json.dumps(registration_data))
            self.logger.info(f"📝 Registered with AGT-MANAGER-1 (PID: {os.getpid()})")
            
            # Start heartbeat
            await self._start_heartbeat()
        except Exception as e:
            self.logger.error(f"❌ Failed to register with manager: {e}")
    
    async def _start_heartbeat(self):
        """Start periodic heartbeat to AGT-MANAGER-1"""
        async def heartbeat_loop():
            while True:
                try:
                    await asyncio.sleep(30)  # Heartbeat every 30 seconds
                    
                    heartbeat_data = {
                        "request_type": "heartbeat",
                        "agent_name": self.agent_id
                    }
                    
                    self.redis_client.publish("centerfire:agent:manager", json.dumps(heartbeat_data))
                    
                except Exception as e:
                    self.logger.error(f"❌ Heartbeat error: {e}")
                    await asyncio.sleep(60)
        
        # Start heartbeat in the background
        asyncio.create_task(heartbeat_loop())
    
    async def _unregister_with_manager(self):
        """Unregister from AGT-MANAGER-1"""
        unregister_data = {
            "request_type": "unregister_running",
            "agent_name": self.agent_id
        }
        
        try:
            self.redis_client.publish("centerfire:agent:manager", json.dumps(unregister_data))
            self.logger.info("📋 Unregistered from AGT-MANAGER-1")
        except Exception as e:
            self.logger.error(f"❌ Failed to unregister: {e}")
    
    async def _listen_for_requests(self):
        """Listen for Redis pub/sub requests"""
        pubsub = self.redis_client.pubsub()
        pubsub.subscribe(self.request_channel)
        
        try:
            for message in pubsub.listen():
                if message['type'] == 'message':
                    await self._handle_request(message['data'])
        except Exception as e:
            self.logger.error(f"❌ Error in request listener: {e}")
        finally:
            pubsub.close()
    
    async def _handle_request(self, payload: str):
        """Handle incoming Redis requests"""
        try:
            request = json.loads(payload)
            action = request.get('action')
            request_id = request.get('request_id', str(uuid.uuid4()))
            
            self.logger.info(f"📥 Processing request: {action}")
            
            response = {"request_id": request_id, "success": False}
            
            if action == "capture_session":
                response = await self._handle_capture_session(request)
            elif action == "stream_conversation":  
                response = await self._handle_stream_conversation(request)
            elif action == "get_status":
                response = await self._handle_get_status(request)
            else:
                response["error"] = f"Unknown action: {action}"
            
            # Publish response
            self.redis_client.publish(self.response_channel, json.dumps(response))
            
        except Exception as e:
            self.logger.error(f"❌ Error handling request: {e}")
    
    async def _capture_current_session(self):
        """Capture the current Claude Code session"""
        # This is the critical function - capture THIS conversation
        self.session_id = f"CLAUDE-CODE-SESSION_{int(time.time())}"
        
        self.logger.info(f"🎯 Starting capture of current Claude Code session: {self.session_id}")
        
        # Stream session start event
        await self._stream_session_event("session_started", {
            "session_type": "claude_code", 
            "capture_method": "development_agent",
            "note": "Capturing architectural discussion session"
        })
        
        # For now, we'll capture this conversation by monitoring the process
        # In production, this would hook into Claude Code's actual conversation flow
        await self._monitor_conversation_flow()
    
    async def _monitor_conversation_flow(self):
        """Monitor and capture conversation flow"""
        conversation_data = {
            "session_id": self.session_id,
            "agent_id": "CLAUDE-CODE",  # Since we're Claude Code
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "turn_count": 1,
            "user": "Human architectural discussion about Redis streams, Weaviate context, and semantic ticketing",
            "assistant": "Claude Code implementing conversation streaming pipeline with AGT-CONTEXT-1 integration"
        }
        
        # Stream this conversation
        await self._stream_conversation(conversation_data)
        
        # Continue monitoring (this is a simplified version)
        while True:
            await asyncio.sleep(60)  # Check every minute
            await self._stream_session_event("heartbeat", {
                "active_conversations": len(self.active_sessions),
                "buffer_size": len(self.conversation_buffer)
            })
    
    async def _stream_conversation(self, conversation_data: Dict[str, Any]):
        """Stream conversation data to Redis with lazy disk buffering failover"""
        stream_data = {
            "data": json.dumps(conversation_data)
        }
        
        # Check Redis health periodically
        await self._check_redis_health()
        
        if self.redis_healthy:
            try:
                # Normal path - direct to Redis
                self.redis_client.xadd(self.conversation_stream, stream_data)
                self.logger.info(f"💾 Streamed conversation to {self.conversation_stream}")
                
                # If we were in failover mode, try to replay buffer
                if self.disk_buffer_active:
                    await self._replay_disk_buffer()
                
            except Exception as e:
                self.logger.warning(f"⚠️ Redis write failed, activating disk buffer: {e}")
                self.redis_healthy = False
                await self._activate_disk_buffer()
                await self._write_to_disk(stream_data)
        else:
            # Failover path - disk buffer
            self.logger.debug(f"📝 Writing to disk buffer (Redis unhealthy)")
            await self._write_to_disk(stream_data)
    
    async def _stream_session_event(self, event_type: str, data: Optional[Dict] = None):
        """Stream session lifecycle events"""
        event = SessionEvent(
            event_type=event_type,
            session_id=self.session_id,
            timestamp=datetime.now(timezone.utc).isoformat(),
            data=data
        )
        
        try:
            self.redis_client.xadd(self.session_stream, asdict(event))
            self.logger.info(f"📊 Streamed {event_type} event for session {self.session_id}")
        except Exception as e:
            self.logger.error(f"❌ Failed to stream session event: {e}")
    
    async def _monitor_claude_processes(self):
        """Monitor Claude Code processes"""
        while True:
            try:
                # Look for Claude Code processes
                claude_processes = []
                for proc in psutil.process_iter(['pid', 'name', 'cmdline']):
                    try:
                        if 'claude' in proc.info['name'].lower() or \
                           any('claude' in arg.lower() for arg in (proc.info['cmdline'] or [])):
                            claude_processes.append(proc.info)
                    except (psutil.NoSuchProcess, psutil.AccessDenied):
                        continue
                
                if claude_processes:
                    self.logger.debug(f"🔍 Found {len(claude_processes)} Claude-related processes")
                
                await asyncio.sleep(30)  # Check every 30 seconds
                
            except Exception as e:
                self.logger.error(f"❌ Error monitoring processes: {e}")
                await asyncio.sleep(60)
    
    
    async def _handle_capture_session(self, request: Dict[str, Any]) -> Dict[str, Any]:
        """Handle session capture request"""
        session_id = request.get('session_id', f"session_{int(time.time())}")
        
        self.active_sessions[session_id] = {
            "started": time.time(),
            "status": "active",
            "turns": 0
        }
        
        await self._stream_session_event("session_started", {"session_id": session_id})
        
        return {
            "success": True,
            "session_id": session_id,
            "message": "Session capture started"
        }
    
    async def _handle_stream_conversation(self, request: Dict[str, Any]) -> Dict[str, Any]:
        """Handle conversation streaming request"""
        conversation_data = request.get('conversation_data', {})
        
        if not conversation_data:
            return {"success": False, "error": "No conversation data provided"}
        
        await self._stream_conversation(conversation_data)
        
        return {
            "success": True,
            "message": "Conversation streamed successfully"
        }
    
    async def _handle_get_status(self, request: Dict[str, Any]) -> Dict[str, Any]:
        """Handle status request"""
        return {
            "success": True,
            "agent_id": self.agent_id,
            "cid": self.cid,
            "active_sessions": len(self.active_sessions),
            "buffer_size": len(self.conversation_buffer),
            "current_session": self.session_id,
            "uptime": time.time() - (hasattr(self, '_start_time') and self._start_time or time.time())
        }
    
    async def _check_redis_health(self):
        """Check Redis health periodically"""
        current_time = time.time()
        if current_time - self.last_redis_check < self.redis_check_interval:
            return
            
        self.last_redis_check = current_time
        
        try:
            # Quick ping to test Redis
            self.redis_client.ping()
            if not self.redis_healthy:
                self.logger.info("✅ Redis recovered - switching back from disk buffer")
                self.redis_healthy = True
                # Replay disk buffer will happen in next write
                
        except Exception as e:
            if self.redis_healthy:
                self.logger.warning(f"⚠️ Redis health check failed: {e}")
                self.redis_healthy = False
    
    async def _activate_disk_buffer(self):
        """Activate disk buffering mode"""
        if not self.disk_buffer_active:
            self.disk_buffer_active = True
            self.logger.info(f"🆘 Activating disk buffer: {self.buffer_file_path}")
    
    async def _write_to_disk(self, stream_data: Dict[str, Any]):
        """Write conversation data to disk buffer"""
        try:
            buffer_entry = {
                "timestamp": datetime.now(timezone.utc).isoformat(),
                "stream": self.conversation_stream,
                "data": stream_data
            }
            
            async with aiofiles.open(self.buffer_file_path, 'a') as f:
                await f.write(json.dumps(buffer_entry) + '\n')
                
        except Exception as e:
            self.logger.error(f"❌ Failed to write to disk buffer: {e}")
    
    async def _replay_disk_buffer(self):
        """Replay buffered conversations to Redis when it recovers"""
        if not self.buffer_file_path.exists():
            return
            
        self.logger.info(f"🔄 Replaying disk buffer to Redis...")
        
        try:
            replayed_count = 0
            
            async with aiofiles.open(self.buffer_file_path, 'r') as f:
                async for line in f:
                    if line.strip():
                        buffer_entry = json.loads(line)
                        stream_data = buffer_entry['data']
                        
                        # Replay to Redis
                        self.redis_client.xadd(self.conversation_stream, stream_data)
                        replayed_count += 1
            
            # Clear the buffer file after successful replay
            self.buffer_file_path.unlink()
            self.disk_buffer_active = False
            
            self.logger.info(f"✅ Replayed {replayed_count} buffered conversations to Redis")
            
        except Exception as e:
            self.logger.error(f"❌ Failed to replay disk buffer: {e}")

async def main():
    """Main entry point"""
    agent = ClaudeCaptureAgent()
    agent._start_time = time.time()
    
    try:
        await agent.start()
    except KeyboardInterrupt:
        print(f"\n🛑 {agent.agent_id} shutting down...")
    except Exception as e:
        print(f"❌ Fatal error: {e}")
        return 1
    
    return 0

if __name__ == "__main__":
    sys.exit(asyncio.run(main()))