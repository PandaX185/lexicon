package tui

import (
	"fmt"
	"strings"

	"github.com/PandaX185/lexicon/internal/installer"
)

func (m model) viewService() (string, string) {
	var b strings.Builder
	b.WriteString("Background service\n\n")
	available := m.serviceAvailable()
	if available {
		b.WriteString(toggle(m.settings.InstallService) + " Install and start the systemd user service\n\n")
		if path := m.daemonPath(); path != "" {
			b.WriteString(labelStyle.Render("Daemon") + "  " + path + "\n")
		}
		b.WriteString(subtitleStyle.Render("The service starts on login and launches the popup."))
	} else {
		b.WriteString(warnStyle.Render("The service is unavailable on this system.") + "\n")
		for _, check := range m.checks {
			if (check.Name == "daemon" || check.Name == "systemd") && check.Status != installer.StatusOK {
				b.WriteString("  " + check.Detail + "\n")
			}
		}
	}
	return b.String(), "space  toggle    enter  continue    esc  back"
}

func (m model) daemonPath() string {
	for _, check := range m.checks {
		if check.Name == "daemon" && check.Status == installer.StatusOK {
			return check.Detail
		}
	}
	return ""
}

func (m model) viewSummary() (string, string) {
	plan := m.plan
	var b strings.Builder
	b.WriteString("Summary\n\n")
	config := plan.ConfigPath + " (new)"
	if !plan.WriteConfig {
		config = plan.ConfigPath + " (kept)"
	}
	fmt.Fprintf(&b, "  %-14s %s\n", labelStyle.Render("configuration"), config)
	fmt.Fprintf(&b, "  %-14s %s\n", labelStyle.Render("dictionary"), summaryDictionary(plan))
	fmt.Fprintf(&b, "  %-14s %s -> %s\n", labelStyle.Render("languages"),
		plan.Config.Dictionary.SourceLanguage, plan.Config.Dictionary.TargetLanguage)
	fmt.Fprintf(&b, "  %-14s %s\n", labelStyle.Render("translation"), onOff(plan.Config.Translation.Enabled))
	if plan.Config.Translation.Enabled {
		modelDir := plan.Config.Translation.ModelDirectory
		if modelDir == "" {
			modelDir = "(default model directory)"
		}
		fmt.Fprintf(&b, "  %-14s %s\n", labelStyle.Render("models"), modelDir)
	}
	fmt.Fprintf(&b, "  %-14s %s\n", labelStyle.Render("service"), summaryService(plan))

	for _, note := range plan.Notes {
		b.WriteString("\n" + subtitleStyle.Render(note))
	}
	if failed := plan.FailedChecks(); len(failed) > 0 {
		b.WriteString("\n")
		for _, check := range failed {
			b.WriteString(failStyle.Render("✗ "+check.Name+": "+check.Detail) + "\n")
		}
	}
	if m.options.DryRun {
		b.WriteString("\n" + warnStyle.Render("Dry run: no changes will be made."))
	}
	return b.String(), summaryHelp(m.options.DryRun)
}

func summaryHelp(dryRun bool) string {
	if dryRun {
		return "enter  finish (dry run)    esc  back    q  quit"
	}
	return "enter  install    esc  back    q  quit"
}

func summaryDictionary(plan installer.Plan) string {
	switch {
	case plan.DictionaryPackaged:
		version := ""
		if plan.DictionaryVersion != "" {
			version = " " + plan.DictionaryVersion
		}
		return plan.DictionaryTarget + " (packaged" + version + ", no copy)"
	case plan.DownloadURL != "":
		return plan.DownloadURL + " -> " + plan.DictionaryTarget
	case plan.CopyDictionary:
		return plan.DictionarySource + " -> " + plan.DictionaryTarget
	case plan.DictionaryTarget != "":
		return plan.DictionaryTarget + " (unchanged)"
	default:
		return "not configured"
	}
}

func summaryService(plan installer.Plan) string {
	switch {
	case plan.InstallUnit:
		return plan.UnitPath
	case plan.EnableUnit:
		return "existing unit at " + plan.UnitPath + " will be started"
	default:
		return "not installed"
	}
}

func (m model) viewProgress() (string, string) {
	var b strings.Builder
	b.WriteString("Installing\n\n")
	if m.progress.Total > 0 {
		fmt.Fprintf(&b, "  [%d/%d] %s\n", m.progress.Index, m.progress.Total, m.progress.Step)
	} else {
		b.WriteString("  " + m.progress.Step + "\n")
	}
	b.WriteString("\n" + m.spinner.View() + " working...\n")
	return b.String(), "installing — please wait"
}

func (m model) viewError() (string, string) {
	err := m.applyErr
	if err == nil {
		err = m.planErr
	}
	var b strings.Builder
	b.WriteString(failStyle.Render("Setup failed") + "\n\n")
	if err != nil {
		b.WriteString(err.Error() + "\n")
	}
	b.WriteString("\n")
	if m.applyErr != nil {
		b.WriteString("Previous files were restored where possible.\n")
	}
	b.WriteString("Run `lexicon doctor` for more detail.\n")
	return b.String(), "esc  try again    q  quit"
}
