#!/usr/bin/env python3
"""
Redis-based Todo Manager for Centerfire Intelligence
Manages general todo items stored in Redis for persistence across sessions
"""

import redis
import sys
from datetime import datetime

class TodoManager:
    def __init__(self):
        self.redis_client = redis.Redis(host='localhost', port=6380, decode_responses=True)
        self.todo_key = "centerfire:general:todos"
    
    def add_todo(self, description, priority="Medium"):
        """Add a new todo item"""
        timestamp = datetime.now().strftime("%Y-%m-%d")
        todo_item = f"{description} - Priority: {priority} - Added: {timestamp}"
        self.redis_client.lpush(self.todo_key, todo_item)
        print(f"✅ Added: {description}")
    
    def list_todos(self):
        """List all todo items"""
        todos = self.redis_client.lrange(self.todo_key, 0, -1)
        if not todos:
            print("📝 No todos found")
            return
        
        print("📋 Current Todos:")
        for i, todo in enumerate(todos, 1):
            print(f"{i:2d}. {todo}")
    
    def remove_todo(self, index):
        """Remove todo by index (1-based)"""
        todos = self.redis_client.lrange(self.todo_key, 0, -1)
        if not todos or index < 1 or index > len(todos):
            print(f"❌ Invalid index: {index}")
            return
        
        # Redis lists are 0-indexed
        todo_to_remove = todos[index - 1]
        self.redis_client.lrem(self.todo_key, 1, todo_to_remove)
        print(f"✅ Removed: {todo_to_remove}")
    
    def search_todos(self, query):
        """Search todos containing query string"""
        todos = self.redis_client.lrange(self.todo_key, 0, -1)
        matches = [todo for todo in todos if query.lower() in todo.lower()]
        
        if not matches:
            print(f"🔍 No todos found containing: {query}")
            return
        
        print(f"🔍 Todos containing '{query}':")
        for i, todo in enumerate(matches, 1):
            print(f"{i:2d}. {todo}")

def main():
    manager = TodoManager()
    
    if len(sys.argv) < 2:
        print("Usage:")
        print("  python3 todo_manager.py list")
        print("  python3 todo_manager.py add 'Description' [priority]")
        print("  python3 todo_manager.py remove <index>")
        print("  python3 todo_manager.py search <query>")
        return
    
    command = sys.argv[1]
    
    if command == "list":
        manager.list_todos()
    elif command == "add" and len(sys.argv) >= 3:
        description = sys.argv[2]
        priority = sys.argv[3] if len(sys.argv) > 3 else "Medium"
        manager.add_todo(description, priority)
    elif command == "remove" and len(sys.argv) >= 3:
        try:
            index = int(sys.argv[2])
            manager.remove_todo(index)
        except ValueError:
            print("❌ Index must be a number")
    elif command == "search" and len(sys.argv) >= 3:
        query = sys.argv[2]
        manager.search_todos(query)
    else:
        print("❌ Invalid command or missing arguments")

if __name__ == "__main__":
    main()