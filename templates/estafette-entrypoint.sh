#!{{.Shell}}

set -e

forward_sigterm() {
    echo "Received SIGTERM, forwarding to child processes..."
    trap - TERM
    pkill -P $$
}

trap forward_sigterm TERM

{{range .Commands}}
    {{- .}} &
    wait
{{end}}