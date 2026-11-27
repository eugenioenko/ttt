package app

import "strings"

func (a *App) appendDebugOutput(run *debugRun, name, category, chunk string) {
	if category == "telemetry" {
		return
	}
	if run.output == nil {
		run.output = make(map[string]string)
	}
	chunk = run.output[category] + chunk
	for {
		line, rest, ok := strings.Cut(chunk, "\n")
		if !ok {
			break
		}
		a.LogOutput("info", "debug:"+name, strings.TrimSuffix(line, "\r"))
		chunk = rest
	}
	if len(chunk) > 65536 || category != "stdout" && category != "stderr" {
		if chunk != "" {
			a.LogOutput("info", "debug:"+name, chunk)
		}
		chunk = ""
	}
	run.output[category] = chunk
}
func (a *App) flushDebugOutput(run *debugRun, name string) {
	for _, chunk := range run.output {
		if chunk != "" {
			a.LogOutput("info", "debug:"+name, chunk)
		}
	}
	clear(run.output)
}
