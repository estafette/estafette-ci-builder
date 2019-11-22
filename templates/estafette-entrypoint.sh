#!{{.Shell}}

set -e

forward_sigterm() {
    echo "Received SIGTERM, forwarding to child processes..."
    trap - 15
    pkill -P $$
}

trap forward_sigterm 15

{{range .Commands}}
    {{- .}} &
    wait
{{end}}