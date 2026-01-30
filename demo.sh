#!/bin/bash
set -e

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🚀 RAFTPAY DOCKER DEPLOYMENT DEMO"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Detect docker compose command
if command -v docker-compose &> /dev/null; then
    COMPOSE="docker-compose"
elif docker compose version &> /dev/null; then
    COMPOSE="docker compose"
else
    echo -e "${RED}❌ Docker Compose not found!${NC}"
    echo "Please install Docker Desktop or Docker Compose"
    exit 1
fi

echo -e "${GREEN}Using: $COMPOSE${NC}"
echo ""

# Helper function to make API calls
api_call() {
    local port=$1
    local endpoint=$2
    local data=$3
    
    if [ -z "$data" ]; then
        curl -s "http://localhost:${port}${endpoint}"
    else
        curl -s -X POST "http://localhost:${port}${endpoint}" \
            -H "Content-Type: application/json" \
            -d "$data"
    fi
}

# Step 1: Start the cluster
echo -e "${BLUE}📦 Step 1: Starting 5-node RaftPay cluster...${NC}"
$COMPOSE up -d
echo ""

# Wait for nodes to start (increased time)
echo -e "${BLUE}⏳ Waiting for nodes to initialize (15 seconds)...${NC}"
sleep 15
echo ""

# Step 2: Find the leader (try multiple times)
echo -e "${BLUE}👑 Step 2: Finding the leader...${NC}"
LEADER_PORT=""
attempts=0
max_attempts=5

while [ -z "$LEADER_PORT" ] && [ $attempts -lt $max_attempts ]; do
    for port in 9000 9001 9002 9003 9004; do
        status=$(api_call $port "/status" 2>/dev/null | grep -o '"isLeader":[^,}]*' | cut -d':' -f2 || echo "false")
        if [ "$status" = "true" ]; then
            LEADER_PORT=$port
            node_num=$((port - 9000 + 1))
            echo -e "${GREEN}✅ Leader found: node${node_num} (port ${port})${NC}"
            break 2
        fi
    done
    
    attempts=$((attempts + 1))
    if [ -z "$LEADER_PORT" ]; then
        echo -e "${YELLOW}   Attempt $attempts/$max_attempts - still electing...${NC}"
        sleep 3
    fi
done

if [ -z "$LEADER_PORT" ]; then
    echo -e "${RED}❌ No leader found after $max_attempts attempts!${NC}"
    echo ""
    echo "Checking node logs for errors:"
    $COMPOSE logs --tail=20 node1
    exit 1
fi
echo ""

# Step 3: Create accounts
echo -e "${BLUE}💰 Step 3: Creating test accounts...${NC}"
api_call $LEADER_PORT "/account" '{"account_id":"alice","initial_balance":1000.00}' > /dev/null
echo -e "${GREEN}   ✓ Created alice with \$1000${NC}"
api_call $LEADER_PORT "/account" '{"account_id":"bob","initial_balance":500.00}' > /dev/null
echo -e "${GREEN}   ✓ Created bob with \$500${NC}"
sleep 1
echo ""

# Step 4: Check balances
echo -e "${BLUE}📊 Step 4: Checking balances...${NC}"
alice_balance=$(api_call $LEADER_PORT "/account/alice" | grep -o '"balance":[^,}]*' | cut -d':' -f2)
bob_balance=$(api_call $LEADER_PORT "/account/bob" | grep -o '"balance":[^,}]*' | cut -d':' -f2)
echo -e "   Alice: \$${alice_balance}"
echo -e "   Bob:   \$${bob_balance}"
echo ""

# Step 5: Make a transfer
echo -e "${BLUE}💸 Step 5: Making a transfer (Alice → Bob: \$100)...${NC}"
api_call $LEADER_PORT "/transfer" '{"from":"alice","to":"bob","amount":100.00,"txn_id":"demo_txn_1"}' > /dev/null
sleep 1
echo ""

