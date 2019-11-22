#!{{.Shell}}

set -e

forward_sigterm() {
    echo "Received SIGTERM, forwarding to child processes..."
    ps aux
    trap - TERM
    pkill -P $$
}

trap forward_sigterm TERM

{{range .Commands}}
    {{- .}}
{{end}}