package installer

import (
	"context"
	"fmt"
	"strings"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/PandaX185/lexicon/internal/translation"
)

// PackProgress reports one pack download in the installer UI.
type PackProgress struct {
	ID    string
	Index int
	Total int
}

func (p PackProgress) Step() string { return "Installing model pack " + p.ID }

// PackResult is the outcome of one explicit pack install.
type PackResult struct {
	ID     string
	Source string
	Target string
	Error  error
}

func (r PackResult) RetryCommand() string { return "lexicon languages install " + r.ID }

// InstallPacks installs selected packs after Apply. Failures are returned as
// data so the base install is never rolled back.
func InstallPacks(ctx context.Context, options Options, plan Plan, report func(PackProgress)) []PackResult {
	if len(plan.PackIDs) == 0 {
		return nil
	}
	entries := options.PackCatalog
	var err error
	if entries == nil {
		entries, err = translation.Catalog()
		if err != nil {
			return []PackResult{{ID: strings.Join(plan.PackIDs, ","), Error: fmt.Errorf("read model catalog: %w", err)}}
		}
	}
	byID := make(map[string]translation.CatalogEntry, len(entries))
	for _, entry := range entries {
		byID[entry.ID] = entry
	}
	install := options.PackInstall
	if install == nil {
		install = translation.InstallPack
	}
	effective := config.Effective(plan.Config)
	modelDir := effective.Translation.ModelDirectory

	results := make([]PackResult, 0, len(plan.PackIDs))
	for index, id := range plan.PackIDs {
		if report != nil {
			report(PackProgress{ID: id, Index: index + 1, Total: len(plan.PackIDs)})
		}
		entry, found := byID[id]
		result := PackResult{ID: id, Source: entry.Source, Target: entry.Target}
		switch {
		case !found:
			result.Error = fmt.Errorf("unknown model pack %q", id)
		case modelDir == "":
			result.Error = fmt.Errorf("no model directory resolved for %s", id)
		default:
			_, result.Error = install(ctx, entry, modelDir, options.HTTPClient)
		}
		results = append(results, result)
	}
	return results
}
