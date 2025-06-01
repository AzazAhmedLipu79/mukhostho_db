#!/bin/bash

# Mukhostho DB Load Testing Script

# Configuration
SERVER_URL="${MUKHOSTHO_URL:-http://localhost:8080}"
NUM_REQUESTS="${NUM_REQUESTS:-10000}"
CONCURRENCY="${CONCURRENCY:-50}"
TEST_DURATION="${TEST_DURATION:-60s}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if server is running
check_server() {
    print_status "Checking if server is running at $SERVER_URL"
    
    if curl -s -f "$SERVER_URL/health" > /dev/null; then
        print_success "Server is running"
        return 0
    else
        print_error "Server is not running or not accessible"
        exit 1
    fi
}

# Install dependencies
install_deps() {
    print_status "Checking dependencies..."
    
    # Check for curl
    if ! command -v curl &> /dev/null; then
        print_error "curl is required but not installed"
        exit 1
    fi
    
    # Check for jq
    if ! command -v jq &> /dev/null; then
        print_warning "jq is not installed, some features will be limited"
    fi
    
    # Check for apache bench
    if ! command -v ab &> /dev/null; then
        print_warning "Apache Bench (ab) is not installed, will use curl for testing"
    fi
    
    # Check for wrk
    if ! command -v wrk &> /dev/null; then
        print_warning "wrk is not installed, will use alternative methods"
    fi
}

# Basic functionality test
test_basic_functionality() {
    print_status "Testing basic functionality..."
    
    # Test SET
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$SERVER_URL/api/v1/keys/test_key" \
        -H "Content-Type: application/json" \
        -d '{"value": "test_value"}')
    
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    BODY=$(echo "$RESPONSE" | head -n -1)
    
    if [ "$HTTP_CODE" = "200" ]; then
        print_success "SET operation working"
    else
        print_error "SET operation failed with code $HTTP_CODE"
        echo "$BODY"
        exit 1
    fi
    
    # Test GET
    RESPONSE=$(curl -s -w "\n%{http_code}" "$SERVER_URL/api/v1/keys/test_key")
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    BODY=$(echo "$RESPONSE" | head -n -1)
    
    if [ "$HTTP_CODE" = "200" ]; then
        if echo "$BODY" | grep -q "test_value"; then
            print_success "GET operation working"
        else
            print_error "GET operation returned unexpected value"
            echo "$BODY"
            exit 1
        fi
    else
        print_error "GET operation failed with code $HTTP_CODE"
        echo "$BODY"
        exit 1
    fi
    
    # Test DELETE
    RESPONSE=$(curl -s -w "\n%{http_code}" -X DELETE "$SERVER_URL/api/v1/keys/test_key")
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    
    if [ "$HTTP_CODE" = "200" ]; then
        print_success "DELETE operation working"
    else
        print_error "DELETE operation failed with code $HTTP_CODE"
        exit 1
    fi
}

# Load test with curl
load_test_curl() {
    print_status "Running load test with curl (concurrent: $CONCURRENCY, requests: $NUM_REQUESTS)"
    
    START_TIME=$(date +%s)
    
    # Create a temporary directory for curl outputs
    TEMP_DIR=$(mktemp -d)
    
    # Function to run a single request
    run_request() {
        local id=$1
        local result_file="$TEMP_DIR/result_$id"
        
        curl -s -w "%{http_code},%{time_total},%{time_connect},%{time_starttransfer}\n" \
            -X POST "$SERVER_URL/api/v1/keys/load_test_$id" \
            -H "Content-Type: application/json" \
            -d "{\"value\": \"load_test_value_$id\"}" \
            -o /dev/null > "$result_file"
    }
    
    # Run concurrent requests
    for ((i=1; i<=NUM_REQUESTS; i++)); do
        if (( $(jobs -r | wc -l) >= CONCURRENCY )); then
            wait -n # Wait for any background job to complete
        fi
        run_request $i &
    done
    
    # Wait for all background jobs to complete
    wait
    
    END_TIME=$(date +%s)
    TOTAL_TIME=$((END_TIME - START_TIME))
    
    # Analyze results
    SUCCESS_COUNT=0
    ERROR_COUNT=0
    TOTAL_RESPONSE_TIME=0
    MIN_TIME=999999
    MAX_TIME=0
    
    for file in "$TEMP_DIR"/result_*; do
        if [ -f "$file" ]; then
            line=$(cat "$file")
            http_code=$(echo "$line" | cut -d',' -f1)
            response_time=$(echo "$line" | cut -d',' -f2)
            
            if [ "$http_code" = "200" ]; then
                SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
                TOTAL_RESPONSE_TIME=$(echo "$TOTAL_RESPONSE_TIME + $response_time" | bc -l)
                
                # Calculate min/max
                if (( $(echo "$response_time < $MIN_TIME" | bc -l) )); then
                    MIN_TIME=$response_time
                fi
                if (( $(echo "$response_time > $MAX_TIME" | bc -l) )); then
                    MAX_TIME=$response_time
                fi
            else
                ERROR_COUNT=$((ERROR_COUNT + 1))
            fi
        fi
    done
    
    # Calculate statistics
    if [ $SUCCESS_COUNT -gt 0 ]; then
        AVG_RESPONSE_TIME=$(echo "scale=4; $TOTAL_RESPONSE_TIME / $SUCCESS_COUNT" | bc -l)
    else
        AVG_RESPONSE_TIME=0
    fi
    
    REQUESTS_PER_SECOND=$(echo "scale=2; $NUM_REQUESTS / $TOTAL_TIME" | bc -l)
    
    # Cleanup
    rm -rf "$TEMP_DIR"
    
    # Print results
    echo ""
    print_success "Load test completed"
    echo "Total requests: $NUM_REQUESTS"
    echo "Successful requests: $SUCCESS_COUNT"
    echo "Failed requests: $ERROR_COUNT"
    echo "Total time: ${TOTAL_TIME}s"
    echo "Requests per second: $REQUESTS_PER_SECOND"
    echo "Average response time: ${AVG_RESPONSE_TIME}s"
    echo "Min response time: ${MIN_TIME}s"
    echo "Max response time: ${MAX_TIME}s"
}

