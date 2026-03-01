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

// TagNoteSchema returns the JSON Schema for the tag_note tool.
func TagNoteSchema() *structpb.Struct {
	s, _ := structpb.NewStruct(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"note_id": map[string]any{"type": "string", "description": "Note ID"},
			"action":  map[string]any{"type": "string", "description": "Action: add or remove", "enum": []any{"add", "remove"}},
			"tags": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Tags to add or remove",
			},
			"project_id": map[string]any{"type": "string", "description": "Project slug (optional, defaults to global)"},
		},
		"required": []any{"note_id", "action", "tags"},
	})
	return s
}

// TagNote returns a tool handler that adds or removes tags from a note.
func TagNote(store *storage.DataStorage) ToolHandler {
	return func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error) {
		if err := helpers.ValidateRequired(req.Arguments, "note_id", "action"); err != nil {
			return helpers.ErrorResult("validation_error", err.Error()), nil
		}

		noteID := helpers.GetString(req.Arguments, "note_id")
		action := helpers.GetString(req.Arguments, "action")
		tags := helpers.GetStringSlice(req.Arguments, "tags")
		projectID := storage.ResolveProject(helpers.GetString(req.Arguments, "project_id"))

		if err := helpers.ValidateOneOf(action, "add", "remove"); err != nil {
			return helpers.ErrorResult("validation_error", err.Error()), nil
		}

		if len(tags) == 0 {
			return helpers.ErrorResult("validation_error", "missing required fields: tags"), nil
		}

		meta, body, version, err := store.ReadNote(ctx, projectID, noteID)
		if err != nil {
			return helpers.ErrorResult("not_found", fmt.Sprintf("note %q not found: %v", noteID, err)), nil
		}

		m := meta.AsMap()

		// Check if deleted.
		if deleted, ok := m["deleted"].(bool); ok && deleted {
			return helpers.ErrorResult("not_found", fmt.Sprintf("note %q has been deleted", noteID)), nil
		}

		// Get existing tags.
		var existing []string
		if tagList, ok := m["tags"].([]any); ok {
			for _, t := range tagList {
				if s, ok := t.(string); ok {
					existing = append(existing, s)
				}
			}
		}

		if action == "add" {
			// Add new tags, avoiding duplicates.
			existingSet := make(map[string]bool)
			for _, t := range existing {
				existingSet[t] = true
			}
			for _, t := range tags {
				if !existingSet[t] {
					existing = append(existing, t)
					existingSet[t] = true
				}
			}
		} else {
			// Remove specified tags.
			removeSet := make(map[string]bool)
			for _, t := range tags {
				removeSet[t] = true
			}
			var filtered []string
			for _, t := range existing {
				if !removeSet[t] {
					filtered = append(filtered, t)
				}
			}
			existing = filtered
		}

		// Convert to []any for structpb.
		tagValues := make([]any, len(existing))
		for i, t := range existing {
			tagValues[i] = t
		}
		m["tags"] = tagValues
		m["updated_at"] = helpers.NowISO()

		newMeta, err := structpb.NewStruct(m)
		if err != nil {
			return helpers.ErrorResult("internal_error", fmt.Sprintf("build metadata: %v", err)), nil
		}

		_, err = store.WriteNote(ctx, projectID, noteID, newMeta, body, version)
		if err != nil {
			return helpers.ErrorResult("storage_error", err.Error()), nil
		}

		verb := "Added"
		if action == "remove" {
			verb = "Removed"
		}
		tagStr := strings.Join(tags, ", ")
		resultTags := "none"
		if len(existing) > 0 {
			resultTags = strings.Join(existing, ", ")
		}
		md := fmt.Sprintf("%s tags [%s] on note %s\n\n- **Current tags:** %s", verb, tagStr, noteID, resultTags)
		return helpers.TextResult(md), nil
	}
}
