#!{{.Shell}}

set -e

# 15 - SIGTERM
trap "graceful_exit" 15

graceful_exit() {
    echo "Received SIGTERM, forwarding to child processes..."
    trap - 15
    pkill -P $$
    exit 0
}

{{range .Commands}}
    {{- .}} &
    wait $!
{{end}}