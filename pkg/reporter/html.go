package reporter

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// HTMLReporter generates a self-contained HTML report.
type HTMLReporter struct {
	outputDir string
}

// NewHTMLReporter creates an HTML reporter that writes to the given directory.
func NewHTMLReporter(outputDir string) *HTMLReporter {
	return &HTMLReporter{outputDir: outputDir}
}

func (h *HTMLReporter) StartSuite(name string)                                                 {}
func (h *HTMLReporter) StartSpec(name string)                                                  {}
func (h *HTMLReporter) StepResult(phase string, stepIndex int, stepText string, o StepOutcome) {}
func (h *HTMLReporter) EndSpec(report *TestReport)                                             {}
func (h *HTMLReporter) EndSuite(report *SuiteReport)                                           {}

func (h *HTMLReporter) Finalize(report *SuiteReport) error {
	if h.outputDir == "" {
		return nil
	}
	if err := os.MkdirAll(h.outputDir, 0755); err != nil {
		return fmt.Errorf("creating html output dir: %w", err)
	}

	filename := filepath.Join(h.outputDir, fmt.Sprintf("report_%s_%d.html", sanitizeFilename(report.Name), time.Now().Unix()))
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("creating html report: %w", err)
	}
	defer f.Close()

	total := len(report.Tests)
	var passRate, failRate, skipRate float64
	if total > 0 {
		passRate = float64(report.Passed) / float64(total) * 100
		failRate = float64(report.Failed) / float64(total) * 100
		skipRate = float64(report.Skipped) / float64(total) * 100
	}
	data := htmlData{
		Suite:     report,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Total:     total,
		PassRate:  passRate,
		FailRate:  failRate,
		SkipRate:  skipRate,
	}

	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"statusClass": func(s TestStatus) string {
			switch s {
			case TestPassed:
				return "pass"
			case TestFailed:
				return "fail"
			default:
				return "skip"
			}
		},
		"statusLabel": func(s TestStatus) string {
			return strings.ToUpper(string(s))
		},
		"stepStatusClass": func(s StepStatus) string {
			switch s {
			case StepPassed:
				return "pass"
			case StepFailed:
				return "fail"
			default:
				return "skip"
			}
		},
		"fmtDuration": func(d time.Duration) string {
			if d < time.Second {
				return fmt.Sprintf("%dms", d.Milliseconds())
			}
			return fmt.Sprintf("%.1fs", d.Seconds())
		},
		"groupSteps": groupStepsByPhase,
	}).Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("parsing html template: %w", err)
	}

	return tmpl.Execute(f, data)
}

type htmlData struct {
	Suite     *SuiteReport
	Timestamp string
	Total     int
	PassRate  float64
	FailRate  float64
	SkipRate  float64
}

type phaseGroup struct {
	Phase string
	Steps []StepReport
}

