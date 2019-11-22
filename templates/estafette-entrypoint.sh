#!{{.Shell}}
set -e

{{- range .Commands}}
{{.}} &
trap "kill $!; wait; exit" 1 2 15
wait
{{- end}}

exec {{.FinalCommand}}