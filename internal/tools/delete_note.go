package tools

import (
	"context"
	"fmt"

	pluginv1 "github.com/orchestra-mcp/gen-go/orchestra/plugin/v1"
	"github.com/orchestra-mcp/sdk-go/helpers"
	"github.com/orchestra-mcp/plugin-tools-notes/internal/storage"
	"google.golang.org/protobuf/types/known/structpb"
)

// DeleteNoteSchema returns the JSON Schema for the delete_note tool.
func DeleteNoteSchema() *structpb.Struct {
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

// DeleteNote returns a tool handler that soft-deletes a note by setting deleted: true.
func DeleteNote(store *storage.DataStorage) ToolHandler {
	return func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error) {
		if err := helpers.ValidateRequired(req.Arguments, "note_id"); err != nil {
			return helpers.ErrorResult("validation_error", err.Error()), nil
		}

		noteID := helpers.GetString(req.Arguments, "note_id")
		projectID := storage.ResolveProject(helpers.GetString(req.Arguments, "project_id"))

		meta, body, version, err := store.ReadNote(ctx, projectID, noteID)
		if err != nil {
			return helpers.ErrorResult("not_found", fmt.Sprintf("note %q not found: %v", noteID, err)), nil
		}

		m := meta.AsMap()
		m["deleted"] = true
		m["updated_at"] = helpers.NowISO()

		newMeta, err := structpb.NewStruct(m)
		if err != nil {
			return helpers.ErrorResult("internal_error", fmt.Sprintf("build metadata: %v", err)), nil
		}

		_, err = store.WriteNote(ctx, projectID, noteID, newMeta, body, version)
		if err != nil {
			return helpers.ErrorResult("storage_error", err.Error()), nil
		}

		return helpers.TextResult(fmt.Sprintf("Deleted note %s", noteID)), nil
	}
}
