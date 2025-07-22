package remotecfg

import (
	"context"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentInterceptor_WrapUnary(t *testing.T) {
	userAgent := "test-agent/1.0"
	interceptor := &agentInterceptor{agent: userAgent}

	// Mock the next function to capture the request
	var capturedRequest connect.AnyRequest
	nextFunc := connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		capturedRequest = req
		// Return a mock response
		return &connect.Response[any]{}, nil
	})

	// Wrap the function with our interceptor
	wrappedFunc := interceptor.WrapUnary(nextFunc)

	// Create a mock request
	req := connect.NewRequest(&struct{}{})

	// Call the wrapped function
	_, err := wrappedFunc(context.Background(), req)
	require.NoError(t, err)

	// Verify the User-Agent header was set
	assert.Equal(t, userAgent, capturedRequest.Header().Get("User-Agent"))
}

func TestAgentInterceptor_WrapStreamingClient(t *testing.T) {
	userAgent := "test-agent/1.0"
	interceptor := &agentInterceptor{agent: userAgent}

	// Mock the next function to capture the connection
	var capturedConn connect.StreamingClientConn
	nextFunc := connect.StreamingClientFunc(func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		// Create a mock streaming client connection
		conn := &mockStreamingClientConn{
			requestHeader: make(http.Header),
		}
		capturedConn = conn
		return conn
	})

	// Wrap the function with our interceptor
	wrappedFunc := interceptor.WrapStreamingClient(nextFunc)

	// Call the wrapped function
	spec := connect.Spec{} // Empty spec for testing
	conn := wrappedFunc(context.Background(), spec)

	// Verify the User-Agent header was set on the connection
	assert.Equal(t, userAgent, conn.RequestHeader().Get("User-Agent"))
	assert.Same(t, capturedConn, conn, "Should return the same connection instance")
}

func TestAgentInterceptor_WrapStreamingHandler(t *testing.T) {
	userAgent := "test-agent/1.0"
	interceptor := &agentInterceptor{agent: userAgent}

	// Mock the next handler that tracks if it was called
	var handlerCalled bool
	nextHandler := connect.StreamingHandlerFunc(func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		handlerCalled = true
		return nil
	})

	// Wrap the handler with our interceptor
	wrappedHandler := interceptor.WrapStreamingHandler(nextHandler)

	// Verify the wrapped handler is not nil and behaves like the original
	require.NotNil(t, wrappedHandler)

	// Call the wrapped handler to verify it delegates to the original
	err := wrappedHandler(context.Background(), nil)
	assert.NoError(t, err)
	assert.True(t, handlerCalled, "The original handler should have been called")
}

func TestAgentInterceptor_MultipleHeaders(t *testing.T) {
	userAgent := "test-agent/1.0"
	interceptor := &agentInterceptor{agent: userAgent}

	// Mock the next function
	nextFunc := connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		// Verify existing headers are preserved and User-Agent is added
		assert.Equal(t, "application/json", req.Header().Get("Content-Type"))
		assert.Equal(t, userAgent, req.Header().Get("User-Agent"))
		return &connect.Response[any]{}, nil
	})

	// Wrap the function with our interceptor
	wrappedFunc := interceptor.WrapUnary(nextFunc)

	// Create a request with existing headers
	req := connect.NewRequest(&struct{}{})
	req.Header().Set("Content-Type", "application/json")

	// Call the wrapped function
	_, err := wrappedFunc(context.Background(), req)
	require.NoError(t, err)
}

func TestAgentInterceptor_OverwriteExistingUserAgent(t *testing.T) {
	userAgent := "test-agent/1.0"
	interceptor := &agentInterceptor{agent: userAgent}

	// Mock the next function
	nextFunc := connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		// Verify our User-Agent overwrites any existing one
		assert.Equal(t, userAgent, req.Header().Get("User-Agent"))
		return &connect.Response[any]{}, nil
	})

	// Wrap the function with our interceptor
	wrappedFunc := interceptor.WrapUnary(nextFunc)

	// Create a request with an existing User-Agent header
	req := connect.NewRequest(&struct{}{})
	req.Header().Set("User-Agent", "old-agent/0.1")

	// Call the wrapped function
	_, err := wrappedFunc(context.Background(), req)
	require.NoError(t, err)
}

// Mock implementation for testing streaming client connections
type mockStreamingClientConn struct {
	requestHeader http.Header
}

func (m *mockStreamingClientConn) Spec() connect.Spec {
	return connect.Spec{}
}

func (m *mockStreamingClientConn) Peer() connect.Peer {
	return connect.Peer{}
}

func (m *mockStreamingClientConn) RequestHeader() http.Header {
	return m.requestHeader
}

func (m *mockStreamingClientConn) Send(msg any) error {
	return nil
}

func (m *mockStreamingClientConn) Receive(msg any) error {
	return nil
}

func (m *mockStreamingClientConn) ResponseHeader() http.Header {
	return make(http.Header)
}

func (m *mockStreamingClientConn) ResponseTrailer() http.Header {
	return make(http.Header)
}

func (m *mockStreamingClientConn) CloseRequest() error {
	return nil
}

func (m *mockStreamingClientConn) CloseResponse() error {
	return nil
}
