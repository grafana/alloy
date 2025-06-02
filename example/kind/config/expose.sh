#!/bin/bash

# Script to expose Grafana and Mimir services via port-forward
KUBECONFIG=${1:-build/kubeconfig.yaml}

# Start Grafana port-forward in background
kubectl --kubeconfig "$KUBECONFIG" port-forward -n monitoring service/grafana 3000:80 &
GRAFANA_PID=$!

# Start Mimir port-forward in background
kubectl --kubeconfig "$KUBECONFIG" port-forward -n mimir service/mimir-nginx 9009:80 &
MIMIR_PID=$!

# Give port-forward a moment to start
sleep 2

# Print available URLs
echo ""
echo "🌐 Services are now available at:"
echo "   Grafana: http://localhost:3000"
echo "   Mimir: http://localhost:9009"
echo ""
echo "Press Ctrl+C to stop all port forwarding"
echo ""

# Function to cleanup background processes
cleanup() {
  echo ""
  echo "Stopping port forwarding..."
  kill $GRAFANA_PID 2>/dev/null || true
  kill $MIMIR_PID 2>/dev/null || true
  exit 0
}

# Set up signal handling
trap cleanup SIGINT SIGTERM

# Wait for background processes
wait 