func groupStepsByPhase(steps []StepReport) []phaseGroup {
	var groups []phaseGroup
	seen := make(map[string]int)
	for _, s := range steps {
		if idx, ok := seen[s.Phase]; ok {
			groups[idx].Steps = append(groups[idx].Steps, s)
		} else {
			seen[s.Phase] = len(groups)
			groups = append(groups, phaseGroup{Phase: s.Phase, Steps: []StepReport{s}})
		}
	}
	return groups
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>SDT Report — {{.Suite.Name}}</title>
<style>
* { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #f5f5f5; color: #333; line-height: 1.5; }
.header { background: #1a1a2e; color: #fff; padding: 24px 32px; }
.header h1 { font-size: 1.4em; font-weight: 600; }
.header .meta { color: #aaa; font-size: 0.85em; margin-top: 4px; }
.summary { display: flex; gap: 24px; padding: 20px 32px; background: #fff; border-bottom: 1px solid #e0e0e0; flex-wrap: wrap; align-items: center; }
.stat { text-align: center; }
.stat .num { font-size: 1.8em; font-weight: 700; }
.stat .label { font-size: 0.75em; color: #666; text-transform: uppercase; }
.stat.pass .num { color: #2e7d32; }
.stat.fail .num { color: #c62828; }
.stat.skip .num { color: #ef6c00; }
.bar-container { flex: 1; min-width: 200px; }
.bar { height: 12px; border-radius: 6px; overflow: hidden; display: flex; background: #e0e0e0; }
.bar .pass { background: #4caf50; }
.bar .fail { background: #e53935; }
.bar .skip { background: #ff9800; }
.bar-label { font-size: 0.8em; color: #666; margin-top: 2px; }
.tests { padding: 16px 32px; }
details.test { background: #fff; border: 1px solid #e0e0e0; border-radius: 6px; margin-bottom: 8px; }
details.test summary { padding: 12px 16px; cursor: pointer; display: flex; align-items: center; gap: 12px; }
details.test summary:hover { background: #fafafa; }
.badge { padding: 2px 10px; border-radius: 4px; font-size: 0.75em; font-weight: 600; color: #fff; text-transform: uppercase; }
.badge.pass { background: #4caf50; }
.badge.fail { background: #e53935; }
.badge.skip { background: #ff9800; }
.test-name { flex: 1; font-weight: 500; }
.test-dur { color: #999; font-size: 0.85em; }
.test-file { color: #999; font-size: 0.8em; }
.test-body { padding: 0 16px 16px; border-top: 1px solid #eee; }
.flaky-tag { background: #fff3e0; color: #e65100; padding: 1px 6px; border-radius: 3px; font-size: 0.7em; font-weight: 600; }
.phase-title { font-size: 0.85em; font-weight: 600; text-transform: uppercase; color: #666; margin: 12px 0 6px; }
.step { display: flex; align-items: flex-start; gap: 8px; padding: 4px 0; font-size: 0.9em; }
.step-icon.pass { color: #4caf50; }
.step-icon.fail { color: #e53935; }
.step-icon.skip { color: #ff9800; }
.step-err { background: #fbe9e7; color: #c62828; padding: 6px 10px; border-radius: 4px; font-size: 0.8em; margin: 4px 0 4px 24px; }
.diagnosis { background: #fff8e1; border-left: 4px solid #ff9800; padding: 12px; margin: 12px 0; font-size: 0.85em; white-space: pre-wrap; }
.test-error { background: #fbe9e7; border-left: 4px solid #e53935; padding: 12px; margin: 12px 0; font-size: 0.85em; }
@media print { .header { background: #333; } details.test { break-inside: avoid; } }
</style>
</head>
<body>
<div class="header">
  <h1>SDT Report — {{.Suite.Name}}</h1>
  <div class="meta">Generated {{.Timestamp}} | Duration: {{fmtDuration .Suite.Duration}}</div>
</div>
<div class="summary">
  <div class="stat pass"><div class="num">{{.Suite.Passed}}</div><div class="label">Passed</div></div>
  <div class="stat fail"><div class="num">{{.Suite.Failed}}</div><div class="label">Failed</div></div>
  <div class="stat skip"><div class="num">{{.Suite.Skipped}}</div><div class="label">Skipped</div></div>
  <div class="bar-container">
    <div class="bar">
      {{if gt .Suite.Passed 0}}<div class="pass" style="width:{{printf "%.1f" .PassRate}}%"></div>{{end}}
      {{if gt .Suite.Failed 0}}<div class="fail" style="width:{{printf "%.1f" .FailRate}}%"></div>{{end}}
      {{if gt .Suite.Skipped 0}}<div class="skip" style="width:{{printf "%.1f" .SkipRate}}%"></div>{{end}}
    </div>
    <div class="bar-label">{{printf "%.0f" .PassRate}}% pass rate ({{.Total}} total)</div>
  </div>
</div>
<div class="tests">
{{range .Suite.Tests}}
<details class="test"{{if eq .Status "failed"}} open{{end}}>
  <summary>
    <span class="badge {{statusClass .Status}}">{{statusLabel .Status}}</span>
    <span class="test-name">{{.Name}}</span>
    {{if .Flaky}}<span class="flaky-tag">FLAKY (retry {{.Retries}})</span>{{end}}
    <span class="test-dur">{{fmtDuration .Duration}}</span>
    <span class="test-file">{{.FilePath}}</span>
  </summary>
  <div class="test-body">
    {{if .Error}}<div class="test-error">{{.Error}}</div>{{end}}
    {{range groupSteps .Steps}}
    <div class="phase-title">{{.Phase}}</div>
    {{range .Steps}}
    <div class="step">
      <span class="step-icon {{stepStatusClass .Outcome.Status}}">{{if eq .Outcome.Status "PASSED"}}&#10003;{{else if eq .Outcome.Status "FAILED"}}&#10007;{{else}}&#8212;{{end}}</span>
      <span>{{.Text}}</span>
      <span class="test-dur">{{fmtDuration .Outcome.Duration}}</span>
    </div>
    {{if .Outcome.Error}}<div class="step-err">{{.Outcome.Error}}</div>{{end}}
    {{end}}
    {{end}}
    {{if .Diagnosis}}<div class="diagnosis"><strong>Auto-Debug Diagnosis</strong><br>{{.Diagnosis}}</div>{{end}}
  </div>
</details>
{{end}}
</div>
</body>
</html>`