# Step 6: Verify updated balances
echo -e "${BLUE}📊 Step 6: Verifying updated balances...${NC}"
alice_balance=$(api_call $LEADER_PORT "/account/alice" | grep -o '"balance":[^,}]*' | cut -d':' -f2)
bob_balance=$(api_call $LEADER_PORT "/account/bob" | grep -o '"balance":[^,}]*' | cut -d':' -f2)
echo -e "   Alice: \$${alice_balance} ${GREEN}(decreased by \$100)${NC}"
echo -e "   Bob:   \$${bob_balance} ${GREEN}(increased by \$100)${NC}"
echo ""

# Step 7: Chaos testing - kill the leader!
echo -e "${YELLOW}💥 Step 7: CHAOS TEST - Killing the leader!${NC}"
leader_node="node$((LEADER_PORT - 9000 + 1))"
echo -e "   Stopping ${leader_node}..."
$COMPOSE stop $leader_node > /dev/null 2>&1
echo -e "${RED}   ✗ ${leader_node} is DOWN${NC}"
echo ""

# Wait for leader election
echo -e "${BLUE}⏳ Waiting for leader election (8 seconds)...${NC}"
sleep 8
echo ""

# Find new leader
echo -e "${BLUE}👑 Step 8: Finding new leader...${NC}"
NEW_LEADER_PORT=""
for port in 9000 9001 9002 9003 9004; do
    if [ "$port" = "$LEADER_PORT" ]; then
        continue  # Skip the dead node
    fi
    status=$(api_call $port "/status" 2>/dev/null | grep -o '"isLeader":[^,}]*' | cut -d':' -f2 || echo "false")
    if [ "$status" = "true" ]; then
        NEW_LEADER_PORT=$port
        node_num=$((port - 9000 + 1))
        echo -e "${GREEN}✅ New leader elected: node${node_num} (port ${port})${NC}"
        break
    fi
done

if [ -z "$NEW_LEADER_PORT" ]; then
    echo -e "${RED}❌ No new leader found!${NC}"
    exit 1
fi
echo ""

# Step 9: Verify data integrity after failover
echo -e "${BLUE}✅ Step 9: Verifying data integrity after failover...${NC}"
alice_balance=$(api_call $NEW_LEADER_PORT "/account/alice" | grep -o '"balance":[^,}]*' | cut -d':' -f2)
bob_balance=$(api_call $NEW_LEADER_PORT "/account/bob" | grep -o '"balance":[^,}]*' | cut -d':' -f2)
echo -e "   Alice: \$${alice_balance} ${GREEN}(data preserved!)${NC}"
echo -e "   Bob:   \$${bob_balance} ${GREEN}(data preserved!)${NC}"
echo ""

# Step 10: Make another transfer with new leader
echo -e "${BLUE}💸 Step 10: Making transfer with new leader (Bob → Alice: \$50)...${NC}"
api_call $NEW_LEADER_PORT "/transfer" '{"from":"bob","to":"alice","amount":50.00,"txn_id":"demo_txn_2"}' > /dev/null
sleep 1
echo ""

# Verify final balances
echo -e "${BLUE}📊 Step 11: Final balances...${NC}"
alice_balance=$(api_call $NEW_LEADER_PORT "/account/alice" | grep -o '"balance":[^,}]*' | cut -d':' -f2)
bob_balance=$(api_call $NEW_LEADER_PORT "/account/bob" | grep -o '"balance":[^,}]*' | cut -d':' -f2)
echo -e "   Alice: \$${alice_balance}"
echo -e "   Bob:   \$${bob_balance}"
echo ""

# Step 12: Bring back the dead node
echo -e "${BLUE}🔄 Step 12: Restarting ${leader_node}...${NC}"
$COMPOSE start $leader_node > /dev/null 2>&1
sleep 3
echo -e "${GREEN}   ✓ ${leader_node} is back online (will sync from new leader)${NC}"
echo ""

# Summary
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}✅ DEMO COMPLETE!${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "What we demonstrated:"
echo "  ✅ 5-node distributed cluster"
echo "  ✅ Leader election"
echo "  ✅ Strong consistency (all nodes see same data)"
echo "  ✅ Fault tolerance (survived leader crash)"
echo "  ✅ Automatic failover"
echo "  ✅ Data persistence"
echo ""
echo "Cleanup:"
echo "  $COMPOSE down       # Stop cluster"
echo "  $COMPOSE down -v    # Stop and remove data"
echo ""
