package tools

import (
	"context"
	"fmt"

	pluginv1 "github.com/orchestra-mcp/gen-go/orchestra/plugin/v1"
	"github.com/orchestra-mcp/sdk-go/helpers"
	"github.com/orchestra-mcp/plugin-tools-notes/internal/storage"
	"google.golang.org/protobuf/types/known/structpb"
)

// UpdateNoteSchema returns the JSON Schema for the update_note tool.
func UpdateNoteSchema() *structpb.Struct {
	s, _ := structpb.NewStruct(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"note_id":    map[string]any{"type": "string", "description": "Note ID"},
			"body":       map[string]any{"type": "string", "description": "New note body content"},
			"title":      map[string]any{"type": "string", "description": "New title (optional)"},
			"project_id": map[string]any{"type": "string", "description": "Project slug (optional, defaults to global)"},
			"tags": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Replace tags (optional)",
			},
			"icon":  map[string]any{"type": "string", "description": "New icon (optional)"},
			"color": map[string]any{"type": "string", "description": "New color (optional)"},
		},
		"required": []any{"note_id", "body"},
	})
	return s
}

// UpdateNote returns a tool handler that updates fields of an existing note.
func UpdateNote(store *storage.DataStorage) ToolHandler {
	return func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error) {
		if err := helpers.ValidateRequired(req.Arguments, "note_id", "body"); err != nil {
			return helpers.ErrorResult("validation_error", err.Error()), nil
		}

		noteID := helpers.GetString(req.Arguments, "note_id")
		body := helpers.GetString(req.Arguments, "body")
		projectID := storage.ResolveProject(helpers.GetString(req.Arguments, "project_id"))

		meta, _, version, err := store.ReadNote(ctx, projectID, noteID)
		if err != nil {
			return helpers.ErrorResult("not_found", fmt.Sprintf("note %q not found: %v", noteID, err)), nil
		}

		m := meta.AsMap()

		// Check if deleted.
		if deleted, ok := m["deleted"].(bool); ok && deleted {
			return helpers.ErrorResult("not_found", fmt.Sprintf("note %q has been deleted", noteID)), nil
		}

		// Update title if provided.
		if t := helpers.GetString(req.Arguments, "title"); t != "" {
			m["title"] = t
		}

		// Update tags if provided.
		if tags := helpers.GetStringSlice(req.Arguments, "tags"); tags != nil {
			tagValues := make([]any, len(tags))
			for i, t := range tags {
				tagValues[i] = t
			}
			m["tags"] = tagValues
		}

		// Update icon if provided.
		if icon := helpers.GetString(req.Arguments, "icon"); icon != "" {
			m["icon"] = icon
		}

		// Update color if provided.
		if color := helpers.GetString(req.Arguments, "color"); color != "" {
			m["color"] = color
		}

		m["updated_at"] = helpers.NowISO()

		newMeta, err := structpb.NewStruct(m)
		if err != nil {
			return helpers.ErrorResult("internal_error", fmt.Sprintf("build metadata: %v", err)), nil
		}

		_, err = store.WriteNote(ctx, projectID, noteID, newMeta, body, version)
		if err != nil {
			return helpers.ErrorResult("storage_error", err.Error()), nil
		}

		md := fmt.Sprintf("Updated note **%s**\n\n%s", noteID, formatNoteMD(newMeta))
		return helpers.TextResult(md), nil
	}
}
