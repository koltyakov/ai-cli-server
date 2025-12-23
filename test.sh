#!/bin/bash

# Test script for AI CLI Server

set -e

SERVER_URL="http://localhost:8080"
TEMP_FILE="/tmp/ai-cli-server-test-client.txt"

echo "🚀 AI CLI Server Test Script"
echo "=============================="
echo ""

# Check if server is running
echo "📡 Checking server health..."
if ! curl -s "${SERVER_URL}/health" > /dev/null 2>&1; then
    echo "❌ Server is not running. Start with: ./bin/server"
    exit 1
fi
echo "✅ Server is running"
echo ""

# Get or create API key
if [ -f "$TEMP_FILE" ]; then
    API_KEY=$(grep "^API_KEY=" "$TEMP_FILE" | cut -d'=' -f2)
    CLIENT_NAME=$(grep "^CLIENT_NAME=" "$TEMP_FILE" | cut -d'=' -f2)
    echo "📋 Using saved client: $CLIENT_NAME"
else
    echo "🔧 Creating new test client..."
    CLIENT_NAME="test-client-$(date +%s)"
    
    CREATE_OUTPUT=$(./bin/server --add "{\"name\":\"$CLIENT_NAME\", \"provider\":\"copilot\", \"models\":[\"gpt-5-mini\"], \"rate_limit\":60}" 2>&1)
    
    if echo "$CREATE_OUTPUT" | grep -q '"success": true'; then
        API_KEY=$(echo "$CREATE_OUTPUT" | grep -o '"api_key": "[^"]*"' | cut -d'"' -f4)
        echo "API_KEY=$API_KEY" > "$TEMP_FILE"
        echo "CLIENT_NAME=$CLIENT_NAME" >> "$TEMP_FILE"
        echo "✅ Client created: $CLIENT_NAME"
    else
        echo "❌ Failed to create client:"
        echo "$CREATE_OUTPUT"
        exit 1
    fi
fi
echo ""

# Test chat completion with defaults
echo "💬 Testing chat completion..."
CHAT_RESPONSE=$(curl -s -X POST "${SERVER_URL}/v1/chat/completions" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"messages": [{"role": "user", "content": "Say hello in 5 words or less"}]}')

echo "$CHAT_RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$CHAT_RESPONSE"
echo ""

# Usage stats
echo "📊 Usage statistics..."
curl -s "${SERVER_URL}/v1/usage/stats" -H "Authorization: Bearer $API_KEY" | python3 -m json.tool 2>/dev/null
echo ""

echo "✅ Done!"
echo ""
echo "💡 Commands:"
echo "   ./bin/server              # Start server"
echo "   ./bin/server --manage     # Interactive client management"
echo "   ./bin/server --list       # List clients (JSON)"
echo "   rm $TEMP_FILE  # Clear saved credentials"
