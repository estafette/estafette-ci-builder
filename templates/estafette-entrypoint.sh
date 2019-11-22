#!{{.Shell}}

set -e

forward_sigterm() {
    echo "Received SIGTERM, forwarding to child processes..."
    trap - TERM QUIT
    pkill -P $$
}
trap 'forward_sigterm' TERM INT QUIT
trap

{{range .Commands}}
    {{- .}} &
    wait $!
{{end}}