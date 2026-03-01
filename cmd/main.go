// Command tools-notes is the entry point for the tools.notes plugin binary.
// It creates the plugin with all 8 note management tools and connects to the
// orchestrator for storage access.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	pluginv1 "github.com/orchestra-mcp/gen-go/orchestra/plugin/v1"
	"github.com/orchestra-mcp/sdk-go/plugin"
	"github.com/orchestra-mcp/plugin-tools-notes/internal"
	"github.com/orchestra-mcp/plugin-tools-notes/internal/storage"
)

func main() {
	builder := plugin.New("tools.notes").
		Version("0.1.0").
		Description("Note management tools plugin with 8 tools").
		Author("Orchestra").
		Binary("tools-notes").
		NeedsStorage("markdown")

	adapter := &clientAdapter{}
	store := storage.NewDataStorage(adapter)

	tp := &internal.ToolsPlugin{Storage: store}
	tp.RegisterTools(builder)

	p := builder.BuildWithTools()
	p.ParseFlags()
	adapter.plugin = p

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	if err := p.Run(ctx); err != nil {
		log.Fatalf("tools.notes: %v", err)
	}
}

// clientAdapter implements storage.StorageClient by forwarding to the plugin's
// orchestrator client. This allows tool handlers to use storage operations
// through the QUIC connection that is established during Run.
type clientAdapter struct {
	plugin *plugin.Plugin
}

func (a *clientAdapter) Send(ctx context.Context, req *pluginv1.PluginRequest) (*pluginv1.PluginResponse, error) {
	client := a.plugin.OrchestratorClient()
	if client == nil {
		return nil, fmt.Errorf("orchestrator client not connected")
	}
	return client.Send(ctx, req)
}
