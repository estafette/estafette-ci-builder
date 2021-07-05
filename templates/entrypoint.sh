#!{{.Shell}}
set -e

{{ range .Commands -}}
{{ if .RunInBackground -}}
printf '\033[38;5;250m> %s &\033[0m\n' $'{{.EscapedCommand}}'
{{.Command}} &
trap "kill $!; wait; exit" 1 2 15
wait $!

{{ else -}}
printf '\033[38;5;250m> %s\033[0m\n' $'{{.EscapedCommand}}'
{{.Command}}

{{ end -}}
{{ end -}}
{{ if .RunFinalCommandWithExec -}}
printf '\033[38;5;250m> exec %s\033[0m\n' $'{{.EscapedFinalCommand}}'
exec {{.FinalCommand -}}
{{ else -}}
printf '\033[38;5;250m> %s\033[0m\n' $'{{.EscapedFinalCommand}}'
{{.FinalCommand -}}
{{ end -}}