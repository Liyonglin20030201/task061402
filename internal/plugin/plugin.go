package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Liyonglin20030201/task061402/internal/config"
	"github.com/Liyonglin20030201/task061402/internal/connector"
	"github.com/Liyonglin20030201/task061402/internal/inspector"
)

type Plugin interface {
	Name() string
	Description() string
	Run(ctx context.Context, conn connector.Connector, cfg *config.Config) (*inspector.Result, error)
}

type PluginInfo struct {
	Name        string
	Path        string
	Description string
	Type        string
}

func Discover(pluginDir string) ([]PluginInfo, error) {
	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin directory: %w", err)
	}

	var plugins []PluginInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := filepath.Ext(name)

		switch ext {
		case ".sh", ".bash":
			plugins = append(plugins, PluginInfo{
				Name:        strings.TrimSuffix(name, ext),
				Path:        filepath.Join(pluginDir, name),
				Description: "Shell script plugin",
				Type:        "script",
			})
		case ".py":
			plugins = append(plugins, PluginInfo{
				Name:        strings.TrimSuffix(name, ext),
				Path:        filepath.Join(pluginDir, name),
				Description: "Python plugin",
				Type:        "script",
			})
		case ".exe", "":
			if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".txt") {
				plugins = append(plugins, PluginInfo{
					Name:        strings.TrimSuffix(name, ext),
					Path:        filepath.Join(pluginDir, name),
					Description: "Executable plugin",
					Type:        "binary",
				})
			}
		}
	}

	return plugins, nil
}
