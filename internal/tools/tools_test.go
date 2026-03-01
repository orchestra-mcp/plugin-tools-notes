package tools_test

import (
	"context"
	"strings"
	"testing"

	pluginv1 "github.com/orchestra-mcp/gen-go/orchestra/plugin/v1"
	"github.com/orchestra-mcp/plugin-tools-notes/internal/storage"
	"github.com/orchestra-mcp/plugin-tools-notes/internal/tools"
	"google.golang.org/protobuf/types/known/structpb"
)

// ---------- Mock storage client ----------

// noteRecord holds in-memory note data.
type noteRecord struct {
	metadata *structpb.Struct
	body     string
	version  int64
}

// mockClient is an in-memory StorageClient that stores notes in a map.
type mockClient struct {
	notes   map[string]*noteRecord // key = path
	version int64
}

func newMockClient() *mockClient {
	return &mockClient{notes: make(map[string]*noteRecord)}
}

func (m *mockClient) Send(_ context.Context, req *pluginv1.PluginRequest) (*pluginv1.PluginResponse, error) {
	switch r := req.Request.(type) {
	case *pluginv1.PluginRequest_StorageWrite:
		m.version++
		m.notes[r.StorageWrite.Path] = &noteRecord{
			metadata: r.StorageWrite.Metadata,
			body:     string(r.StorageWrite.Content),
			version:  m.version,
		}
		return &pluginv1.PluginResponse{
			Response: &pluginv1.PluginResponse_StorageWrite{
				StorageWrite: &pluginv1.StorageWriteResponse{
					Success:    true,
					NewVersion: m.version,
				},
			},
		}, nil

	case *pluginv1.PluginRequest_StorageRead:
		rec, ok := m.notes[r.StorageRead.Path]
		if !ok {
			return &pluginv1.PluginResponse{
				Response: &pluginv1.PluginResponse_StorageRead{
					StorageRead: &pluginv1.StorageReadResponse{},
				},
			}, nil
		}
		return &pluginv1.PluginResponse{
			Response: &pluginv1.PluginResponse_StorageRead{
				StorageRead: &pluginv1.StorageReadResponse{
					Metadata: rec.metadata,
					Content:  []byte(rec.body),
					Version:  rec.version,
				},
			},
		}, nil

	case *pluginv1.PluginRequest_StorageDelete:
		delete(m.notes, r.StorageDelete.Path)
		return &pluginv1.PluginResponse{
			Response: &pluginv1.PluginResponse_StorageDelete{
				StorageDelete: &pluginv1.StorageDeleteResponse{Success: true},
			},
		}, nil

	case *pluginv1.PluginRequest_StorageList:
		prefix := r.StorageList.Prefix
		var entries []*pluginv1.StorageEntry
		for path := range m.notes {
			if strings.HasPrefix(path, prefix) {
				entries = append(entries, &pluginv1.StorageEntry{Path: path})
			}
		}
		return &pluginv1.PluginResponse{
			Response: &pluginv1.PluginResponse_StorageList{
				StorageList: &pluginv1.StorageListResponse{Entries: entries},
			},
		}, nil
	}

	return &pluginv1.PluginResponse{}, nil
}

// ---------- Helpers ----------

func makeStore() *storage.DataStorage {
	return storage.NewDataStorage(newMockClient())
}

func makeArgs(t *testing.T, m map[string]any) *structpb.Struct {
	t.Helper()
	s, err := structpb.NewStruct(m)
	if err != nil {
		t.Fatalf("makeArgs: %v", err)
	}
	return s
}

func assertSuccess(t *testing.T, resp *pluginv1.ToolResponse) {
	t.Helper()
	if !resp.Success {
		t.Fatalf("expected success, got error: %s — %s", resp.ErrorCode, resp.ErrorMessage)
	}
}

func assertError(t *testing.T, resp *pluginv1.ToolResponse, code string) {
	t.Helper()
	if resp.Success {
		t.Fatalf("expected error code %q but got success", code)
	}
	if resp.ErrorCode != code {
		t.Fatalf("expected error code %q, got %q", code, resp.ErrorCode)
	}
}

func responseText(t *testing.T, resp *pluginv1.ToolResponse) string {
	t.Helper()
	if resp.Result == nil {
		return ""
	}
	v, ok := resp.Result.Fields["text"]
	if !ok {
		t.Fatalf("result missing 'text' key")
	}
	return v.GetStringValue()
}

// ---------- create_note ----------

