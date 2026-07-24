package main

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrentUserProfileToolRegistration(t *testing.T) {
	ctx := context.Background()
	server := InitMCPServer(NewAppServer(NewXiaohongshuService()))
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, serverSession.Close()) })

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "xiaohongshu-mcp-test",
		Version: "1.0.0",
	}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clientSession.Close()) })

	result, err := clientSession.ListTools(ctx, nil)
	require.NoError(t, err)
	require.Len(t, result.Tools, 14)

	var currentUserProfileTool *mcp.Tool
	for _, tool := range result.Tools {
		if tool.Name == "current_user_profile" {
			currentUserProfileTool = tool
			break
		}
	}

	require.NotNil(t, currentUserProfileTool)
	require.NotNil(t, currentUserProfileTool.Annotations)
	assert.True(t, currentUserProfileTool.Annotations.ReadOnlyHint)
}
