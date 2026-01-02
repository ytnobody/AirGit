#!/bin/bash

# Test script for GitHub issue creation endpoint

echo "Testing GitHub Issue Creation API"
echo "==================================="
echo ""

# Define test server URL
SERVER="http://localhost:8080"

# Test 1: Missing required title field
echo "Test 1: Create issue without title (should fail)"
curl -X POST "$SERVER/api/github/issues/create" \
  -H "Content-Type: application/json" \
  -d '{"body": "Test body"}' 2>/dev/null | jq . 2>/dev/null || echo "Could not parse JSON response"
echo ""

# Test 2: Create issue with only required fields
echo "Test 2: Create issue with title only"
curl -X POST "$SERVER/api/github/issues/create" \
  -H "Content-Type: application/json" \
  -d '{"title": "Test Issue"}' 2>/dev/null | jq . 2>/dev/null || echo "Could not parse JSON response"
echo ""

# Test 3: Create issue with all fields
echo "Test 3: Create issue with title, body, and labels"
curl -X POST "$SERVER/api/github/issues/create" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Test Issue with Labels",
    "body": "This is a test issue",
    "labels": ["test", "bug"]
  }' 2>/dev/null | jq . 2>/dev/null || echo "Could not parse JSON response"
echo ""

# Test 4: Wrong HTTP method
echo "Test 4: Using GET instead of POST (should fail)"
curl -X GET "$SERVER/api/github/issues/create" 2>/dev/null | jq . 2>/dev/null || echo "Could not parse JSON response"
echo ""

echo "Tests completed!"
