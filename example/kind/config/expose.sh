#!/bin/bash

# Script to expose Grafana Cloud Collector pod via port-forward
KUBECONFIG=${1:-build/kubeconfig.yaml}

# Get the first pod name from the daemonset
POD_NAME=$(kubectl --kubeconfig "$KUBECONFIG" get pods -n monitoring -l app.kubernetes.io/name=alloy-daemon -o jsonpath="{.items[0].metadata.name}")

if [ -z "$POD_NAME" ]; then
    echo "Error: No pods found with label 'app.kubernetes.io/name=alloy-daemon' in namespace 'monitoring'"
    exit 1
fi

# Start port-forward in background
kubectl --kubeconfig "$KUBECONFIG" port-forward -n monitoring pod/$POD_NAME 12345:12345 &
PORT_FORWARD_PID=$!

# Give port-forward a moment to start
sleep 2

# Print available URL
echo ""
echo "🌐 Service is now available at:"
echo "   Alloy: http://localhost:12345"
echo ""
echo "Press Ctrl+C to stop port forwarding"
echo ""

# Function to cleanup background processes
cleanup() {
  echo ""
  echo "Stopping port forwarding..."
  kill $PORT_FORWARD_PID 2>/dev/null || true
  exit 0
}

# Set up signal handling
trap cleanup SIGINT SIGTERM

# Wait for background processes
wait