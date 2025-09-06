#!/usr/bin/env python3
"""
Comprehensive test suite for the Centerfire HTTP Client with fallback capabilities.
Tests various scenarios including HTTP success, Redis fallback, and error conditions.
"""

import json
import time
import logging
from claude_http_client import CenterfireClient, TransportMode, call_naming_agent

def test_http_transport():
    """Test HTTP transport exclusively"""
    print("🔬 Testing HTTP Transport Only...")
    client = CenterfireClient(transport_mode=TransportMode.HTTP_ONLY)
    
    result = client.call_agent('naming', 'allocate_capability', {
        'domain': 'HTTPONLY',
        'description': 'Testing HTTP-only transport'
    })
    
    print(f"   Result: {result.success}")
    print(f"   Transport: {result.transport_used}")
    print(f"   Time: {result.response_time_ms}ms")
    if result.error:
        print(f"   Error: {result.error}")
    return result.success

def test_redis_transport():
    """Test Redis transport exclusively"""
    print("🔬 Testing Redis Transport Only...")
    client = CenterfireClient(transport_mode=TransportMode.REDIS_ONLY)
    
    result = client.call_agent('naming', 'allocate_capability', {
        'domain': 'REDISONLY',
        'description': 'Testing Redis-only transport'
    })
    
    print(f"   Result: {result.success}")
    print(f"   Transport: {result.transport_used}")
    print(f"   Time: {result.response_time_ms}ms")
    if result.error:
        print(f"   Error: {result.error}")
    return result.success

def test_http_with_fallback():
    """Test HTTP with Redis fallback (default mode)"""
    print("🔬 Testing HTTP with Redis Fallback...")
    client = CenterfireClient(transport_mode=TransportMode.HTTP_WITH_FALLBACK)
    
    result = client.call_agent('naming', 'allocate_capability', {
        'domain': 'HTTPFALLBACK',
        'description': 'Testing HTTP with fallback transport'
    })
    
    print(f"   Result: {result.success}")
    print(f"   Transport: {result.transport_used}")
    print(f"   Time: {result.response_time_ms}ms")
    if result.error:
        print(f"   Error: {result.error}")
    return result.success

def test_semantic_agent():
    """Test different agent (semantic) via HTTP"""
    print("🔬 Testing AGT-SEMANTIC-1 via HTTP...")
    client = CenterfireClient()
    
    result = client.call_agent('semantic', 'store_concept', {
        'concept': 'HTTP Client Test',
        'description': 'Testing semantic agent via HTTP gateway',
        'tags': ['test', 'http', 'client']
    })
    
    print(f"   Result: {result.success}")
    print(f"   Transport: {result.transport_used}")
    print(f"   Time: {result.response_time_ms}ms")
    if result.error:
        print(f"   Error: {result.error}")
    return result.success

def test_convenience_functions():
    """Test convenience functions"""
    print("🔬 Testing Convenience Functions...")
    
    # Test naming agent convenience function
    result = call_naming_agent('allocate_capability', {
        'domain': 'CONVENIENCE',
        'description': 'Testing convenience function'
    })
    
    print(f"   Result: {result.success}")
    print(f"   Transport: {result.transport_used}")
    print(f"   Time: {result.response_time_ms}ms")
    return result.success

def test_health_checks():
    """Test comprehensive health check"""
    print("🔬 Testing Health Check...")
    client = CenterfireClient()
    
    health = client.health_check()
    print(f"   Manager Available: {health.get('manager', {}).get('available', False)}")
    print(f"   HTTP Gateway Available: {health.get('transports', {}).get('http_gateway', {}).get('available', False)}")
    print(f"   Redis Available: {health.get('transports', {}).get('redis', {}).get('available', False)}")
    
    return True

def test_service_discovery():
    """Test service discovery functionality"""
    print("🔬 Testing Service Discovery...")
    client = CenterfireClient()
    
    try:
        status = client.get_gateway_status()
        print(f"   Gateway Discovered: {status.get('discovered', False)}")
        if status.get('gateway_info'):
            print(f"   Gateway Port: {status['gateway_info'].get('port', 'Unknown')}")
        print(f"   Cache Age: {status.get('cache_age_seconds', 0):.1f}s")
        return status.get('discovered', False)
    except Exception as e:
        print(f"   Error: {e}")
        return False

def test_error_handling():
    """Test error handling with invalid agent"""
    print("🔬 Testing Error Handling...")
    client = CenterfireClient()
    
    result = client.call_agent('nonexistent', 'invalid_action', {})
    
    print(f"   Expected Failure: {not result.success}")
    print(f"   Transport: {result.transport_used}")
    print(f"   Error: {result.error}")
    return not result.success  # We expect this to fail

def performance_comparison():
    """Compare HTTP vs Redis performance"""
    print("🔬 Performance Comparison...")
    
    # HTTP performance
    client_http = CenterfireClient(transport_mode=TransportMode.HTTP_ONLY)
    start = time.time()
    result_http = client_http.call_agent('naming', 'allocate_capability', {
        'domain': 'PERFHTTP',
        'description': 'HTTP performance test'
    })
    http_time = time.time() - start
    
    # Redis performance  
    client_redis = CenterfireClient(transport_mode=TransportMode.REDIS_ONLY)
    start = time.time()
    result_redis = client_redis.call_agent('naming', 'allocate_capability', {
        'domain': 'PERFREDIS',
        'description': 'Redis performance test'
    })
    redis_time = time.time() - start
    
    print(f"   HTTP Time: {http_time*1000:.1f}ms (Success: {result_http.success})")
    print(f"   Redis Time: {redis_time*1000:.1f}ms (Success: {result_redis.success})")
    
    if result_http.success and result_redis.success:
        faster = "HTTP" if http_time < redis_time else "Redis"
        ratio = max(http_time, redis_time) / min(http_time, redis_time)
        print(f"   Winner: {faster} is {ratio:.1f}x faster")

def main():
    """Run comprehensive test suite"""
    logging.basicConfig(level=logging.WARNING)  # Reduce noise
    
    print("🧪 Centerfire HTTP Client Test Suite")
    print("=" * 50)
    
    tests = [
        ("Service Discovery", test_service_discovery),
        ("Health Checks", test_health_checks),
        ("HTTP Transport", test_http_transport),
        ("Redis Transport", test_redis_transport), 
        ("HTTP with Fallback", test_http_with_fallback),
        ("Semantic Agent", test_semantic_agent),
        ("Convenience Functions", test_convenience_functions),
        ("Error Handling", test_error_handling),
    ]
    
    results = {}
    for test_name, test_func in tests:
        print(f"\n{test_name}:")
        try:
            results[test_name] = test_func()
        except Exception as e:
            print(f"   ❌ Test failed with exception: {e}")
            results[test_name] = False
    
    print(f"\n🔬 Performance Comparison:")
    performance_comparison()
    
    # Summary
    print(f"\n📊 Test Results Summary:")
    print("=" * 30)
    passed = sum(1 for r in results.values() if r)
    total = len(results)
    
    for test_name, passed_test in results.items():
        status = "✅ PASS" if passed_test else "❌ FAIL"
        print(f"{status} {test_name}")
    
    print(f"\n🎯 Overall: {passed}/{total} tests passed ({passed/total*100:.0f}%)")
    
    if passed == total:
        print("🎉 All tests passed! HTTP client with fallback is fully operational.")
    elif passed >= total * 0.7:
        print("⚠️  Most tests passed. Minor issues detected.")  
    else:
        print("🚨 Multiple test failures. HTTP client needs attention.")

if __name__ == "__main__":
    main()