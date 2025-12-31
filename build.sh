#!/bin/bash
set -e

echo "Building AirGit..."
go build -o airgit .

echo "Restarting AirGit service..."
systemctl --user restart airgit

sleep 2

echo "Checking service status..."
systemctl --user status airgit --no-pager

echo "✓ Build and restart completed"
