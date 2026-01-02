#!/bin/bash

# Test script for GitHub Issue Creation API
# This script demonstrates how to use the issue creation endpoint

set -e

BASE_URL="${1:-http://localhost:8080}"

echo "=== GitHub Issue Creation API Test ==="
echo "Base URL: $BASE_URL"
echo ""

# Test 1: Create a simple issue with just title
echo "Test 1: Create simple issue with title only"
echo "Command: curl -X POST $BASE_URL/api/github/issues/create -H 'Content-Type: application/json' -d '{\"title\": \"Test Issue\"}'"
echo ""

# Test 2: Create issue with body
echo "Test 2: Create issue with title and body"
echo "Command: curl -X POST $BASE_URL/api/github/issues/create -H 'Content-Type: application/json' -d '{\"title\": \"Feature Request\", \"body\": \"Please implement feature X\"}'"
echo ""

# Test 3: Create issue with labels
echo "Test 3: Create issue with title, body and labels"
echo "Command: curl -X POST $BASE_URL/api/github/issues/create -H 'Content-Type: application/json' -d '{\"title\": \"Bug Fix\", \"body\": \"This is a bug\", \"labels\": [\"bug\", \"critical\"]}'"
echo ""

# Test 4: Test error - missing title
echo "Test 4: Error case - missing title (should fail with 400)"
echo "Command: curl -X POST $BASE_URL/api/github/issues/create -H 'Content-Type: application/json' -d '{\"title\": \"\"}'"
echo ""

# Test 5: List issues
echo "Test 5: List issues from repository"
echo "Command: curl $BASE_URL/api/github/issues"
echo ""

echo "=== Notes ==="
echo "- The endpoint requires GitHub CLI authentication (gh auth login -s copilot)"
echo "- The repository must have a configured GitHub remote (origin)"
echo "- Only 'title' is required; 'body' and 'labels' are optional"
echo "- Success response includes the issue URL"
echo "- Error responses include descriptive error messages"
echo ""

echo "=== Example with actual curl command ==="
echo ""
echo "# Create a simple test issue"
echo "curl -X POST $BASE_URL/api/github/issues/create \\"
echo "  -H 'Content-Type: application/json' \\"
echo "  -d '{\"title\": \"Test Issue\", \"body\": \"This is a test\", \"labels\": [\"test\"]}'"
echo ""
