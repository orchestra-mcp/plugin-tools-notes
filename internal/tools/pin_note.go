package tools

import (
	"context"
	"fmt"

	pluginv1 "github.com/orchestra-mcp/gen-go/orchestra/plugin/v1"
	"github.com/orchestra-mcp/sdk-go/helpers"
	"github.com/orchestra-mcp/plugin-tools-notes/internal/storage"
	"google.golang.org/protobuf/types/known/structpb"
)

// PinNoteSchema returns the JSON Schema for the pin_note tool.
func PinNoteSchema() *structpb.Struct {
	s, _ := structpb.NewStruct(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"note_id":    map[string]any{"type": "string", "description": "Note ID"},
			"pinned":     map[string]any{"type": "boolean", "description": "Pin (true) or unpin (false)"},
			"project_id": map[string]any{"type": "string", "description": "Project slug (optional, defaults to global)"},
		},
		"required": []any{"note_id", "pinned"},
	})
	return s
}

// PinNote returns a tool handler that toggles the pinned status of a note.
func PinNote(store *storage.DataStorage) ToolHandler {
	return func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error) {
		if err := helpers.ValidateRequired(req.Arguments, "note_id"); err != nil {
			return helpers.ErrorResult("validation_error", err.Error()), nil
		}

		// Validate that pinned field is present.
		if req.Arguments == nil {
			return helpers.ErrorResult("validation_error", "missing required fields: pinned"), nil
		}
		if _, ok := req.Arguments.Fields["pinned"]; !ok {
			return helpers.ErrorResult("validation_error", "missing required fields: pinned"), nil
		}

		noteID := helpers.GetString(req.Arguments, "note_id")
		pinned := helpers.GetBool(req.Arguments, "pinned")
		projectID := storage.ResolveProject(helpers.GetString(req.Arguments, "project_id"))

		meta, body, version, err := store.ReadNote(ctx, projectID, noteID)
		if err != nil {
			return helpers.ErrorResult("not_found", fmt.Sprintf("note %q not found: %v", noteID, err)), nil
		}

		m := meta.AsMap()

		// Check if deleted.
		if deleted, ok := m["deleted"].(bool); ok && deleted {
			return helpers.ErrorResult("not_found", fmt.Sprintf("note %q has been deleted", noteID)), nil
		}

		m["pinned"] = pinned
		m["updated_at"] = helpers.NowISO()

		newMeta, err := structpb.NewStruct(m)
		if err != nil {
			return helpers.ErrorResult("internal_error", fmt.Sprintf("build metadata: %v", err)), nil
		}

		_, err = store.WriteNote(ctx, projectID, noteID, newMeta, body, version)
		if err != nil {
			return helpers.ErrorResult("storage_error", err.Error()), nil
		}

		action := "Pinned"
		if !pinned {
			action = "Unpinned"
		}
		return helpers.TextResult(fmt.Sprintf("%s note %s", action, noteID)), nil
	}
}
