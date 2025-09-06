# PTY Proxy Multi-Interface Implementation Plan
*Documented: 2025-09-06*  
*Context: Comprehensive plan for PTY-based Claude Code isolation and multi-interface architecture*  
*Status: Implementation Ready*

---

## 🎯 Executive Summary

This document defines the complete implementation plan for transitioning from direct Claude Code agent coupling to a PTY proxy-based multi-interface architecture. The system will enable conversation capture, agent orchestration, and support for multiple client interfaces (web, API, terminal) while maintaining operational continuity of existing systems.

---

## 📁 Directory Structure Design - Separation of Concerns

### **Critical Design Principle**: Future Separation Path
The directory structure must enable **easy removal of Claude Code direct editing functionality** once the orchestrator can reliably handle all operations through agent delegation.

### **Proposed Directory Architecture**

```
CenterfireIntelligence/
├── agents/                          # KEEP: Core Go agents (persistent)
│   ├── AGT-NAMING-1__01K4EAF1/      # Current Redis-based agents
│   ├── AGT-STRUCT-1__01K4EAF1/      # Extended with socket interfaces
│   ├── AGT-SEMANTIC-1__01K4EAF1/    # Dual-mode: Redis + Unix sockets
│   └── AGT-MANAGER-1__manager1/     # Enhanced for multi-interface management
│
├── orchestrator/                    # NEW: Rust-based core system
│   ├── src/
│   │   ├── main.rs                  # Entry point and service coordination
│   │   ├── pty_proxy/               # PTY proxy implementation
│   │   │   ├── mod.rs               # PTY proxy module
│   │   │   ├── claude_proxy.rs      # Claude Code PTY interception
│   │   │   ├── conversation_capture.rs # I/O capture and processing
│   │   │   └── session_manager.rs   # Session lifecycle management
│   │   ├── socket_manager/          # Agent communication layer
│   │   │   ├── mod.rs               # Socket management module
│   │   │   ├── agent_pool.rs        # Unix socket pool for Go agents
│   │   │   ├── request_router.rs    # Route requests to appropriate agents
│   │   │   └── response_processor.rs # Process and format agent responses
│   │   ├── interfaces/              # Multi-interface support
│   │   │   ├── mod.rs               # Interface abstraction
│   │   │   ├── http_server.rs       # HTTP API for external clients
│   │   │   ├── websocket_server.rs  # WebSocket for real-time web interface
│   │   │   └── api_gateway.rs       # Unified API gateway
│   │   ├── llm_integration/         # LLM API management
│   │   │   ├── mod.rs               # LLM integration module
│   │   │   ├── claude_client.rs     # Claude API integration
│   │   │   ├── context_manager.rs   # Token and context management
│   │   │   └── response_parser.rs   # Parse LLM responses for action extraction
│   │   ├── conversation_processor/  # Conversation analysis and storage
│   │   │   ├── mod.rs               # Conversation processing
│   │   │   ├── capture_engine.rs    # Real-time conversation capture
│   │   │   ├── semantic_extractor.rs # Extract semantic information
│   │   │   └── storage_dispatcher.rs # Route to Neo4j, Weaviate, etc.
│   │   └── config/                  # Configuration management
│   │       ├── mod.rs               # Configuration module
│   │       ├── orchestrator_config.rs # Core orchestrator settings
│   │       └── interface_config.rs  # Interface-specific configurations
│   ├── Cargo.toml                   # Rust dependencies
│   ├── build.rs                     # Build script for cross-platform
│   └── README.md                    # Orchestrator documentation
│
├── claude-direct/                   # TEMPORARY: Direct Claude Code functionality
│   ├── file_operations/             # File editing, creation, deletion
│   │   ├── edit_handlers.rs         # Current file editing logic
│   │   ├── creation_handlers.rs     # File/directory creation
│   │   └── permission_manager.rs    # File permission handling
│   ├── tool_integrations/           # Direct tool usage (Bash, Read, Write, etc.)
│   │   ├── bash_wrapper.rs          # Bash command execution
│   │   ├── file_io_wrapper.rs       # File I/O operations
│   │   └── git_wrapper.rs           # Git operations
│   └── migration_plan.md            # Plan for deprecating this module
│
├── web-interface/                   # NEW: Web-based client interface
│   ├── src/                         # Node.js/React frontend
│   │   ├── components/              # React components
│   │   ├── services/                # WebSocket client services
│   │   └── pages/                   # Application pages
│   ├── server/                      # Node.js backend
│   │   ├── websocket_client.rs      # Connect to Rust WebSocket server
│   │   └── api_proxy.rs             # Proxy HTTP requests to orchestrator
│   ├── package.json                 # Node dependencies
│   └── README.md                    # Web interface documentation
│
├── testing-environments/           # NEW: Isolated testing setups
│   ├── account-1-current/          # Current system preservation
│   │   ├── test-session-setup.md   # Instructions for current account testing
│   │   └── comparison-metrics.md   # Metrics for system comparison
│   ├── account-2-pty-proxy/        # PTY proxy testing environment
│   │   ├── proxy-session-setup.md  # PTY proxy testing instructions
│   │   └── conversation-logs/      # Captured conversation analysis
│   └── integration-tests/          # Cross-system validation tests
│       ├── socket-communication/   # Agent socket communication tests
│       ├── pty-interception/       # PTY proxy validation tests
│       └── multi-interface/        # Multi-client interface tests
│
├── legacy-redis-system/            # PRESERVE: Current Redis pub/sub system
│   ├── current-hooks/              # Existing Claude Code hooks
│   ├── redis-channels/             # Current Redis channel documentation
│   └── migration-compatibility.md  # Compatibility layer documentation
│
├── utils/                          # KEEP: Enhanced utilities
│   ├── stream-processor/           # Current Redis Streams processor
│   ├── backfill-utility/           # Historical data migration
│   └── pty-utilities/              # NEW: PTY-specific utilities
│       ├── session-recorder/       # Session recording tools
│       ├── log-analyzer/           # Conversation log analysis
│       └── benchmark-tools/        # Performance measurement tools
│
├── DOC-ARCH-1__genesis/            # KEEP: Enhanced documentation
│   ├── SOCKET_ARCHITECTURE_DECISIONS.md    # Previous architectural decisions
│   ├── PTY_PROXY_IMPLEMENTATION_PLAN.md    # This document
│   ├── MIGRATION_ROADMAP.md                # NEW: Migration timeline and steps
│   └── INTERFACE_SPECIFICATIONS.md         # NEW: API specifications for all interfaces
│
└── DIR-SYS-1__genesis/             # KEEP: System directives and status
    └── claude-agent-protocol.yaml  # Updated with PTY proxy architecture
```

