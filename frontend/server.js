const express = require('express');
const { exec } = require('child_process');
const { promisify } = require('util');
const axios = require('axios');
const redis = require('redis');
const path = require('path');

const execAsync = promisify(exec);
const app = express();
const PORT = 9191; // High port to avoid conflicts with other services

// Serve static files (prefer built Vite app if present)
const viteDist = path.join(__dirname, 'web', 'dist')
const publicDir = path.join(__dirname, 'public')
const fs = require('fs')
if (fs.existsSync(viteDist)) {
  console.log('Serving static from web/dist (Vite build)')
  app.use(express.static(viteDist))
} else {
  console.log('Serving static from public (legacy)')
  app.use(express.static(publicDir))
}

// Expected containers
const EXPECTED_CONTAINERS = [
    { name: 'mem0-redis', ports: ['6380'] },
    { name: 'centerfire-weaviate', ports: ['8080'] },
    { name: 'centerfire-neo4j', ports: ['7474', '7687'] },
    { name: 'centerfire-clickhouse', ports: ['8123', '9001'] },
    { name: 'centerfire-casbin', ports: ['50051'] },
    { name: 'centerfire-transformers', ports: [] }
];

// Expected agents (should be running processes)
const EXPECTED_AGENTS = [
    'AGT-NAMING-1',
    'AGT-CONTEXT-1', 
    'AGT-MANAGER-1',
    'AGT-SYSTEM-COMMANDER-1',
    'AGT-CLAUDE-CAPTURE-1',
    'AGT-STACK-1'
];

async function checkDockerContainers() {
    try {
        const { stdout } = await execAsync('docker ps --format "{{.Names}}\t{{.Status}}\t{{.Ports}}"');
        const containers = stdout.trim().split('\n').map(line => {
            const [name, status, ports] = line.split('\t');
            return { name, status, ports };
        });

        return EXPECTED_CONTAINERS.map(expected => {
            const running = containers.find(c => c.name === expected.name);
            return {
                name: expected.name,
                expected: true,
                running: !!running,
                status: running ? running.status : 'Not running',
                ports: running ? running.ports : 'N/A',
                expectedPorts: expected.ports
            };
        });
    } catch (error) {
        return { error: `Failed to check containers: ${error.message}` };
    }
}

async function checkAgentProcesses() {
    try {
        const { stdout } = await execAsync('ps aux');
        
        return EXPECTED_AGENTS.map(agentName => {
            const isRunning = stdout.includes(agentName);
            return {
                name: agentName,
                expected: true,
                running: isRunning,
                status: isRunning ? 'Running' : 'Not running'
            };
        });
    } catch (error) {
        return { error: `Failed to check processes: ${error.message}` };
    }
}

async function checkRedisHealth() {
    try {
        const client = redis.createClient({ 
            host: 'localhost', 
            port: 6380,
            socket: { connectTimeout: 5000 }
        });
        
        await client.connect();
        
        // Check basic connectivity
        const pingResult = await client.ping();
        
        // Check key streams
        const conversationLength = await client.xLen('centerfire:semantic:conversations');
        const namesLength = await client.xLen('centerfire:semantic:names');
        
        await client.disconnect();
        
        return {
            connected: true,
            ping: pingResult,
            streams: {
                'centerfire:semantic:conversations': conversationLength,
                'centerfire:semantic:names': namesLength
            }
        };
    } catch (error) {
        return {
            connected: false,
            error: error.message
        };
    }
}

async function checkServiceEndpoints() {
    const endpoints = [
        { name: 'Weaviate', url: 'http://localhost:8080/v1/meta', timeout: 5000 },
        { name: 'Neo4j', url: 'http://localhost:7474', timeout: 5000 },
        { name: 'ClickHouse', url: 'http://localhost:8123/ping', timeout: 5000 }
    ];

    const results = await Promise.all(
        endpoints.map(async (endpoint) => {
            try {
                const response = await axios.get(endpoint.url, { 
                    timeout: endpoint.timeout,
                    validateStatus: () => true // Accept any status
                });
                return {
                    name: endpoint.name,
                    url: endpoint.url,
                    status: response.status,
                    accessible: response.status < 500,
                    responseTime: response.headers['x-response-time'] || 'N/A'
                };
            } catch (error) {
                return {
                    name: endpoint.name,
                    url: endpoint.url,
                    accessible: false,
                    error: error.code || error.message
                };
            }
        })
    );

    return results;
}

async function checkGitStatus() {
    try {
        const [statusResult, branchResult] = await Promise.all([
            execAsync('git status --porcelain'),
            execAsync('git branch --show-current')
        ]);

        const files = statusResult.stdout.trim();
        const currentBranch = branchResult.stdout.trim();
        
        // Parse git status output
        const changes = files ? files.split('\n').map(line => {
            const status = line.substring(0, 2);
            const file = line.substring(3);
            return { status: status.trim(), file };
        }) : [];

        // Get ahead/behind info
        let aheadBehind = null;
        try {
            const { stdout } = await execAsync(`git rev-list --count --left-right origin/${currentBranch}...HEAD 2>/dev/null || echo "0	0"`);
            const [behind, ahead] = stdout.trim().split('\t').map(Number);
            aheadBehind = { ahead, behind };
        } catch (e) {
            // Ignore if no remote tracking
        }

        return {
            branch: currentBranch,
            clean: changes.length === 0,
            changes: changes.length,
            files: changes,
            aheadBehind,
            lastCheck: new Date().toISOString()
        };
    } catch (error) {
        return {
            error: error.message,
            available: false
        };
    }
}

// Main health check endpoint
app.get('/api/health', async (req, res) => {
    const startTime = Date.now();
    
    try {
        const [containers, agents, redis, endpoints, git] = await Promise.all([
            checkDockerContainers(),
            checkAgentProcesses(), 
            checkRedisHealth(),
            checkServiceEndpoints(),
            checkGitStatus()
        ]);

        const healthData = {
            timestamp: new Date().toISOString(),
            checkDuration: Date.now() - startTime,
            containers,
            agents,
            redis,
            endpoints,
            git,
            summary: {
                containersRunning: containers.filter(c => c.running).length,
                containersExpected: EXPECTED_CONTAINERS.length,
                agentsRunning: agents.filter(a => a.running).length,
                agentsExpected: EXPECTED_AGENTS.length,
                redisConnected: redis.connected,
                endpointsAccessible: endpoints.filter(e => e.accessible).length,
                gitClean: git.clean || false,
                gitChanges: git.changes || 0,
                gitBranch: git.branch || 'unknown'
            }
        };

        res.json(healthData);
    } catch (error) {
        res.status(500).json({
            error: 'Health check failed',
            message: error.message,
            timestamp: new Date().toISOString()
        });
    }
});

// Serve the main page
app.get('/', (req, res) => {
    if (fs.existsSync(path.join(viteDist, 'index.html'))) {
        res.sendFile(path.join(viteDist, 'index.html'))
    } else {
        res.sendFile(path.join(publicDir, 'index.html'))
    }
});

app.listen(PORT, () => {
    console.log(`🏥 Centerfire Health Monitor running at http://localhost:${PORT}`);
    console.log(`📊 Health API available at http://localhost:${PORT}/api/health`);
    console.log(`📝 Note: This is the first of several monitoring tools planned`);
});
