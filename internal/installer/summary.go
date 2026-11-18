package installer

import (
	"fmt"
	"io"
	"strings"
)

// WriteChecks prints preflight results for the non-interactive path.
func WriteChecks(w io.Writer, checks []Check) {
	for _, check := range checks {
		fmt.Fprintf(w, "  %-4s %-18s %s\n", check.Status, check.Name, check.Detail)
	}
}

// WriteSummary prints the resolved plan for the non-interactive path.
func WriteSummary(w io.Writer, plan Plan) {
	fmt.Fprintln(w, "\nSummary")
	switch {
	case plan.WriteConfig:
		fmt.Fprintf(w, "  config:      %s\n", plan.ConfigPath)
	case plan.ConfigExists:
		fmt.Fprintf(w, "  config:      %s (kept)\n", plan.ConfigPath)
	default:
		fmt.Fprintf(w, "  config:      %s\n", plan.ConfigPath)
	}

	switch {
	case plan.DictionaryPackaged:
		version := ""
		if plan.DictionaryVersion != "" {
			version = " " + plan.DictionaryVersion
		}
		fmt.Fprintf(w, "  dictionary:  %s (packaged%s, no copy)\n", plan.DictionaryTarget, version)
	case plan.DownloadURL != "":
		fmt.Fprintf(w, "  dictionary:  %s -> %s\n", plan.DownloadURL, plan.DictionaryTarget)
	case plan.CopyDictionary:
		fmt.Fprintf(w, "  dictionary:  %s -> %s\n", plan.DictionarySource, plan.DictionaryTarget)
	case plan.DictionaryTarget != "":
		fmt.Fprintf(w, "  dictionary:  %s (unchanged)\n", plan.DictionaryTarget)
	default:
		fmt.Fprintln(w, "  dictionary:  not configured")
	}

	fmt.Fprintf(w, "  languages:   %s -> %s\n", plan.Config.Dictionary.SourceLanguage, plan.Config.Dictionary.TargetLanguage)
	fmt.Fprintf(w, "  translation: %t\n", plan.Config.Translation.Enabled)
	if plan.Config.Translation.Enabled {
		modelDir := plan.Config.Translation.ModelDirectory
		if modelDir == "" {
			modelDir = "(default model directory)"
		}
		fmt.Fprintf(w, "  models:      %s\n", modelDir)
	}
	if len(plan.PackIDs) > 0 {
		fmt.Fprintf(w, "  packs:       %s\n", strings.Join(plan.PackIDs, ", "))
	}
	switch {
	case plan.InstallUnit:
		fmt.Fprintf(w, "  service:     %s\n", plan.UnitPath)
	case plan.EnableUnit:
		fmt.Fprintf(w, "  service:     existing unit at %s will be started\n", plan.UnitPath)
	default:
		fmt.Fprintln(w, "  service:     not installed")
	}
	for _, note := range plan.Notes {
		fmt.Fprintf(w, "  note:        %s\n", note)
	}
}

// WritePackResults prints best-effort pack outcomes and retry commands.
func WritePackResults(w io.Writer, results []PackResult) {
	if len(results) == 0 {
		return
	}
	fmt.Fprintln(w, "\nModel packs")
	for _, result := range results {
		if result.Error == nil {
			fmt.Fprintf(w, "  installed  %s (%s -> %s)\n", result.ID, result.Source, result.Target)
			continue
		}
		fmt.Fprintf(w, "  failed     %s: %v\n", result.ID, result.Error)
		fmt.Fprintf(w, "             retry: %s\n", result.RetryCommand())
	}
}

// WriteNextSteps prints the commands a user needs after a successful install.
func WriteNextSteps(w io.Writer, plan Plan) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Installed.")
	fmt.Fprintln(w, "Next steps:")
	fmt.Fprintln(w, "  lex doctor")
	if plan.InstallUnit || plan.EnableUnit {
		fmt.Fprintln(w, "  systemctl --user status lexicon.service")
	} else {
		fmt.Fprintln(w, "  lexicon-daemon --config "+plan.ConfigPath)
	}
	fmt.Fprintln(w, "  systemctl --user import-environment WAYLAND_DISPLAY   # for popup auto-launch")
}
