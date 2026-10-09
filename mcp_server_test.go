package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestPublishContentImagesSchema(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	ctx := context.Background()
	server := InitMCPServer(&AppServer{})
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "schema-test", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { session.Close() })
	tools, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	var schema jsonschema.Schema
	for _, tool := range tools.Tools {
		if tool.Name == "publish_content" {
			encoded, err := json.Marshal(tool.InputSchema)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(encoded, &schema))
		}
	}
	require.Contains(t, schema.Properties, "images")
	resolved, err := schema.Resolve(nil)
	require.NoError(t, err)
	for _, test := range []struct {
		name  string
		input string
		valid bool
	}{
		{"missing", `{"title":"test","content":"test"}`, false},
		{"null", `{"title":"test","content":"test","images":null}`, false},
		{"empty", `{"title":"test","content":"test","images":[]}`, false},
		{"wrong_item_type", `{"title":"test","content":"test","images":[123]}`, false},
		{"one_image", `{"title":"test","content":"test","images":["/tmp/image.jpg"]}`, true},
		{"multiple_images", `{"title":"test","content":"test","images":["/tmp/a.jpg","/tmp/b.jpg"]}`, true},
		{"optional_nulls", `{"title":"test","content":"test","images":["/tmp/image.jpg"],"tags":null,"products":null}`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var input any
			require.NoError(t, json.Unmarshal([]byte(test.input), &input))
			if test.valid {
				require.NoError(t, resolved.Validate(input))
				return
			}
			t.Run("advertised_schema", func(t *testing.T) {
				require.Error(t, resolved.Validate(input))
			})
			t.Run("tool_call", func(t *testing.T) {
				_, err := session.CallTool(ctx, &mcp.CallToolParams{
					Name: "publish_content", Arguments: json.RawMessage(test.input),
				})
				require.ErrorContains(t, err, "validating")
				require.ErrorContains(t, err, "images")
			})
		})
	}
}