---

## 🏗️ Implementation Architecture

### **Multi-Layer System Design**

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          Client Interface Layer                         │
├─────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │Claude Code  │  │Web Browser  │  │VS Code Ext  │  │API Clients  │     │
│  │(PTY Proxy)  │  │(WebSocket)  │  │   (HTTP)    │  │   (HTTP)    │     │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘     │
└─────────┼─────────────────┼─────────────────┼─────────────────┼──────────┘
          │                 │                 │                 │
┌─────────▼─────────────────▼─────────────────▼─────────────────▼──────────┐
│                     Rust Orchestrator Core                              │
├─────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                    Interface Manager                           │    │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │    │
│  │  │PTY Proxy    │  │HTTP Server  │  │WebSocket    │              │    │
│  │  │Manager      │  │             │  │Server       │              │    │
│  │  └─────────────┘  └─────────────┘  └─────────────┘              │    │
│  └─────────────────────┬───────────────────────────────────────────┘    │
│                        │                                                │
│  ┌─────────────────────▼───────────────────────────────────────────┐    │
│  │                 Agent Socket Manager                           │    │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │    │
│  │  │Naming       │  │Struct       │  │Semantic     │              │    │
│  │  │Socket       │  │Socket       │  │Socket       │              │    │
│  │  └─────────────┘  └─────────────┘  └─────────────┘              │    │
│  └─────────────────────┬───────────────────────────────────────────┘    │
│                        │                                                │
│  ┌─────────────────────▼───────────────────────────────────────────┐    │
│  │               Conversation Processor                           │    │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │    │
│  │  │Capture      │  │Semantic     │  │Storage      │              │    │
│  │  │Engine       │  │Extractor    │  │Dispatcher   │              │    │
│  │  └─────────────┘  └─────────────┘  └─────────────┘              │    │
│  └─────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────┘
          │                 │                 │                 │
┌─────────▼─────────────────▼─────────────────▼─────────────────▼──────────┐
│                       Go Agent Layer                                    │
├─────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │AGT-NAMING-1 │  │AGT-STRUCT-1 │  │AGT-SEMANTIC │  │AGT-MANAGER-1│     │
│  │(Dual Mode)  │  │(Dual Mode)  │  │(Dual Mode)  │  │(Enhanced)   │     │
│  │Redis+Socket │  │Redis+Socket │  │Redis+Socket │  │Multi-Iface  │     │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘     │
└─────────────────────────────────────────────────────────────────────────┘
          │                 │                 │                 │
┌─────────▼─────────────────▼─────────────────▼─────────────────▼──────────┐
│                        Storage Layer                                    │
├─────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │Redis        │  │Neo4j        │  │Weaviate     │  │Qdrant       │     │
│  │(Streams)    │  │(Relations)  │  │(Code Sem.)  │  │(Conv. Mem.) │     │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘     │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 🔧 Technical Implementation Details

### **PTY Proxy Implementation (orchestrator/src/pty_proxy/)**

