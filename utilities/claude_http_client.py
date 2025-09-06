#!/usr/bin/env python3
"""
Claude Code HTTP Client with Redis Fallback

Provides a unified interface for Claude Code to communicate with Centerfire agents
via HTTP Gateway with automatic fallback to direct Redis pub/sub if needed.

Usage:
    from claude_http_client import CenterfireClient
    
    client = CenterfireClient()
    result = client.call_agent('naming', 'allocate_capability', {
        'domain': 'TEST',
        'description': 'Test capability'
    })
"""

import json
import time
import uuid
import requests
import redis
import logging
from typing import Dict, Any, Optional, Union
from dataclasses import dataclass
from enum import Enum

class TransportMode(Enum):
    HTTP_ONLY = "http_only"
    REDIS_ONLY = "redis_only"
    HTTP_WITH_FALLBACK = "http_with_fallback"

@dataclass
class AgentResponse:
    success: bool
    data: Optional[Dict[str, Any]] = None
    error: Optional[str] = None
    request_id: Optional[str] = None
    transport_used: Optional[str] = None
    response_time_ms: Optional[int] = None

class CenterfireClient:
    """
    Centerfire agent communication client with HTTP Gateway and Redis fallback
    """
    
    def __init__(self, 
                 manager_host: str = "localhost",
                 manager_port: int = 8380,
                 redis_host: str = "localhost", 
                 redis_port: int = 6380,
                 client_id: str = "claude_code",
                 transport_mode: TransportMode = TransportMode.HTTP_WITH_FALLBACK,
                 timeout_seconds: int = 30):
        """
        Initialize the Centerfire client
        
        Args:
            manager_host: AGT-MANAGER-1 host for service discovery
            manager_port: AGT-MANAGER-1 HTTP port for service discovery  
            redis_host: Redis host for direct/fallback communication
            redis_port: Redis port for direct/fallback communication
            client_id: Client identifier for contract validation
            transport_mode: Communication transport preference
            timeout_seconds: Request timeout
        """
        self.manager_host = manager_host
        self.manager_port = manager_port
        self.redis_host = redis_host
        self.redis_port = redis_port
        self.client_id = client_id
        self.transport_mode = transport_mode
        self.timeout_seconds = timeout_seconds
        
        # Service discovery cache
        self._gateway_info: Optional[Dict[str, Any]] = None
        self._gateway_cache_time: Optional[float] = None
        self._cache_ttl_seconds = 300  # 5 minutes
        
        # Redis client for fallback
        self._redis_client: Optional[redis.Redis] = None
        
        # Logging
        self.logger = logging.getLogger(__name__)
        
    def call_agent(self, 
                   agent_name: str, 
                   action: str, 
                   params: Optional[Dict[str, Any]] = None,
                   force_transport: Optional[TransportMode] = None) -> AgentResponse:
        """
        Call an agent with automatic transport selection and fallback
        
        Args:
            agent_name: Target agent name (e.g., 'naming', 'semantic')
            action: Agent action to perform
            params: Action parameters
            force_transport: Override default transport mode
            
        Returns:
            AgentResponse with result and metadata
        """
        start_time = time.time()
        request_id = f"claude_{int(time.time() * 1000)}_{uuid.uuid4().hex[:8]}"
        
        effective_mode = force_transport or self.transport_mode
        
        try:
            # Try HTTP Gateway first (if not REDIS_ONLY)
            if effective_mode != TransportMode.REDIS_ONLY:
                try:
                    response = self._call_via_http(agent_name, action, params, request_id)
                    response.transport_used = "http_gateway"
                    response.response_time_ms = int((time.time() - start_time) * 1000)
                    self.logger.info(f"HTTP call successful: {agent_name}.{action} in {response.response_time_ms}ms")
                    return response
                except Exception as e:
                    self.logger.warning(f"HTTP call failed for {agent_name}.{action}: {e}")
                    
                    # If HTTP_ONLY, don't fallback
                    if effective_mode == TransportMode.HTTP_ONLY:
                        return AgentResponse(
                            success=False,
                            error=f"HTTP transport failed: {str(e)}",
                            request_id=request_id,
                            transport_used="http_gateway_failed",
                            response_time_ms=int((time.time() - start_time) * 1000)
                        )
            
            # Fallback to Redis (if not HTTP_ONLY)
            if effective_mode != TransportMode.HTTP_ONLY:
                try:
                    response = self._call_via_redis(agent_name, action, params, request_id)
                    response.transport_used = "redis_fallback"
                    response.response_time_ms = int((time.time() - start_time) * 1000)
                    self.logger.info(f"Redis fallback successful: {agent_name}.{action} in {response.response_time_ms}ms")
                    return response
                except Exception as e:
                    self.logger.error(f"Redis fallback failed for {agent_name}.{action}: {e}")
                    return AgentResponse(
                        success=False,
                        error=f"Both HTTP and Redis transports failed. HTTP: {str(e)}, Redis: {str(e)}",
                        request_id=request_id,
                        transport_used="all_failed",
                        response_time_ms=int((time.time() - start_time) * 1000)
                    )
            
        except Exception as e:
            self.logger.error(f"Unexpected error calling {agent_name}.{action}: {e}")
            return AgentResponse(
                success=False,
                error=f"Unexpected error: {str(e)}",
                request_id=request_id,
                transport_used="error",
                response_time_ms=int((time.time() - start_time) * 1000)
            )
    
    def _call_via_http(self, 
                       agent_name: str, 
                       action: str, 
                       params: Optional[Dict[str, Any]], 
                       request_id: str) -> AgentResponse:
        """Call agent via HTTP Gateway"""
        
        # Discover gateway if needed
        gateway_info = self._discover_gateway()
        
        # Build request
        url = f"http://localhost:{gateway_info['port']}/api/agents/{agent_name}/{action}"
        headers = {
            'Content-Type': 'application/json',
            'X-Client-ID': self.client_id
        }
        payload = params or {}
        
        # Make HTTP request
        response = requests.post(
            url,
            headers=headers,
            json=payload,
            timeout=self.timeout_seconds
        )
        
        if response.status_code == 200:
            data = response.json()
            return AgentResponse(
                success=data.get('success', True),
                data=data.get('data', data),  # Gateway might wrap or not wrap data
                error=data.get('error'),
                request_id=request_id
            )
        else:
            # Try to parse error response
            try:
                error_data = response.json()
                error_msg = error_data.get('error', f"HTTP {response.status_code}")
            except:
                error_msg = f"HTTP {response.status_code}: {response.text}"
                
            return AgentResponse(
                success=False,
                error=error_msg,
                request_id=request_id
            )
    
    def _call_via_redis(self, 
                        agent_name: str, 
                        action: str, 
                        params: Optional[Dict[str, Any]], 
                        request_id: str) -> AgentResponse:
        """Call agent via direct Redis pub/sub (fallback)"""
        
        # Initialize Redis client if needed
        if not self._redis_client:
            self._redis_client = redis.Redis(
                host=self.redis_host,
                port=self.redis_port,
                decode_responses=True
            )
        
        # Map agent names to channels
        request_channel = f"agent.{agent_name}.request"
        response_channel = f"agent.{agent_name}.response"
        
        # Subscribe to response channel
        pubsub = self._redis_client.pubsub()
        pubsub.subscribe(response_channel)
        
        try:
            # Publish request
            request_data = {
                'action': action,
                'params': params or {},
                'client_id': self.client_id,
                'request_id': request_id
            }
            
            self._redis_client.publish(request_channel, json.dumps(request_data))
            
            # Wait for response
            timeout_time = time.time() + self.timeout_seconds
            
            for message in pubsub.listen():
                if message['type'] == 'message':
                    try:
                        response_data = json.loads(message['data'])
                        
                        # Check if this is our response
                        if response_data.get('request_id') == request_id:
                            return AgentResponse(
                                success=response_data.get('success', not bool(response_data.get('error'))),
                                data=response_data,
                                error=response_data.get('error'),
                                request_id=request_id
                            )
                    except json.JSONDecodeError:
                        continue
                
                # Check timeout
                if time.time() > timeout_time:
                    raise TimeoutError(f"Redis call timed out after {self.timeout_seconds} seconds")
                    
        finally:
            pubsub.unsubscribe(response_channel)
            pubsub.close()
    
    def _discover_gateway(self) -> Dict[str, Any]:
        """Discover HTTP Gateway via service discovery"""
        
        # Check cache first
        if (self._gateway_info and 
            self._gateway_cache_time and 
            time.time() - self._gateway_cache_time < self._cache_ttl_seconds):
            return self._gateway_info
        
        # Query manager for gateway info
        try:
            discovery_url = f"http://{self.manager_host}:{self.manager_port}/api/services/http-gateway"
            response = requests.get(discovery_url, timeout=10)
            
            if response.status_code == 200:
                data = response.json()
                if data.get('success'):
                    service_info = data.get('service', {})
                    
                    # Cache the result
                    self._gateway_info = service_info
                    self._gateway_cache_time = time.time()
                    
                    return service_info
                else:
                    raise Exception(f"Service discovery failed: {data.get('error', 'Unknown error')}")
            else:
                raise Exception(f"Manager returned HTTP {response.status_code}")
                
        except Exception as e:
            raise Exception(f"Service discovery failed: {str(e)}")
    
    def get_gateway_status(self) -> Dict[str, Any]:
        """Get current gateway status and connection info"""
        try:
            gateway_info = self._discover_gateway()
            return {
                'discovered': True,
                'gateway_info': gateway_info,
                'cache_age_seconds': time.time() - (self._gateway_cache_time or 0),
                'transport_mode': self.transport_mode.value
            }
        except Exception as e:
            return {
                'discovered': False,
                'error': str(e),
                'transport_mode': self.transport_mode.value,
                'redis_available': self._check_redis_connection()
            }
    
    def _check_redis_connection(self) -> bool:
        """Check if Redis fallback is available"""
        try:
            if not self._redis_client:
                self._redis_client = redis.Redis(
                    host=self.redis_host,
                    port=self.redis_port,
                    decode_responses=True
                )
            self._redis_client.ping()
            return True
        except:
            return False
    
    def health_check(self) -> Dict[str, Any]:
        """Comprehensive health check of all transport methods"""
        result = {
            'timestamp': time.time(),
            'client_id': self.client_id,
            'transports': {}
        }
        
        # Check HTTP Gateway
        try:
            gateway_info = self._discover_gateway()
            gateway_url = f"http://localhost:{gateway_info['port']}/health"
            response = requests.get(gateway_url, timeout=5)
            result['transports']['http_gateway'] = {
                'available': response.status_code == 200,
                'gateway_port': gateway_info.get('port'),
                'response_time_ms': response.elapsed.total_seconds() * 1000 if response else None
            }
        except Exception as e:
            result['transports']['http_gateway'] = {
                'available': False,
                'error': str(e)
            }
        
        # Check Redis
        result['transports']['redis'] = {
            'available': self._check_redis_connection(),
            'host': self.redis_host,
            'port': self.redis_port
        }
        
        # Check Manager
        try:
            manager_url = f"http://{self.manager_host}:{self.manager_port}/health"
            response = requests.get(manager_url, timeout=5)
            result['manager'] = {
                'available': response.status_code == 200,
                'response_time_ms': response.elapsed.total_seconds() * 1000
            }
        except Exception as e:
            result['manager'] = {
                'available': False,
                'error': str(e)
            }
        
        return result


