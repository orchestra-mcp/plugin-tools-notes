// Package internal contains the core registration logic for the tools.notes
// plugin. The ToolsPlugin struct wires all 8 tool handlers to the plugin
// builder with their schemas and descriptions.
package internal

import (
	"github.com/orchestra-mcp/sdk-go/plugin"
	"github.com/orchestra-mcp/plugin-tools-notes/internal/storage"
	"github.com/orchestra-mcp/plugin-tools-notes/internal/tools"
)

// ToolsPlugin holds the storage reference and registers all tools.
type ToolsPlugin struct {
	Storage *storage.DataStorage
}

// RegisterTools registers all 8 note management tools with the plugin builder.
func (tp *ToolsPlugin) RegisterTools(builder *plugin.PluginBuilder) {
	s := tp.Storage

	builder.RegisterTool("create_note",
		"Create a new note with title, body, and optional tags/icon/color",
		tools.CreateNoteSchema(), tools.CreateNote(s))

	builder.RegisterTool("get_note",
		"Get a note by ID",
		tools.GetNoteSchema(), tools.GetNote(s))

	builder.RegisterTool("update_note",
		"Update a note's body and optionally title, tags, icon, or color",
		tools.UpdateNoteSchema(), tools.UpdateNote(s))

	builder.RegisterTool("delete_note",
		"Soft-delete a note (sets deleted flag)",
		tools.DeleteNoteSchema(), tools.DeleteNote(s))

	builder.RegisterTool("list_notes",
		"List notes, optionally filtered by project, tag, or pinned status",
		tools.ListNotesSchema(), tools.ListNotes(s))

	builder.RegisterTool("search_notes",
		"Full-text search notes by title and body content",
		tools.SearchNotesSchema(), tools.SearchNotes(s))

	builder.RegisterTool("pin_note",
		"Pin or unpin a note",
		tools.PinNoteSchema(), tools.PinNote(s))

	builder.RegisterTool("tag_note",
		"Add or remove tags from a note",
		tools.TagNoteSchema(), tools.TagNote(s))
}