#### **Core PTY Proxy (`claude_proxy.rs`)**
```rust
use portable_pty::{CommandBuilder, PtySize, native_pty_system};
use tokio::net::UnixStream;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone)]
pub struct ClaudeProxyConfig {
    pub claude_command: String,
    pub session_id: String,
    pub working_directory: String,
    pub socket_path: String,
    pub log_file: PathBuf,
    pub buffer_size: usize,
}

pub struct ClaudeProxy {
    config: ClaudeProxyConfig,
    pty_master: Box<dyn MasterPty + Send>,
    orchestrator_socket: UnixStream,
    conversation_capture: ConversationCapture,
    session_manager: SessionManager,
}

impl ClaudeProxy {
    pub async fn new(config: ClaudeProxyConfig) -> Result<Self, ProxyError> {
        let pty_system = native_pty_system();
        let pty_pair = pty_system.openpty(PtySize {
            rows: 24,
            cols: 80,
            pixel_width: 0,
            pixel_height: 0,
        })?;

        let orchestrator_socket = UnixStream::connect(&config.socket_path).await?;
        let conversation_capture = ConversationCapture::new(&config.log_file)?;
        let session_manager = SessionManager::new(config.session_id.clone());

        Ok(ClaudeProxy {
            config,
            pty_master: pty_pair.master,
            orchestrator_socket,
            conversation_capture,
            session_manager,
        })
    }

    pub async fn start_claude_session(&mut self) -> Result<(), ProxyError> {
        // Spawn Claude Code in PTY
        let mut cmd = CommandBuilder::new(&self.config.claude_command);
        cmd.cwd(&self.config.working_directory);
        
        let mut child = self.pty_master.slave().spawn_command(cmd)?;
        
        // Create async readers/writers for PTY
        let mut pty_reader = self.pty_master.try_clone_reader()?;
        let mut pty_writer = self.pty_master.try_clone_writer()?;
        
        // Create stdin reader for user input
        let stdin = tokio::io::stdin();
        let mut stdin_reader = BufReader::new(stdin);
        
        // Main event loop
        let mut claude_output_buffer = vec![0u8; self.config.buffer_size];
        let mut user_input_buffer = vec![0u8; self.config.buffer_size];
        
        loop {
            tokio::select! {
                // Handle Claude Code output
                result = pty_reader.read(&mut claude_output_buffer) => {
                    match result {
                        Ok(n) if n > 0 => {
                            let output = &claude_output_buffer[..n];
                            
                            // Capture conversation
                            self.conversation_capture.capture_claude_output(output).await?;
                            
                            // Send to orchestrator for processing
                            self.send_to_orchestrator(
                                ProxyMessage::ClaudeOutput { 
                                    data: output.to_vec(),
                                    timestamp: chrono::Utc::now(),
                                }
                            ).await?;
                            
                            // Forward to user terminal (with potential modifications)
                            let processed_output = self.process_claude_output(output).await?;
                            print!("{}", String::from_utf8_lossy(&processed_output));
                            io::stdout().flush()?;
                        },
                        Ok(0) => {
                            println!("Claude Code session ended");
                            break;
                        },
                        Err(e) => return Err(ProxyError::PtyRead(e)),
                    }
                },
                
                // Handle user input
                result = stdin_reader.read(&mut user_input_buffer) => {
                    match result {
                        Ok(n) if n > 0 => {
                            let input = &user_input_buffer[..n];
                            
                            // Capture user input
                            self.conversation_capture.capture_user_input(input).await?;
                            
                            // Send to orchestrator for analysis
                            self.send_to_orchestrator(
                                ProxyMessage::UserInput {
                                    data: input.to_vec(),
                                    timestamp: chrono::Utc::now(),
                                }
                            ).await?;
                            
                            // Forward to Claude Code (with potential interception)
                            let processed_input = self.process_user_input(input).await?;
                            pty_writer.write_all(&processed_input).await?;
                        },
                        Ok(0) => {
                            println!("User input stream ended");
                            break;
                        },
                        Err(e) => return Err(ProxyError::StdinRead(e)),
                    }
                }
            }
        }
        
        Ok(())
    }
    
    async fn send_to_orchestrator(&mut self, message: ProxyMessage) -> Result<(), ProxyError> {
        let serialized = serde_json::to_vec(&message)?;
        let length = serialized.len() as u32;
        
        // Send length prefix + message
        self.orchestrator_socket.write_all(&length.to_le_bytes()).await?;
        self.orchestrator_socket.write_all(&serialized).await?;
        
        Ok(())
    }
    
    async fn process_claude_output(&self, output: &[u8]) -> Result<Vec<u8>, ProxyError> {
        // Future: Modify Claude output based on orchestrator responses
        // For now, pass through unchanged
        Ok(output.to_vec())
    }
    
    async fn process_user_input(&self, input: &[u8]) -> Result<Vec<u8>, ProxyError> {
        // Future: Intercept specific commands or modify user input
        // For now, pass through unchanged
        Ok(input.to_vec())
    }
}

#[derive(Debug, Serialize, Deserialize)]
pub enum ProxyMessage {
    ClaudeOutput {
        data: Vec<u8>,
        timestamp: chrono::DateTime<chrono::Utc>,
    },
    UserInput {
        data: Vec<u8>,
        timestamp: chrono::DateTime<chrono::Utc>,
    },
    SessionStart {
        session_id: String,
        working_directory: String,
    },
    SessionEnd {
        session_id: String,
        duration: std::time::Duration,
    },
}

#[derive(Debug, thiserror::Error)]
pub enum ProxyError {
    #[error("PTY error: {0}")]
    Pty(#[from] portable_pty::Error),
    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),
    #[error("Serialization error: {0}")]
    Serde(#[from] serde_json::Error),
    #[error("PTY read error: {0}")]
    PtyRead(std::io::Error),
    #[error("Stdin read error: {0}")]
    StdinRead(std::io::Error),
    #[error("Socket connection error: {0}")]
    Socket(#[from] tokio::io::Error),
}
```

