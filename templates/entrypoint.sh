#!{{.Shell}}
set -e

{{- range .Commands }}
{{- if .RunInBackground}}
printf "\033[38;5;250m> {{.EscapedCommand}} &\033[0m"
{{.Command}} &
trap "kill $!; wait; exit" 1 2 15
wait $!
{{- else }}
printf "\033[38;5;250m> {{.EscapedCommand}}\033[0m"
{{.Command}}
{{- end }}
{{- end }}
{{- if .RunFinalCommandWithExec}}

printf "\033[38;5;250m> exec {{.EscapedFinalCommand}}\033[0m"
exec {{.FinalCommand}}
{{- else }}

printf "\033[38;5;250m> {{.EscapedFinalCommand}}\033[0m"
{{.FinalCommand}}
{{- end }}