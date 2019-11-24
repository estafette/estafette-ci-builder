#!{{.Shell}}
set -e

{{- range .Commands }}
echo -e "\x1b[38;5;250m> {{.Command}}\x1b[0m"
{{- if .RunInBackground}}
{{.Command}} &
trap "kill $!; wait; exit" 1 2 15
wait
{{- else }}
{{.Command}}
{{- end }}
{{- end }}

echo -e "\x1b[38;5;250m> {{.FinalCommand}}\x1b[0m"
exec {{.FinalCommand}}