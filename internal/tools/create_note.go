package tools

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	pluginv1 "github.com/orchestra-mcp/gen-go/orchestra/plugin/v1"
	"github.com/orchestra-mcp/sdk-go/helpers"
	"github.com/orchestra-mcp/plugin-tools-notes/internal/storage"
	"google.golang.org/protobuf/types/known/structpb"
)

// ToolHandler is the function signature for all tool handlers.
type ToolHandler = func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error)

// newNoteID generates a short random note ID like "note-a1b2c3".
func newNoteID() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return "note-" + hex.EncodeToString(b)
}

// CreateNoteSchema returns the JSON Schema for the create_note tool.
func CreateNoteSchema() *structpb.Struct {
	s, _ := structpb.NewStruct(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title":      map[string]any{"type": "string", "description": "Note title"},
			"body":       map[string]any{"type": "string", "description": "Note body content (markdown)"},
			"project_id": map[string]any{"type": "string", "description": "Project slug (optional, defaults to global)"},
			"tags": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Tags for the note",
			},
			"icon":  map[string]any{"type": "string", "description": "Icon name (e.g., lightbulb)"},
			"color": map[string]any{"type": "string", "description": "Color label (e.g., yellow)"},
		},
		"required": []any{"title", "body"},
	})
	return s
}

// CreateNote returns a tool handler that creates a new note.
func CreateNote(store *storage.DataStorage) ToolHandler {
	return func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error) {
		if err := helpers.ValidateRequired(req.Arguments, "title", "body"); err != nil {
			return helpers.ErrorResult("validation_error", err.Error()), nil
		}

		title := helpers.GetString(req.Arguments, "title")
		body := helpers.GetString(req.Arguments, "body")
		projectID := storage.ResolveProject(helpers.GetString(req.Arguments, "project_id"))
		tags := helpers.GetStringSlice(req.Arguments, "tags")
		icon := helpers.GetString(req.Arguments, "icon")
		color := helpers.GetString(req.Arguments, "color")

		noteID := newNoteID()
		now := helpers.NowISO()

		meta := map[string]any{
			"id":         noteID,
			"title":      title,
			"pinned":     false,
			"deleted":    false,
			"created_at": now,
			"updated_at": now,
		}
		if len(tags) > 0 {
			tagValues := make([]any, len(tags))
			for i, t := range tags {
				tagValues[i] = t
			}
			meta["tags"] = tagValues
		}
		if icon != "" {
			meta["icon"] = icon
		}
		if color != "" {
			meta["color"] = color
		}

		metadata, err := structpb.NewStruct(meta)
		if err != nil {
			return helpers.ErrorResult("internal_error", fmt.Sprintf("build metadata: %v", err)), nil
		}

		_, err = store.WriteNote(ctx, projectID, noteID, metadata, body, 0)
		if err != nil {
			return helpers.ErrorResult("storage_error", err.Error()), nil
		}

		var tagStr string
		if len(tags) > 0 {
			tagStr = fmt.Sprintf("\n- **Tags:** %s", strings.Join(tags, ", "))
		}

		md := fmt.Sprintf("Created note **%s**: %s\n\n- **ID:** %s\n- **Project:** %s%s\n- **Created:** %s",
			noteID, title, noteID, projectID, tagStr, now)
		return helpers.TextResult(md), nil
	}
}
