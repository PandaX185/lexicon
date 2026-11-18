package installer

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/PandaX185/lexicon/internal/translation"
)

func TestInstallPacksContinuesAfterFailure(t *testing.T) {
	entries := []translation.CatalogEntry{
		{ID: "pack-a", Name: "A", Source: "en", Target: "de"},
		{ID: "pack-b", Name: "B", Source: "de", Target: "en"},
	}
	options := Options{
		PackCatalog: entries,
		PackInstall: func(_ context.Context, entry translation.CatalogEntry, _ string, _ *http.Client) (string, error) {
			if entry.ID == "pack-a" {
				return "", errors.New("checksum mismatch")
			}
			return "/tmp/pack-b", nil
		},
	}
	plan := Plan{
		PackIDs: []string{"pack-a", "pack-b"},
		Config:  config.Config{},
	}
	plan.Config.Translation.ModelDirectory = t.TempDir()

	var progress []PackProgress
	results := InstallPacks(context.Background(), options, plan, func(p PackProgress) {
		progress = append(progress, p)
	})

	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if results[0].Error == nil || !strings.Contains(results[0].Error.Error(), "checksum mismatch") {
		t.Fatalf("first result error = %v", results[0].Error)
	}
	if results[1].Error != nil {
		t.Fatalf("second result error = %v", results[1].Error)
	}
	if results[0].RetryCommand() != "lexicon languages install pack-a" {
		t.Fatalf("retry = %q", results[0].RetryCommand())
	}
	if len(progress) != 2 || progress[1].Index != 2 || progress[1].Total != 2 {
		t.Fatalf("progress = %+v", progress)
	}
}