# Load test with Apache Bench
load_test_ab() {
    if ! command -v ab &> /dev/null; then
        print_warning "Apache Bench not available, skipping ab test"
        return
    fi
    
    print_status "Running load test with Apache Bench"
    
    # Create a POST data file
    echo '{"value": "ab_test_value"}' > /tmp/ab_data.json
    
    ab -n "$NUM_REQUESTS" -c "$CONCURRENCY" -p /tmp/ab_data.json -T application/json \
        "$SERVER_URL/api/v1/keys/ab_test" || print_error "Apache Bench test failed"
    
    # Cleanup
    rm -f /tmp/ab_data.json
}

# Load test with wrk
load_test_wrk() {
    if ! command -v wrk &> /dev/null; then
        print_warning "wrk not available, skipping wrk test"
        return
    fi
    
    print_status "Running load test with wrk"
    
    # Create a Lua script for POST requests
    cat > /tmp/wrk_script.lua << 'EOF'
wrk.method = "POST"
wrk.body   = '{"value": "wrk_test_value"}'
wrk.headers["Content-Type"] = "application/json"

request = function()
    path = "/api/v1/keys/wrk_test_" .. math.random(1, 10000)
    return wrk.format(nil, path)
end
EOF
    
    wrk -t4 -c"$CONCURRENCY" -d"$TEST_DURATION" -s /tmp/wrk_script.lua "$SERVER_URL" || \
        print_error "wrk test failed"
    
    # Cleanup
    rm -f /tmp/wrk_script.lua
}

# Test transactions
test_transactions() {
    print_status "Testing transaction functionality..."
    
    # Begin transaction
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$SERVER_URL/api/v1/transactions" \
        -H "Content-Type: application/json")
    
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    BODY=$(echo "$RESPONSE" | head -n -1)
    
    if [ "$HTTP_CODE" != "200" ]; then
        print_error "Failed to begin transaction"
        return
    fi
    
    # Extract transaction ID
    if command -v jq &> /dev/null; then
        TXN_ID=$(echo "$BODY" | jq -r '.transaction_id')
    else
        TXN_ID=$(echo "$BODY" | grep -o '"transaction_id":"[^"]*"' | cut -d'"' -f4)
    fi
    
    if [ -z "$TXN_ID" ]; then
        print_error "Failed to extract transaction ID"
        return
    fi
    
    # Set value in transaction
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$SERVER_URL/api/v1/keys/txn_test" \
        -H "Content-Type: application/json" \
        -d "{\"value\": \"txn_value\", \"transaction_id\": \"$TXN_ID\"}")
    
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    
    if [ "$HTTP_CODE" != "200" ]; then
        print_error "Failed to set value in transaction"
        return
    fi
    
    # Commit transaction
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$SERVER_URL/api/v1/transactions/$TXN_ID" \
        -H "Content-Type: application/json" \
        -d '{"action": "commit"}')
    
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    
    if [ "$HTTP_CODE" = "200" ]; then
        print_success "Transaction test passed"
    else
        print_error "Failed to commit transaction"
    fi
}

