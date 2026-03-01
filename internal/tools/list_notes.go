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

// ListNotesSchema returns the JSON Schema for the list_notes tool.
func ListNotesSchema() *structpb.Struct {
	s, _ := structpb.NewStruct(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_id": map[string]any{"type": "string", "description": "Project slug (optional, defaults to global)"},
			"tag":        map[string]any{"type": "string", "description": "Filter by tag (optional)"},
			"pinned":     map[string]any{"type": "boolean", "description": "Filter by pinned status (optional)"},
		},
	})
	return s
}

// ListNotes returns a tool handler that lists notes, optionally filtered.
func ListNotes(store *storage.DataStorage) ToolHandler {
	return func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error) {
		projectID := storage.ResolveProject(helpers.GetString(req.Arguments, "project_id"))
		tagFilter := helpers.GetString(req.Arguments, "tag")

		// Check if pinned filter was explicitly provided.
		var pinnedFilter *bool
		if req.Arguments != nil {
			if _, ok := req.Arguments.Fields["pinned"]; ok {
				v := helpers.GetBool(req.Arguments, "pinned")
				pinnedFilter = &v
			}
		}

		entries, err := store.ListNotes(ctx, projectID)
		if err != nil {
			return helpers.ErrorResult("storage_error", err.Error()), nil
		}

		type noteInfo struct {
			id     string
			title  string
			pinned bool
			tags   []string
		}

		var notes []noteInfo
		for _, entry := range entries {
			noteID := storage.NoteIDFromPath(entry.Path)
			meta, _, _, err := store.ReadNote(ctx, projectID, noteID)
			if err != nil {
				continue
			}
			m := meta.AsMap()

			// Skip deleted notes.
			if deleted, ok := m["deleted"].(bool); ok && deleted {
				continue
			}

			title, _ := m["title"].(string)
			pinned, _ := m["pinned"].(bool)
			var tags []string
			if tagList, ok := m["tags"].([]any); ok {
				for _, t := range tagList {
					if s, ok := t.(string); ok {
						tags = append(tags, s)
					}
				}
			}

			// Apply pinned filter.
			if pinnedFilter != nil && pinned != *pinnedFilter {
				continue
			}

			// Apply tag filter.
			if tagFilter != "" {
				found := false
				for _, t := range tags {
					if t == tagFilter {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			notes = append(notes, noteInfo{
				id:     noteID,
				title:  title,
				pinned: pinned,
				tags:   tags,
			})
		}

		if len(notes) == 0 {
			return helpers.TextResult(fmt.Sprintf("## Notes (%s)\n\nNo notes found.\n", projectID)), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "## Notes (%s) — %d found\n\n", projectID, len(notes))
		fmt.Fprintf(&b, "| ID | Title | Pinned | Tags |\n")
		fmt.Fprintf(&b, "|----|-------|--------|------|\n")
		for _, n := range notes {
			pinnedStr := "no"
			if n.pinned {
				pinnedStr = "yes"
			}
			tagStr := "—"
			if len(n.tags) > 0 {
				tagStr = strings.Join(n.tags, ", ")
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", n.id, n.title, pinnedStr, tagStr)
		}

		return helpers.TextResult(b.String()), nil
	}
}
