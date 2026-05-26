#!/bin/bash

# Test script for manual trigger commands

echo "================================"
echo "Testing Manual Trigger Commands"
echo "================================"
echo ""

# Make sure we're in the right directory
cd "$(dirname "$0")"

echo "1. Starting daemon..."
./devtrack start &
DAEMON_PID=$!
sleep 3

echo ""
echo "2. Checking status..."
./devtrack status
echo ""

echo "3. Testing force-trigger command..."
./devtrack force-trigger
echo ""
sleep 2

echo "4. Checking logs after force-trigger..."
./devtrack logs | tail -10
echo ""

echo "5. Testing skip-next command..."
./devtrack skip-next
echo ""

echo "6. Testing send-summary command..."
./devtrack send-summary
echo ""

echo "7. Final status check..."
./devtrack status
echo ""

echo "8. Stopping daemon..."
./devtrack stop
echo ""

echo "================================"
echo "Manual Trigger Test Complete!"
echo "================================"
