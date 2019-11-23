#!{{.Shell}}
set -e

{{- range .Commands}}
echo -e "\x1b[1m> {{.}}\x1b[0m"
{{.}} &
trap "kill $!; wait; exit" 1 2 15
wait
{{- end}}

echo -e "\x1b[1m> {{.FinalCommand}}\x1b[0m"
exec {{.FinalCommand}}