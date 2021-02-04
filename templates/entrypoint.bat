@echo off

{{- range .Commands }}
echo > {{.Command}}
{{.Command}}
{{end}}

echo > {{.FinalCommand}}
{{.FinalCommand}}