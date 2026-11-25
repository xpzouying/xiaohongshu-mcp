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
				result, err := session.CallTool(ctx, &mcp.CallToolParams{
					Name: "publish_content", Arguments: json.RawMessage(test.input),
				})
				require.NoError(t, err)
				require.NotNil(t, result)
				require.True(t, result.IsError)
				require.NotEmpty(t, result.Content)
				content, ok := result.Content[0].(*mcp.TextContent)
				require.True(t, ok)
				require.Contains(t, content.Text, "validating")
				require.Contains(t, content.Text, "images")
			})
		})
	}
}

func TestPublishAIDeclarationSchema(t *testing.T) {
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

	schemas := map[string]*jsonschema.Resolved{}
	for _, tool := range tools.Tools {
		if tool.Name != "publish_content" && tool.Name != "publish_with_video" {
			continue
		}
		var schema jsonschema.Schema
		encoded, err := json.Marshal(tool.InputSchema)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(encoded, &schema))
		require.Contains(t, schema.Properties, "is_ai_generated")
		resolved, err := schema.Resolve(nil)
		require.NoError(t, err)
		schemas[tool.Name] = resolved
	}
	require.Len(t, schemas, 2)

	cases := []struct {
		name  string
		tool  string
		input string
		valid bool
	}{
		{"image_omitted", "publish_content", `{"title":"t","content":"c","images":["/tmp/a.png"]}`, true},
		{"image_true", "publish_content", `{"title":"t","content":"c","images":["/tmp/a.png"],"is_ai_generated":true}`, true},
		{"image_false", "publish_content", `{"title":"t","content":"c","images":["/tmp/a.png"],"is_ai_generated":false}`, true},
		{"image_wrong_type", "publish_content", `{"title":"t","content":"c","images":["/tmp/a.png"],"is_ai_generated":"yes"}`, false},
		{"video_omitted", "publish_with_video", `{"title":"t","content":"c","video":"/tmp/a.mp4"}`, true},
		{"video_true", "publish_with_video", `{"title":"t","content":"c","video":"/tmp/a.mp4","is_ai_generated":true}`, true},
		{"video_wrong_type", "publish_with_video", `{"title":"t","content":"c","video":"/tmp/a.mp4","is_ai_generated":1}`, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var input any
			require.NoError(t, json.Unmarshal([]byte(tc.input), &input))
			if tc.valid {
				require.NoError(t, schemas[tc.tool].Validate(input))
			} else {
				require.Error(t, schemas[tc.tool].Validate(input))
			}
		})
	}

	var imgReq PublishRequest
	require.NoError(t, json.Unmarshal([]byte(`{"title":"t","content":"c","images":["/tmp/a.png"],"is_ai_generated":true}`), &imgReq))
	require.True(t, imgReq.IsAIGenerated)

	var vidReq PublishVideoRequest
	require.NoError(t, json.Unmarshal([]byte(`{"title":"t","content":"c","video":"/tmp/a.mp4","is_ai_generated":true}`), &vidReq))
	require.True(t, vidReq.IsAIGenerated)
}
