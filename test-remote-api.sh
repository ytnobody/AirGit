#!/bin/bash

# Test script for Git Remote Management API
# This script tests the three new remote management endpoints

BASE_URL="${1:-http://localhost:8080}"
REPO_PATH="${2:-.}"

echo "Testing Git Remote Management API"
echo "=================================="
echo "Base URL: $BASE_URL"
echo "Repo Path: $REPO_PATH"
echo ""

# Color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

test_endpoint() {
    local method=$1
    local endpoint=$2
    local data=$3
    local description=$4
    
    echo -n "Testing $description... "
    
    if [ -z "$data" ]; then
        response=$(curl -s -w "\n%{http_code}" -X $method "$BASE_URL$endpoint")
    else
        response=$(curl -s -w "\n%{http_code}" -X $method "$BASE_URL$endpoint" \
            -H "Content-Type: application/json" \
            -d "$data")
    fi
    
    http_code=$(echo "$response" | tail -n 1)
    body=$(echo "$response" | head -n -1)
    
    if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 201 ]; then
        echo -e "${GREEN}✓ (HTTP $http_code)${NC}"
        echo "Response: $body"
    else
        echo -e "${RED}✗ (HTTP $http_code)${NC}"
        echo "Response: $body"
    fi
    echo ""
}

# Test 1: List existing remotes
echo "1. Listing existing remotes..."
test_endpoint "GET" "/api/remotes?repoPath=$REPO_PATH" "" "List Remotes"

# Test 2: Add a test remote
echo "2. Adding a test remote..."
test_endpoint "POST" "/api/remote/add?repoPath=$REPO_PATH" \
    '{"name":"test-upstream","url":"https://github.com/test/repo.git"}' \
    "Add Remote"

# Test 3: List remotes again
echo "3. Listing remotes after adding..."
test_endpoint "GET" "/api/remotes?repoPath=$REPO_PATH" "" "List Remotes (after add)"

# Test 4: Update the test remote
echo "4. Updating the test remote URL..."
test_endpoint "POST" "/api/remote/update?repoPath=$REPO_PATH" \
    '{"name":"test-upstream","url":"https://github.com/test/updated-repo.git"}' \
    "Update Remote"

# Test 5: List remotes again
echo "5. Listing remotes after updating..."
test_endpoint "GET" "/api/remotes?repoPath=$REPO_PATH" "" "List Remotes (after update)"

# Test 6: Remove the test remote
echo "6. Removing the test remote..."
test_endpoint "POST" "/api/remote/remove?repoPath=$REPO_PATH" \
    '{"name":"test-upstream"}' \
    "Remove Remote"

# Test 7: List remotes again (final check)
echo "7. Listing remotes after removing..."
test_endpoint "GET" "/api/remotes?repoPath=$REPO_PATH" "" "List Remotes (after remove)"

echo "=================================="
echo "All tests completed!"