# Memory stress test
stress_test_memory() {
    print_status "Running memory stress test..."
    
    # Set large values to test memory limits
    for i in {1..1000}; do
        LARGE_VALUE=$(head -c 10240 /dev/urandom | base64) # 10KB value
        curl -s -X POST "$SERVER_URL/api/v1/keys/stress_$i" \
            -H "Content-Type: application/json" \
            -d "{\"value\": \"$LARGE_VALUE\"}" > /dev/null
        
        if [ $((i % 100)) -eq 0 ]; then
            echo -n "."
        fi
    done
    
    echo ""
    print_success "Memory stress test completed"
}

# TTL functionality test
test_ttl() {
    print_status "Testing TTL functionality..."
    
    # Set key with 2 second TTL
    curl -s -X POST "$SERVER_URL/api/v1/keys/ttl_test" \
        -H "Content-Type: application/json" \
        -d '{"value": "ttl_value", "ttl": 2}' > /dev/null
    
    # Check if key exists immediately
    RESPONSE=$(curl -s "$SERVER_URL/api/v1/keys/ttl_test")
    if echo "$RESPONSE" | grep -q "ttl_value"; then
        print_success "TTL key set successfully"
    else
        print_error "TTL key not found immediately after creation"
        return
    fi
    
    # Wait for TTL to expire
    sleep 3
    
    # Check if key is expired
    RESPONSE=$(curl -s -w "\n%{http_code}" "$SERVER_URL/api/v1/keys/ttl_test")
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    
    if [ "$HTTP_CODE" = "404" ]; then
        print_success "TTL functionality working correctly"
    else
        print_error "TTL key should have expired"
    fi
}

# Health check monitoring
monitor_health() {
    print_status "Monitoring server health during load test..."
    
    HEALTH_LOG="/tmp/health_log.txt"
    echo "timestamp,status,memory_usage,active_connections" > "$HEALTH_LOG"
    
    for i in {1..10}; do
        TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
        RESPONSE=$(curl -s "$SERVER_URL/health")
        
        if command -v jq &> /dev/null; then
            STATUS=$(echo "$RESPONSE" | jq -r '.status // "unknown"')
            MEMORY=$(echo "$RESPONSE" | jq -r '.memory_usage // "unknown"')
            CONNECTIONS=$(echo "$RESPONSE" | jq -r '.active_connections // "unknown"')
        else
            STATUS="unknown"
            MEMORY="unknown"
            CONNECTIONS="unknown"
        fi
        
        echo "$TIMESTAMP,$STATUS,$MEMORY,$CONNECTIONS" >> "$HEALTH_LOG"
        sleep 5
    done
    
    print_success "Health monitoring completed, log saved to $HEALTH_LOG"
}

# Cleanup function
cleanup() {
    print_status "Cleaning up test data..."
    
    # Delete test keys
    for key in test_key load_test ab_test wrk_test txn_test ttl_test; do
        curl -s -X DELETE "$SERVER_URL/api/v1/keys/$key" > /dev/null
    done
    
    # Delete stress test keys
    for i in {1..100}; do
        curl -s -X DELETE "$SERVER_URL/api/v1/keys/stress_$i" > /dev/null
    done
    
    print_success "Cleanup completed"
}

# Main function
main() {
    echo "Mukhostho DB Load Testing Script"
    echo "================================"
    echo "Server URL: $SERVER_URL"
    echo "Requests: $NUM_REQUESTS"
    echo "Concurrency: $CONCURRENCY"
    echo "Duration: $TEST_DURATION"
    echo ""
    
    install_deps
    check_server
    
    # Run tests based on command line arguments
    case "${1:-all}" in
        "basic")
            test_basic_functionality
            ;;
        "load")
            load_test_curl
            ;;
        "ab")
            load_test_ab
            ;;
        "wrk")
            load_test_wrk
            ;;
        "transactions")
            test_transactions
            ;;
        "ttl")
            test_ttl
            ;;
        "stress")
            stress_test_memory
            ;;
        "health")
            monitor_health
            ;;
        "cleanup")
            cleanup
            ;;
        "all")
            test_basic_functionality
            test_transactions
            test_ttl
            load_test_curl
            load_test_ab
            load_test_wrk
            stress_test_memory
            cleanup
            ;;
        *)
            echo "Usage: $0 [basic|load|ab|wrk|transactions|ttl|stress|health|cleanup|all]"
            echo ""
            echo "  basic        - Test basic CRUD operations"
            echo "  load         - Run load test with curl"
            echo "  ab           - Run load test with Apache Bench"
            echo "  wrk          - Run load test with wrk"
            echo "  transactions - Test transaction functionality"
            echo "  ttl          - Test TTL functionality"
            echo "  stress       - Run memory stress test"
            echo "  health       - Monitor server health"
            echo "  cleanup      - Clean up test data"
            echo "  all          - Run all tests (default)"
            exit 1
            ;;
    esac
    
    print_success "Load testing completed!"
}

# Run main function with all arguments
main "$@"