func TestCreateNote_Basic(t *testing.T) {
	store := makeStore()
	fn := tools.CreateNote(store)
	resp, err := fn(context.Background(), &pluginv1.ToolRequest{
		Arguments: makeArgs(t, map[string]any{
			"title": "My Note",
			"body":  "Note content here.",
		}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSuccess(t, resp)
	text := responseText(t, resp)
	if !strings.Contains(text, "My Note") {
		t.Fatalf("expected note title in response, got: %s", text)
	}
	if !strings.Contains(text, "note-") {
		t.Fatalf("expected note ID in response, got: %s", text)
	}
}

func TestCreateNote_WithTags(t *testing.T) {
	store := makeStore()
	fn := tools.CreateNote(store)
	resp, err := fn(context.Background(), &pluginv1.ToolRequest{
		Arguments: makeArgs(t, map[string]any{
			"title": "Tagged Note",
			"body":  "Body",
			"tags":  []any{"go", "testing"},
		}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSuccess(t, resp)
	text := responseText(t, resp)
	if !strings.Contains(text, "go, testing") {
		t.Fatalf("expected tags in response, got: %s", text)
	}
}

func TestCreateNote_MissingTitle(t *testing.T) {
	store := makeStore()
	fn := tools.CreateNote(store)
	resp, err := fn(context.Background(), &pluginv1.ToolRequest{
		Arguments: makeArgs(t, map[string]any{"body": "No title"}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertError(t, resp, "validation_error")
}

func TestCreateNote_MissingBody(t *testing.T) {
	store := makeStore()
	fn := tools.CreateNote(store)
	resp, err := fn(context.Background(), &pluginv1.ToolRequest{
		Arguments: makeArgs(t, map[string]any{"title": "No body"}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertError(t, resp, "validation_error")
}

// ---------- get_note ----------

func TestGetNote_Exists(t *testing.T) {
	store := makeStore()
	// Create a note first
	createFn := tools.CreateNote(store)
	createResp, _ := createFn(context.Background(), &pluginv1.ToolRequest{
		Arguments: makeArgs(t, map[string]any{"title": "Fetch Me", "body": "Some body"}),
	})
	text := responseText(t, createResp)
	// Extract the note ID from the response text "Created note note-XXXXXX:"
	parts := strings.Fields(text)
	var noteID string
	for _, p := range parts {
		if strings.HasPrefix(p, "note-") {
			noteID = strings.TrimSuffix(p, ":")
			break
		}
	}
	if noteID == "" {
		t.Fatalf("could not extract note ID from create response: %s", text)
	}

	getFn := tools.GetNote(store)
	resp, err := getFn(context.Background(), &pluginv1.ToolRequest{
		Arguments: makeArgs(t, map[string]any{"note_id": noteID}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSuccess(t, resp)
	got := responseText(t, resp)
	if !strings.Contains(got, "Fetch Me") {
		t.Fatalf("expected note title in get response, got: %s", got)
	}
}

func TestGetNote_MissingID(t *testing.T) {
	store := makeStore()
	fn := tools.GetNote(store)
	resp, err := fn(context.Background(), &pluginv1.ToolRequest{
		Arguments: makeArgs(t, map[string]any{}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertError(t, resp, "validation_error")
}

// ---------- list_notes ----------

func TestListNotes_Empty(t *testing.T) {
	store := makeStore()
	fn := tools.ListNotes(store)
	resp, err := fn(context.Background(), &pluginv1.ToolRequest{
		Arguments: makeArgs(t, map[string]any{}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSuccess(t, resp)
	text := responseText(t, resp)
	if !strings.Contains(text, "No notes found") {
		t.Fatalf("expected no notes message, got: %s", text)
	}
}

func TestListNotes_WithNotes(t *testing.T) {
	store := makeStore()
	createFn := tools.CreateNote(store)
	for _, title := range []string{"Note Alpha", "Note Beta"} {
		createFn(context.Background(), &pluginv1.ToolRequest{
			Arguments: makeArgs(t, map[string]any{"title": title, "body": "body"}),
		})
	}

	listFn := tools.ListNotes(store)
	resp, err := listFn(context.Background(), &pluginv1.ToolRequest{
		Arguments: makeArgs(t, map[string]any{}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSuccess(t, resp)
	text := responseText(t, resp)
	if !strings.Contains(text, "Note Alpha") {
		t.Fatalf("expected Note Alpha in list, got: %s", text)
	}
	if !strings.Contains(text, "Note Beta") {
		t.Fatalf("expected Note Beta in list, got: %s", text)
	}
}

// ---------- delete_note ----------

func TestDeleteNote_Succeeds(t *testing.T) {
	store := makeStore()
	// Create a note
	createFn := tools.CreateNote(store)
	createResp, _ := createFn(context.Background(), &pluginv1.ToolRequest{
		Arguments: makeArgs(t, map[string]any{"title": "Delete Me", "body": "gone"}),
	})
	text := responseText(t, createResp)
	var noteID string
	for _, p := range strings.Fields(text) {
		if strings.HasPrefix(p, "note-") {
			noteID = strings.TrimSuffix(p, ":")
			break
		}
	}

	deleteFn := tools.DeleteNote(store)
	resp, err := deleteFn(context.Background(), &pluginv1.ToolRequest{
		Arguments: makeArgs(t, map[string]any{"note_id": noteID}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSuccess(t, resp)
}

func TestDeleteNote_MissingID(t *testing.T) {
	store := makeStore()
	fn := tools.DeleteNote(store)
	resp, err := fn(context.Background(), &pluginv1.ToolRequest{
		Arguments: makeArgs(t, map[string]any{}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertError(t, resp, "validation_error")
}

// ---------- storage helpers ----------

func TestResolveProject_Default(t *testing.T) {
	if storage.ResolveProject("") != storage.GlobalProject {
		t.Fatalf("expected GlobalProject for empty string")
	}
}

func TestResolveProject_Custom(t *testing.T) {
	if storage.ResolveProject("my-project") != "my-project" {
		t.Fatalf("expected my-project")
	}
}

func TestNoteIDFromPath(t *testing.T) {
	id := storage.NoteIDFromPath(".global/notes/note-abc123.md")
	if id != "note-abc123" {
		t.Fatalf("expected note-abc123, got %s", id)
	}
}