# Convenience functions for quick usage
def call_agent(agent_name: str, 
               action: str, 
               params: Optional[Dict[str, Any]] = None,
               **kwargs) -> AgentResponse:
    """Quick agent call with default client"""
    client = CenterfireClient(**kwargs)
    return client.call_agent(agent_name, action, params)

def call_naming_agent(action: str, params: Optional[Dict[str, Any]] = None, **kwargs) -> AgentResponse:
    """Quick naming agent call"""
    return call_agent('naming', action, params, **kwargs)

def call_semantic_agent(action: str, params: Optional[Dict[str, Any]] = None, **kwargs) -> AgentResponse:
    """Quick semantic agent call"""
    return call_agent('semantic', action, params, **kwargs)


if __name__ == "__main__":
    # Demo usage
    import sys
    
    logging.basicConfig(level=logging.INFO)
    
    client = CenterfireClient()
    
    print("🔍 Centerfire HTTP Client with Redis Fallback")
    print("=" * 50)
    
    # Health check
    print("\n📋 Health Check:")
    health = client.health_check()
    print(json.dumps(health, indent=2, default=str))
    
    # Test call
    print("\n🧪 Test Call (AGT-NAMING-1):")
    result = client.call_agent('naming', 'allocate_capability', {
        'domain': 'HTTPTEST',
        'description': 'Testing HTTP client with fallback'
    })
    
    print(f"Success: {result.success}")
    print(f"Transport: {result.transport_used}")
    print(f"Time: {result.response_time_ms}ms")
    if result.data:
        print(f"Data: {json.dumps(result.data, indent=2)}")
    if result.error:
        print(f"Error: {result.error}")