#### **Conversation Capture Engine (`conversation_capture.rs`)**
```rust
use tokio::fs::OpenOptions;
use tokio::io::{AsyncWriteExt, BufWriter};
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ConversationEntry {
    pub timestamp: DateTime<Utc>,
    pub entry_type: ConversationEntryType,
    pub content: String,
    pub metadata: ConversationMetadata,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum ConversationEntryType {
    UserInput,
    ClaudeOutput,
    ToolExecution,
    SystemMessage,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ConversationMetadata {
    pub session_id: String,
    pub working_directory: String,
    pub sequence_number: u64,
    pub content_length: usize,
    pub is_complete_message: bool,
}

pub struct ConversationCapture {
    log_writer: BufWriter<tokio::fs::File>,
    session_id: String,
    sequence_counter: std::sync::Arc<std::sync::atomic::AtomicU64>,
    current_working_directory: String,
}

impl ConversationCapture {
    pub async fn new(log_file: &std::path::Path, session_id: String, working_dir: String) -> Result<Self, std::io::Error> {
        let file = OpenOptions::new()
            .create(true)
            .append(true)
            .open(log_file)
            .await?;
        
        let log_writer = BufWriter::new(file);
        
        Ok(ConversationCapture {
            log_writer,
            session_id,
            sequence_counter: std::sync::Arc::new(std::sync::atomic::AtomicU64::new(0)),
            current_working_directory: working_dir,
        })
    }
    
    pub async fn capture_user_input(&mut self, input: &[u8]) -> Result<(), std::io::Error> {
        let content = String::from_utf8_lossy(input).to_string();
        let entry = ConversationEntry {
            timestamp: Utc::now(),
            entry_type: ConversationEntryType::UserInput,
            content,
            metadata: ConversationMetadata {
                session_id: self.session_id.clone(),
                working_directory: self.current_working_directory.clone(),
                sequence_number: self.sequence_counter.fetch_add(1, std::sync::atomic::Ordering::SeqCst),
                content_length: input.len(),
                is_complete_message: self.is_complete_input(input),
            },
        };
        
        self.write_entry(entry).await
    }
    
    pub async fn capture_claude_output(&mut self, output: &[u8]) -> Result<(), std::io::Error> {
        let content = String::from_utf8_lossy(output).to_string();
        let entry = ConversationEntry {
            timestamp: Utc::now(),
            entry_type: ConversationEntryType::ClaudeOutput,
            content,
            metadata: ConversationMetadata {
                session_id: self.session_id.clone(),
                working_directory: self.current_working_directory.clone(),
                sequence_number: self.sequence_counter.fetch_add(1, std::sync::atomic::Ordering::SeqCst),
                content_length: output.len(),
                is_complete_message: self.is_complete_output(output),
            },
        };
        
        self.write_entry(entry).await
    }
    
    async fn write_entry(&mut self, entry: ConversationEntry) -> Result<(), std::io::Error> {
        let serialized = serde_json::to_string(&entry)?;
        self.log_writer.write_all(serialized.as_bytes()).await?;
        self.log_writer.write_all(b"\n").await?;
        self.log_writer.flush().await?;
        Ok(())
    }
    
    fn is_complete_input(&self, input: &[u8]) -> bool {
        // Check if input ends with newline (complete command)
        input.ends_with(b"\n") || input.ends_with(b"\r\n")
    }
    
    fn is_complete_output(&self, output: &[u8]) -> bool {
        // Heuristic: Check for Claude Code prompt patterns
        let content = String::from_utf8_lossy(output);
        content.contains("claudeCode:") || content.contains("$ ")
    }
}
```

### **Socket Manager Implementation (orchestrator/src/socket_manager/)**

