#!/usr/bin/env python3
"""
Simple test to verify Casbin gRPC service is working
"""
import grpc
import sys

def test_casbin_connection():
    """Test basic gRPC connection to Casbin"""
    try:
        # Test if we can connect to the gRPC service
        with grpc.insecure_channel('localhost:50051') as channel:
            # Try to establish connection
            grpc.channel_ready_future(channel).result(timeout=10)
            print("✅ Casbin gRPC service is accessible on localhost:50051")
            return True
    except grpc.FutureTimeoutError:
        print("❌ Timeout connecting to Casbin gRPC service")
        return False
    except Exception as e:
        print(f"❌ Error connecting to Casbin: {e}")
        return False

if __name__ == "__main__":
    print("Testing Casbin gRPC Service Connection...")
    success = test_casbin_connection()
    
    if success:
        print("\n🎉 Casbin authorization service is ready for SemDoc Stage 1!")
        print("Next steps:")
        print("- Implement gRPC client in AGT-SEMDOC-PARSER-1")
        print("- Add authorization checks using casbin/policies/semdoc_agents.csv")
        print("- Begin Stage 1 traditional development with RBAC")
        sys.exit(0)
    else:
        print("\n❌ Casbin service needs debugging")
        print("Check: docker logs centerfire-casbin")
        sys.exit(1)