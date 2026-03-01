package tools

import (
	"context"
	"fmt"
	"strings"

	pluginv1 "github.com/orchestra-mcp/gen-go/orchestra/plugin/v1"
	"github.com/orchestra-mcp/sdk-go/helpers"
	"github.com/orchestra-mcp/plugin-tools-notes/internal/storage"
	"google.golang.org/protobuf/types/known/structpb"
)

// GetNoteSchema returns the JSON Schema for the get_note tool.
func GetNoteSchema() *structpb.Struct {
	s, _ := structpb.NewStruct(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"note_id":    map[string]any{"type": "string", "description": "Note ID"},
			"project_id": map[string]any{"type": "string", "description": "Project slug (optional, defaults to global)"},
		},
		"required": []any{"note_id"},
	})
	return s
}

// GetNote returns a tool handler that retrieves a note by ID.
func GetNote(store *storage.DataStorage) ToolHandler {
	return func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error) {
		if err := helpers.ValidateRequired(req.Arguments, "note_id"); err != nil {
			return helpers.ErrorResult("validation_error", err.Error()), nil
		}

		noteID := helpers.GetString(req.Arguments, "note_id")
		projectID := storage.ResolveProject(helpers.GetString(req.Arguments, "project_id"))

		meta, body, _, err := store.ReadNote(ctx, projectID, noteID)
		if err != nil {
			return helpers.ErrorResult("not_found", fmt.Sprintf("note %q not found: %v", noteID, err)), nil
		}

		md := formatNoteMD(meta) + "\n---\n\n" + body
		return helpers.TextResult(md), nil
	}
}

// formatNoteMD formats note metadata as a Markdown block.
func formatNoteMD(meta *structpb.Struct) string {
	if meta == nil {
		return ""
	}
	m := meta.AsMap()

	var b strings.Builder
	id, _ := m["id"].(string)
	title, _ := m["title"].(string)
	fmt.Fprintf(&b, "### %s — %s\n", id, title)

	if pinned, ok := m["pinned"].(bool); ok && pinned {
		fmt.Fprintf(&b, "- **Pinned:** yes\n")
	}
	if icon, ok := m["icon"].(string); ok && icon != "" {
		fmt.Fprintf(&b, "- **Icon:** %s\n", icon)
	}
	if color, ok := m["color"].(string); ok && color != "" {
		fmt.Fprintf(&b, "- **Color:** %s\n", color)
	}
	if tags, ok := m["tags"].([]any); ok && len(tags) > 0 {
		var tagStrs []string
		for _, t := range tags {
			if s, ok := t.(string); ok {
				tagStrs = append(tagStrs, s)
			}
		}
		if len(tagStrs) > 0 {
			fmt.Fprintf(&b, "- **Tags:** %s\n", strings.Join(tagStrs, ", "))
		}
	}
	if createdAt, ok := m["created_at"].(string); ok {
		fmt.Fprintf(&b, "- **Created:** %s\n", createdAt)
	}
	if updatedAt, ok := m["updated_at"].(string); ok {
		fmt.Fprintf(&b, "- **Updated:** %s\n", updatedAt)
	}
	return b.String()
}