#### **Agent Pool Manager (`agent_pool.rs`)**
```rust
use tokio::net::{UnixListener, UnixStream};
use std::collections::HashMap;
use std::path::PathBuf;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone)]
pub struct AgentPoolConfig {
    pub socket_directory: PathBuf,
    pub agents: Vec<AgentConfig>,
    pub connection_timeout: std::time::Duration,
    pub retry_attempts: u32,
}

#[derive(Debug, Clone)]
pub struct AgentConfig {
    pub agent_id: String,
    pub socket_path: PathBuf,
    pub capabilities: Vec<String>,
    pub health_check_interval: std::time::Duration,
}

pub struct AgentPool {
    config: AgentPoolConfig,
    connections: HashMap<String, AgentConnection>,
    listener: UnixListener,
}

struct AgentConnection {
    stream: UnixStream,
    config: AgentConfig,
    last_health_check: std::time::Instant,
    is_healthy: bool,
}

impl AgentPool {
    pub async fn new(config: AgentPoolConfig) -> Result<Self, AgentPoolError> {
        // Create socket directory if it doesn't exist
        tokio::fs::create_dir_all(&config.socket_directory).await?;
        
        // Bind to main orchestrator socket
        let orchestrator_socket = config.socket_directory.join("orchestrator.sock");
        if orchestrator_socket.exists() {
            tokio::fs::remove_file(&orchestrator_socket).await?;
        }
        
        let listener = UnixListener::bind(&orchestrator_socket)?;
        
        let mut agent_pool = AgentPool {
            config,
            connections: HashMap::new(),
            listener,
        };
        
        // Initialize connections to all configured agents
        agent_pool.initialize_agent_connections().await?;
        
        Ok(agent_pool)
    }
    
    async fn initialize_agent_connections(&mut self) -> Result<(), AgentPoolError> {
        for agent_config in &self.config.agents.clone() {
            match self.connect_to_agent(agent_config).await {
                Ok(connection) => {
                    self.connections.insert(agent_config.agent_id.clone(), connection);
                    println!("Connected to agent: {}", agent_config.agent_id);
                },
                Err(e) => {
                    eprintln!("Failed to connect to agent {}: {}", agent_config.agent_id, e);
                    // Continue with other agents, don't fail completely
                }
            }
        }
        
        Ok(())
    }
    
    async fn connect_to_agent(&self, config: &AgentConfig) -> Result<AgentConnection, AgentPoolError> {
        let stream = tokio::time::timeout(
            self.config.connection_timeout,
            UnixStream::connect(&config.socket_path)
        ).await??;
        
        let connection = AgentConnection {
            stream,
            config: config.clone(),
            last_health_check: std::time::Instant::now(),
            is_healthy: true,
        };
        
        Ok(connection)
    }
    
    pub async fn send_request(&mut self, agent_id: &str, request: AgentRequest) -> Result<AgentResponse, AgentPoolError> {
        let connection = self.connections.get_mut(agent_id)
            .ok_or(AgentPoolError::AgentNotFound(agent_id.to_string()))?;
        
        if !connection.is_healthy {
            // Attempt reconnection
            self.reconnect_agent(agent_id).await?;
            connection = self.connections.get_mut(agent_id).unwrap();
        }
        
        // Serialize request
        let serialized_request = serde_json::to_vec(&request)?;
        let length = serialized_request.len() as u32;
        
        // Send length prefix + request
        connection.stream.write_all(&length.to_le_bytes()).await?;
        connection.stream.write_all(&serialized_request).await?;
        
        // Read response length
        let mut length_buf = [0u8; 4];
        connection.stream.read_exact(&mut length_buf).await?;
        let response_length = u32::from_le_bytes(length_buf) as usize;
        
        // Read response
        let mut response_buf = vec![0u8; response_length];
        connection.stream.read_exact(&mut response_buf).await?;
        
        // Deserialize response
        let response: AgentResponse = serde_json::from_slice(&response_buf)?;
        
        Ok(response)
    }
    
    async fn reconnect_agent(&mut self, agent_id: &str) -> Result<(), AgentPoolError> {
        let config = self.connections.get(agent_id)
            .ok_or(AgentPoolError::AgentNotFound(agent_id.to_string()))?
            .config.clone();
        
        // Remove old connection
        self.connections.remove(agent_id);
        
        // Attempt reconnection with retries
        for attempt in 1..=self.config.retry_attempts {
            match self.connect_to_agent(&config).await {
                Ok(connection) => {
                    self.connections.insert(agent_id.to_string(), connection);
                    println!("Reconnected to agent {} on attempt {}", agent_id, attempt);
                    return Ok(());
                },
                Err(e) => {
                    eprintln!("Reconnection attempt {} failed for agent {}: {}", attempt, agent_id, e);
                    if attempt < self.config.retry_attempts {
                        tokio::time::sleep(std::time::Duration::from_secs(1)).await;
                    }
                }
            }
        }
        
        Err(AgentPoolError::ReconnectionFailed(agent_id.to_string()))
    }
    
    pub async fn health_check_all_agents(&mut self) -> HashMap<String, bool> {
        let mut results = HashMap::new();
        
        for (agent_id, _connection) in &mut self.connections {
            let health_request = AgentRequest {
                request_id: uuid::Uuid::new_v4().to_string(),
                agent_name: agent_id.clone(),
                action: "health_check".to_string(),
                params: serde_json::Value::Object(serde_json::Map::new()),
                context: RequestContext::default(),
            };
            
            match self.send_request(agent_id, health_request).await {
                Ok(response) if response.success => {
                    results.insert(agent_id.clone(), true);
                },
                _ => {
                    results.insert(agent_id.clone(), false);
                }
            }
        }
        
        results
    }
}

#[derive(Debug, Serialize, Deserialize)]
pub struct AgentRequest {
    pub request_id: String,
    pub agent_name: String,
    pub action: String,
    pub params: serde_json::Value,
    pub context: RequestContext,
}

#[derive(Debug, Serialize, Deserialize, Default)]
pub struct RequestContext {
    pub session_id: String,
    pub working_directory: String,
    pub project_name: String,
    pub timestamp: chrono::DateTime<chrono::Utc>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct AgentResponse {
    pub request_id: String,
    pub agent_name: String,
    pub success: bool,
    pub result: serde_json::Value,
    pub error: Option<String>,
    pub execution_time_ms: u64,
}

#[derive(Debug, thiserror::Error)]
pub enum AgentPoolError {
    #[error("Agent not found: {0}")]
    AgentNotFound(String),
    #[error("Connection timeout")]
    ConnectionTimeout(#[from] tokio::time::error::Elapsed),
    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),
    #[error("Serialization error: {0}")]
    Serde(#[from] serde_json::Error),
    #[error("Reconnection failed for agent: {0}")]
    ReconnectionFailed(String),
}
```

---

## 🎭 Multi-Interface Support Architecture

