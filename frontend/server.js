const express = require('express');
const { exec } = require('child_process');
const { promisify } = require('util');
const axios = require('axios');
const redis = require('redis');
const path = require('path');

const execAsync = promisify(exec);
const app = express();
const PORT = 9191; // High port to avoid conflicts with other services

// Serve static files
app.use(express.static(path.join(__dirname, 'public')));

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

// Main health check endpoint
app.get('/api/health', async (req, res) => {
    const startTime = Date.now();
    
    try {
        const [containers, agents, redis, endpoints] = await Promise.all([
            checkDockerContainers(),
            checkAgentProcesses(), 
            checkRedisHealth(),
            checkServiceEndpoints()
        ]);

        const healthData = {
            timestamp: new Date().toISOString(),
            checkDuration: Date.now() - startTime,
            containers,
            agents,
            redis,
            endpoints,
            summary: {
                containersRunning: containers.filter(c => c.running).length,
                containersExpected: EXPECTED_CONTAINERS.length,
                agentsRunning: agents.filter(a => a.running).length,
                agentsExpected: EXPECTED_AGENTS.length,
                redisConnected: redis.connected,
                endpointsAccessible: endpoints.filter(e => e.accessible).length
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
    res.sendFile(path.join(__dirname, 'public', 'index.html'));
});

app.listen(PORT, () => {
    console.log(`🏥 Centerfire Health Monitor running at http://localhost:${PORT}`);
    console.log(`📊 Health API available at http://localhost:${PORT}/api/health`);
    console.log(`📝 Note: This is the first of several monitoring tools planned`);
});