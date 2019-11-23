#!{{.Shell}}
set -e

{{- range .Commands}}
echo "\x1b[1m> {{.}}\x1b[0m"
{{.}} &
trap "kill $!; wait; exit" 1 2 15
wait
{{- end}}

exec {{.FinalCommand}}