### **HTTP API Server (`orchestrator/src/interfaces/http_server.rs`)**
```rust
use axum::{
    extract::{Path, Query, State},
    http::StatusCode,
    response::Json,
    routing::{get, post},
    Router,
};
use serde::{Deserialize, Serialize};
use std::sync::Arc;
use tokio::sync::Mutex;

#[derive(Clone)]
pub struct HttpServerState {
    agent_pool: Arc<Mutex<AgentPool>>,
    conversation_processor: Arc<ConversationProcessor>,
}

pub fn create_http_router(state: HttpServerState) -> Router {
    Router::new()
        // Agent interaction endpoints
        .route("/api/agent/:agent_id/:action", post(execute_agent_action))
        .route("/api/agents/status", get(get_agents_status))
        .route("/api/agents/health", get(health_check_agents))
        
        // Conversation management
        .route("/api/conversation/capture", post(capture_conversation))
        .route("/api/conversation/session/:session_id", get(get_session_history))
        .route("/api/conversation/active", get(get_active_sessions))
        
        // Project management
        .route("/api/projects/detect", post(detect_project))
        .route("/api/projects/list", get(list_projects))
        .route("/api/projects/:project_id/summary", get(get_project_summary))
        
        // System health and monitoring
        .route("/api/health", get(system_health))
        .route("/api/metrics", get(system_metrics))
        
        .with_state(state)
}

async fn execute_agent_action(
    Path((agent_id, action)): Path<(String, String)>,
    State(state): State<HttpServerState>,
    Json(params): Json<serde_json::Value>,
) -> Result<Json<AgentResponse>, (StatusCode, Json<ErrorResponse>)> {
    let request = AgentRequest {
        request_id: uuid::Uuid::new_v4().to_string(),
        agent_name: agent_id,
        action,
        params,
        context: RequestContext {
            session_id: "http_client_session".to_string(),
            working_directory: std::env::current_dir()
                .unwrap_or_default()
                .to_string_lossy()
                .to_string(),
            project_name: "http_client_project".to_string(),
            timestamp: chrono::Utc::now(),
        },
    };
    
    let mut agent_pool = state.agent_pool.lock().await;
    match agent_pool.send_request(&request.agent_name, request).await {
        Ok(response) => Ok(Json(response)),
        Err(e) => Err((
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ErrorResponse {
                error: format!("Agent execution failed: {}", e),
                code: "AGENT_EXECUTION_ERROR".to_string(),
            })
        ))
    }
}

async fn capture_conversation(
    State(state): State<HttpServerState>,
    Json(payload): Json<ConversationCaptureRequest>,
) -> Result<Json<ConversationCaptureResponse>, (StatusCode, Json<ErrorResponse>)> {
    // Process conversation capture request
    let session_id = format!("http_session_{}", chrono::Utc::now().timestamp());
    
    // Forward to conversation processor
    let result = state.conversation_processor
        .process_conversation_data(payload.into_conversation_data(session_id.clone()))
        .await;
    
    match result {
        Ok(processed) => Ok(Json(ConversationCaptureResponse {
            session_id,
            processed: true,
            services_active: vec!["agent_pool".to_string()],
        })),
        Err(e) => Err((
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ErrorResponse {
                error: format!("Conversation capture failed: {}", e),
                code: "CONVERSATION_CAPTURE_ERROR".to_string(),
            })
        ))
    }
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ConversationCaptureRequest {
    pub working_dir: String,
    pub timestamp: chrono::DateTime<chrono::Utc>,
    pub conversation_data: Option<String>,
    pub session_context: Option<serde_json::Value>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ConversationCaptureResponse {
    pub session_id: String,
    pub processed: bool,
    pub services_active: Vec<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ErrorResponse {
    pub error: String,
    pub code: String,
}
```

### **WebSocket Server (`orchestrator/src/interfaces/websocket_server.rs`)**
```rust
use axum::{
    extract::{
        ws::{Message, WebSocket, WebSocketUpgrade},
        State,
    },
    response::Response,
};
use futures::{sink::SinkExt, stream::StreamExt};
use serde::{Deserialize, Serialize};
use std::sync::Arc;
use tokio::sync::Mutex;

pub async fn websocket_handler(
    ws: WebSocketUpgrade,
    State(state): State<HttpServerState>,
) -> Response {
    ws.on_upgrade(|socket| websocket_connection(socket, state))
}

async fn websocket_connection(socket: WebSocket, state: HttpServerState) {
    let (mut sender, mut receiver) = socket.split();
    let session_id = uuid::Uuid::new_v4().to_string();
    
    println!("WebSocket connection established for session: {}", session_id);
    
    // Send welcome message
    let welcome = WebSocketMessage::Connected {
        session_id: session_id.clone(),
        server_time: chrono::Utc::now(),
    };
    
    if sender.send(Message::Text(serde_json::to_string(&welcome).unwrap())).await.is_err() {
        return;
    }
    
    // Handle incoming messages
    while let Some(msg) = receiver.next().await {
        match msg {
            Ok(Message::Text(text)) => {
                match serde_json::from_str::<WebSocketMessage>(&text) {
                    Ok(ws_message) => {
                        let response = handle_websocket_message(ws_message, &state, &session_id).await;
                        if let Ok(response_text) = serde_json::to_string(&response) {
                            if sender.send(Message::Text(response_text)).await.is_err() {
                                break;
                            }
                        }
                    },
                    Err(e) => {
                        let error_response = WebSocketMessage::Error {
                            message: format!("Invalid message format: {}", e),
                            code: "INVALID_MESSAGE".to_string(),
                        };
                        if let Ok(error_text) = serde_json::to_string(&error_response) {
                            if sender.send(Message::Text(error_text)).await.is_err() {
                                break;
                            }
                        }
                    }
                }
            },
            Ok(Message::Close(_)) => {
                println!("WebSocket connection closed for session: {}", session_id);
                break;
            },
            _ => {}
        }
    }
}

async fn handle_websocket_message(
    message: WebSocketMessage,
    state: &HttpServerState,
    session_id: &str,
) -> WebSocketMessage {
    match message {
        WebSocketMessage::AgentRequest { agent_id, action, params } => {
            let request = AgentRequest {
                request_id: uuid::Uuid::new_v4().to_string(),
                agent_name: agent_id.clone(),
                action: action.clone(),
                params,
                context: RequestContext {
                    session_id: session_id.to_string(),
                    working_directory: "/tmp".to_string(), // WebSocket clients don't have working dir
                    project_name: "websocket_project".to_string(),
                    timestamp: chrono::Utc::now(),
                },
            };
            
            let mut agent_pool = state.agent_pool.lock().await;
            match agent_pool.send_request(&agent_id, request).await {
                Ok(response) => WebSocketMessage::AgentResponse { response },
                Err(e) => WebSocketMessage::Error {
                    message: format!("Agent request failed: {}", e),
                    code: "AGENT_REQUEST_ERROR".to_string(),
                }
            }
        },
        WebSocketMessage::ConversationCapture { content } => {
            // Process conversation content
            WebSocketMessage::ConversationCaptured {
                session_id: session_id.to_string(),
                processed: true,
            }
        },
        _ => WebSocketMessage::Error {
            message: "Unsupported message type".to_string(),
            code: "UNSUPPORTED_MESSAGE".to_string(),
        }
    }
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(tag = "type")]
pub enum WebSocketMessage {
    Connected {
        session_id: String,
        server_time: chrono::DateTime<chrono::Utc>,
    },
    AgentRequest {
        agent_id: String,
        action: String,
        params: serde_json::Value,
    },
    AgentResponse {
        response: AgentResponse,
    },
    ConversationCapture {
        content: String,
    },
    ConversationCaptured {
        session_id: String,
        processed: bool,
    },
    Error {
        message: String,
        code: String,
    },
}
```

