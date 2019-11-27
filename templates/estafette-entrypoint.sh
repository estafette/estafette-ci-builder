#!{{.Shell}}
set -e

{{- range .Commands }}
{{- if .RunInBackground}}
echo -e $'\x1b[38;5;250m> {{.EscapedCommand}} &\x1b[0m'
{{.Command}} &
trap "kill $!; wait; exit" 1 2 15
wait
{{- else }}
echo -e $'\x1b[38;5;250m> {{.EscapedCommand}}\x1b[0m'
{{.Command}}
{{- end }}
{{- end }}
{{- if .RunFinalCommandWithExec}}

echo -e $'\x1b[38;5;250m> exec {{.EscapedFinalCommand}}\x1b[0m'
exec {{.FinalCommand}}
{{- else }}

echo -e $'\x1b[38;5;250m> {{.EscapedFinalCommand}}\x1b[0m'
{{.FinalCommand}}
{{- end }}