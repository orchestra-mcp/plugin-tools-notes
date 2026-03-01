// Package storage provides an abstraction over the orchestrator's storage
// protocol for reading and writing note data as markdown files with YAML
// frontmatter metadata.
package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	pluginv1 "github.com/orchestra-mcp/gen-go/orchestra/plugin/v1"
	"github.com/orchestra-mcp/sdk-go/helpers"
	"google.golang.org/protobuf/types/known/structpb"
)

// NotesDir is the subdirectory within a project that holds note files.
const NotesDir = "notes"

// GlobalProject is the project slug used for unscoped notes.
const GlobalProject = ".global"

// StorageClient is the interface that tool handlers use to communicate with the
// orchestrator. In production this is backed by a QUIC OrchestratorClient.
type StorageClient interface {
	Send(ctx context.Context, req *pluginv1.PluginRequest) (*pluginv1.PluginResponse, error)
}

// DataStorage provides high-level operations for reading and writing note data.
type DataStorage struct {
	client StorageClient
}

// NewDataStorage creates a new DataStorage backed by the given client.
func NewDataStorage(client StorageClient) *DataStorage {
	return &DataStorage{client: client}
}

// ResolveProject returns the project slug to use, defaulting to GlobalProject.
func ResolveProject(projectID string) string {
	if projectID == "" {
		return GlobalProject
	}
	return projectID
}

// notePath returns the storage path for a note.
func notePath(projectSlug, noteID string) string {
	return filepath.Join(projectSlug, NotesDir, noteID+".md")
}

// ReadNote loads a note by project slug and note ID. Returns the metadata struct,
// the body content, and the version number.
func (ds *DataStorage) ReadNote(ctx context.Context, projectSlug, noteID string) (*structpb.Struct, string, int64, error) {
	path := notePath(projectSlug, noteID)
	resp, err := ds.storageRead(ctx, path)
	if err != nil {
		return nil, "", 0, fmt.Errorf("read note %s/%s: %w", projectSlug, noteID, err)
	}
	return resp.Metadata, string(resp.Content), resp.Version, nil
}

// WriteNote persists a note to storage with the given metadata and body.
func (ds *DataStorage) WriteNote(ctx context.Context, projectSlug, noteID string, metadata *structpb.Struct, body string, expectedVersion int64) (int64, error) {
	path := notePath(projectSlug, noteID)
	return ds.storageWrite(ctx, path, metadata, []byte(body), expectedVersion)
}

// DeleteNote removes a note file from storage.
func (ds *DataStorage) DeleteNote(ctx context.Context, projectSlug, noteID string) error {
	path := notePath(projectSlug, noteID)
	return ds.storageDelete(ctx, path)
}

// ListNotes returns all note storage entries for a project.
func (ds *DataStorage) ListNotes(ctx context.Context, projectSlug string) ([]*pluginv1.StorageEntry, error) {
	prefix := filepath.Join(projectSlug, NotesDir) + string(filepath.Separator)
	return ds.storageList(ctx, prefix, "*.md")
}

// NoteIDFromPath extracts the note ID from a storage entry path.
func NoteIDFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, ".md")
}

// ---------- Low-level storage protocol ----------

func (ds *DataStorage) storageRead(ctx context.Context, path string) (*pluginv1.StorageReadResponse, error) {
	resp, err := ds.client.Send(ctx, &pluginv1.PluginRequest{
		RequestId: helpers.NewUUID(),
		Request: &pluginv1.PluginRequest_StorageRead{
			StorageRead: &pluginv1.StorageReadRequest{
				Path:        path,
				StorageType: "markdown",
			},
		},
	})
	if err != nil {
		return nil, err
	}
	sr := resp.GetStorageRead()
	if sr == nil {
		return nil, fmt.Errorf("unexpected response type for storage read")
	}
	return sr, nil
}

func (ds *DataStorage) storageWrite(ctx context.Context, path string, metadata *structpb.Struct, content []byte, expectedVersion int64) (int64, error) {
	resp, err := ds.client.Send(ctx, &pluginv1.PluginRequest{
		RequestId: helpers.NewUUID(),
		Request: &pluginv1.PluginRequest_StorageWrite{
			StorageWrite: &pluginv1.StorageWriteRequest{
				Path:            path,
				Content:         content,
				Metadata:        metadata,
				ExpectedVersion: expectedVersion,
				StorageType:     "markdown",
			},
		},
	})
	if err != nil {
		return 0, err
	}
	sw := resp.GetStorageWrite()
	if sw == nil {
		return 0, fmt.Errorf("unexpected response type for storage write")
	}
	if !sw.Success {
		return 0, fmt.Errorf("storage write failed: %s", sw.Error)
	}
	return sw.NewVersion, nil
}

func (ds *DataStorage) storageDelete(ctx context.Context, path string) error {
	resp, err := ds.client.Send(ctx, &pluginv1.PluginRequest{
		RequestId: helpers.NewUUID(),
		Request: &pluginv1.PluginRequest_StorageDelete{
			StorageDelete: &pluginv1.StorageDeleteRequest{
				Path:        path,
				StorageType: "markdown",
			},
		},
	})
	if err != nil {
		return err
	}
	sd := resp.GetStorageDelete()
	if sd == nil {
		return fmt.Errorf("unexpected response type for storage delete")
	}
	if !sd.Success {
		return fmt.Errorf("storage delete failed")
	}
	return nil
}

func (ds *DataStorage) storageList(ctx context.Context, prefix, pattern string) ([]*pluginv1.StorageEntry, error) {
	resp, err := ds.client.Send(ctx, &pluginv1.PluginRequest{
		RequestId: helpers.NewUUID(),
		Request: &pluginv1.PluginRequest_StorageList{
			StorageList: &pluginv1.StorageListRequest{
				Prefix:      prefix,
				Pattern:     pattern,
				StorageType: "markdown",
			},
		},
	})
	if err != nil {
		return nil, err
	}
	sl := resp.GetStorageList()
	if sl == nil {
		return nil, fmt.Errorf("unexpected response type for storage list")
	}
	return sl.Entries, nil
}