---

## 🧪 Testing Strategy and Implementation

### **Two-Account Testing Environment**

#### **Account 1: Current System Preservation**
```bash
# /Users/larrydiffey/projects/CenterfireIntelligence/testing-environments/account-1-current/
# test-session-setup.md

## Current System Testing Protocol

### Prerequisites
- Existing Redis agents running (AGT-NAMING-1, AGT-STRUCT-1, etc.)
- Current Claude Code hooks operational
- All existing functionality preserved

### Test Procedures
1. **Baseline Functionality Test**
   ```bash
   cd /Users/larrydiffey/projects/CenterfireIntelligence
   # Verify agents are running
   ps aux | grep "go run main.go"
   # Test naming agent
   echo "Testing current naming system..."
   # Record conversation quality and agent responsiveness
   ```

2. **Conversation Capture Quality Assessment**
   ```bash
   # Monitor Redis streams
   redis-cli -p 6380 XREAD STREAMS centerfire:semantic:names $
   # Document conversation capture completeness
   ```

3. **Performance Baseline**
   ```bash
   # Measure current system performance
   time <agent_operation>
   # Document response times and resource usage
   ```
```

#### **Account 2: PTY Proxy Testing Environment**
```bash
# /Users/testaccount/projects/CenterfireIntelligence/testing-environments/account-2-pty-proxy/
# proxy-session-setup.md

## PTY Proxy Testing Protocol

### Prerequisites
- Fresh CenterfireIntelligence clone
- Rust orchestrator compiled and ready
- Go agents configured for dual-mode (Redis + Socket)

### Test Procedures
1. **PTY Proxy Basic Functionality**
   ```bash
   cd /Users/testaccount/projects/CenterfireIntelligence/orchestrator
   cargo run -- --mode pty-proxy --claude-command claude-code
   # Verify PTY interception works
   # Verify conversation capture to log files
   ```

2. **Socket Communication Test**
   ```bash
   # Start orchestrator
   ./target/release/orchestrator --config config/test.yaml
   
   # Start agents in socket mode
   cd ../agents/AGT-NAMING-1__01K4EAF1
   SOCKET_MODE=true go run main.go &
   
   # Test socket communication
   curl -X POST http://localhost:8080/api/agent/naming/allocate \
     -H "Content-Type: application/json" \
     -d '{"domain":"TEST","purpose":"socket testing"}'
   ```

3. **Multi-Interface Testing**
   ```bash
   # Test HTTP API
   curl http://localhost:8080/api/agents/status
   
   # Test WebSocket (using wscat)
   wscat -c ws://localhost:8080/ws
   ```
```

### **Integration Testing Framework**
```bash
# /Users/larrydiffey/projects/CenterfireIntelligence/testing-environments/integration-tests/

## Cross-System Validation Tests

### Socket Communication Tests
tests/
├── socket-communication/
│   ├── agent-connection-test.rs
│   ├── request-response-cycle-test.rs
│   ├── error-handling-test.rs
│   └── performance-benchmark-test.rs

### PTY Interception Tests  
├── pty-interception/
│   ├── basic-io-capture-test.rs
│   ├── conversation-completeness-test.rs
│   ├── session-management-test.rs
│   └── claude-response-parsing-test.rs

### Multi-Interface Tests
└── multi-interface/
    ├── concurrent-client-test.rs
    ├── http-api-compatibility-test.rs
    ├── websocket-realtime-test.rs
    └── load-testing-suite.rs
```

---

## 🚀 Implementation Timeline and Migration Path

### **Phase 1: Foundation (Weeks 1-2)**
1. **Directory Structure Creation**
   - Create complete directory structure as specified
   - Set up Rust workspace with proper Cargo.toml
   - Initialize testing environments for both accounts

2. **PTY Proxy Proof-of-Concept**
   - Implement basic PTY proxy with `portable-pty`
   - Test conversation capture to log files
   - Verify transparency to Claude Code

