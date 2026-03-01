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

// SearchNotesSchema returns the JSON Schema for the search_notes tool.
func SearchNotesSchema() *structpb.Struct {
	s, _ := structpb.NewStruct(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query":      map[string]any{"type": "string", "description": "Search query (substring match on title and body)"},
			"project_id": map[string]any{"type": "string", "description": "Project slug (optional, defaults to global)"},
		},
		"required": []any{"query"},
	})
	return s
}

// SearchNotes returns a tool handler that searches notes by title and body content.
func SearchNotes(store *storage.DataStorage) ToolHandler {
	return func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error) {
		if err := helpers.ValidateRequired(req.Arguments, "query"); err != nil {
			return helpers.ErrorResult("validation_error", err.Error()), nil
		}

		query := strings.ToLower(helpers.GetString(req.Arguments, "query"))
		projectID := storage.ResolveProject(helpers.GetString(req.Arguments, "project_id"))

		entries, err := store.ListNotes(ctx, projectID)
		if err != nil {
			return helpers.ErrorResult("storage_error", err.Error()), nil
		}

		type matchInfo struct {
			id    string
			title string
		}

		var matches []matchInfo
		for _, entry := range entries {
			noteID := storage.NoteIDFromPath(entry.Path)
			meta, body, _, err := store.ReadNote(ctx, projectID, noteID)
			if err != nil {
				continue
			}
			m := meta.AsMap()

			// Skip deleted notes.
			if deleted, ok := m["deleted"].(bool); ok && deleted {
				continue
			}

			title, _ := m["title"].(string)
			titleMatch := strings.Contains(strings.ToLower(title), query)
			bodyMatch := strings.Contains(strings.ToLower(body), query)

			if titleMatch || bodyMatch {
				matches = append(matches, matchInfo{id: noteID, title: title})
			}
		}

		if len(matches) == 0 {
			return helpers.TextResult(fmt.Sprintf("## Search results for %q\n\nNo notes found.\n", query)), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "## Search results for %q — %d found\n\n", query, len(matches))
		fmt.Fprintf(&b, "| ID | Title |\n")
		fmt.Fprintf(&b, "|----|-------|\n")
		for _, m := range matches {
			fmt.Fprintf(&b, "| %s | %s |\n", m.id, m.title)
		}

		return helpers.TextResult(b.String()), nil
	}
}
