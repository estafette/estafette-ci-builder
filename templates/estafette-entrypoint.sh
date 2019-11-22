#!{{.Shell}}

set -e

{{- if .UseExec }}
{{range .Commands}}
exec {{.}}
{{end}}
{{- else }}

forward_sigterm() {
    echo "Received SIGTERM, forwarding to child processes..."
    trap - 15
    if [ -x "$(command -v pkill)" ]; then
        pkill -P $$
    elif [ -x "$(command -v ps)" ]; then
        ps -o pid --ppid $$ --no-heading | xargs kill
    else
        echo "Sorry, this container doesn't seem to have the commands (pkill, ps, xargs) to reliably kill child processes"
    fi
}

trap forward_sigterm 15

{{range .Commands}}
    {{- .}} &
    wait
{{end}}
{{end}}