3. **Socket Infrastructure**  
   - Implement Unix socket management in Rust
   - Add socket listeners to existing Go agents (dual-mode)
   - Test basic socket communication

### **Phase 2: Core Functionality (Weeks 3-4)**  
1. **Agent Pool Management**
   - Complete agent pool implementation
   - Add health checking and reconnection logic
   - Implement request routing and response processing

2. **Conversation Processing**
   - Build conversation capture engine
   - Add semantic extraction capabilities
   - Integrate with existing storage systems

3. **Basic HTTP API**
   - Implement core HTTP endpoints
   - Add agent interaction APIs
   - Test compatibility with current Claude Code hooks

### **Phase 3: Multi-Interface Support (Weeks 5-6)**
1. **WebSocket Server**
   - Implement real-time WebSocket interface
   - Build web client proof-of-concept
   - Test concurrent multi-client scenarios

2. **Advanced PTY Features**
   - Add intelligent response parsing
   - Implement command interception capabilities
   - Build agent orchestration based on Claude responses

3. **Integration Testing**
   - Complete cross-system validation
   - Performance benchmarking and optimization
   - Documentation and deployment guides

### **Phase 4: Production Readiness (Weeks 7-8)**
1. **Monitoring and Observability**
   - Add comprehensive logging
   - Implement metrics collection
   - Build health monitoring dashboards

2. **Security and Robustness**
   - Add error recovery mechanisms
   - Implement security measures for multi-interface access
   - Stress testing and reliability improvements

3. **Migration Preparation**
   - Prepare migration tools and documentation
   - Create rollback procedures
   - Final validation and deployment

---

## 🎯 Success Metrics and Validation Criteria

### **Technical Success Criteria**
1. **PTY Proxy Reliability**
   - 100% conversation capture rate (no missed I/O)
   - < 10ms latency overhead for PTY interception
   - Zero data corruption or loss during proxy operation

2. **Socket Communication Performance**
   - < 5ms average response time for agent socket communication
   - 99.9% uptime for agent socket connections
   - Automatic recovery from connection failures within 1 second

3. **Multi-Interface Compatibility**
   - Simultaneous support for 10+ concurrent clients
   - API response times < 100ms for 95th percentile
   - WebSocket real-time communication with < 50ms latency

### **Functional Success Criteria**
1. **Feature Parity**
   - All current agent functionality available through new interfaces
   - Conversation quality equivalent to current Redis-based system
   - No loss of semantic storage or processing capabilities

2. **Operational Excellence** 
   - Zero-downtime deployment capability
   - Complete rollback capability to current system
   - Comprehensive monitoring and alerting

3. **Development Efficiency**
   - Clear separation between temporary and permanent components
   - Easy future removal of Claude Code direct editing functionality
   - Maintainable and extensible architecture

---

## 📋 Risk Mitigation and Contingency Planning

### **High-Risk Areas and Mitigation Strategies**

#### **PTY Proxy Stability Risk**
- **Risk**: PTY interception could be unreliable or cause Claude Code instability
- **Mitigation**: Extensive testing in isolated environment, fallback to HTTP hook system
- **Contingency**: Maintain current hook-based system as backup

#### **Socket Communication Reliability Risk**  
- **Risk**: Unix socket connections could fail or become unstable
- **Mitigation**: Robust error handling, automatic reconnection, health monitoring
- **Contingency**: HTTP-based agent communication as fallback

#### **Performance Degradation Risk**
- **Risk**: New architecture could be slower than current direct agent access
- **Mitigation**: Comprehensive benchmarking, performance optimization
- **Contingency**: Performance-based rollback triggers

#### **Migration Complexity Risk**
- **Risk**: Migration from current system could be disruptive
- **Mitigation**: Dual-mode operation, gradual transition, extensive testing
- **Contingency**: Immediate rollback capability to current system

### **Rollback Strategy**
1. **Level 1**: Disable new interfaces, maintain current system
2. **Level 2**: Revert agents to Redis-only mode  
3. **Level 3**: Complete restoration of pre-migration state
4. **Level 4**: Emergency procedure for critical system recovery

---

## 🎉 Expected Outcomes and Future Capabilities

### **Immediate Benefits**
1. **Complete Conversation Capture**: Full visibility into Claude Code interactions
2. **Multi-Interface Support**: Web, API, and terminal access to agent system  
3. **Improved Reliability**: Robust error handling and automatic recovery
4. **Enhanced Monitoring**: Comprehensive system health and performance visibility

### **Long-Term Capabilities**
1. **Agent Orchestration**: Intelligent routing of tasks to appropriate agents
2. **Context-Aware Processing**: Leveraging complete conversation history
3. **Scalable Architecture**: Support for additional interfaces and client types
4. **Advanced Analytics**: Deep insights into development patterns and efficiency

### **Future Extension Points**
1. **LLM Integration**: Direct integration with Claude API for background processing
2. **Advanced Automation**: Intelligent automation based on conversation patterns
3. **Collaborative Features**: Multi-user and team collaboration capabilities  
4. **Enterprise Integration**: Integration with enterprise development tools

---

**This implementation plan provides a comprehensive roadmap for transitioning to a PTY proxy-based multi-interface architecture while maintaining operational continuity and enabling future capabilities. The plan emphasizes careful separation of concerns to enable easy removal of temporary functionality once the full orchestration system is operational.**

---

*Document Status: Implementation Ready*  
*Next Steps: Begin Phase 1 implementation with directory structure creation and PTY proxy proof-of-concept*