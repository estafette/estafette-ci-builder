#!{{.Shell}}

set -e

forward_sigterm() {
    echo "Received SIGTERM, forwarding to child processes..."
    trap - TERM INT QUIT KILL STOP
    pkill -P $$
}
trap 'forward_sigterm' TERM INT QUIT KILL STOP
trap

{{range .Commands}}
    {{- .}} &
    wait $!
{{end}}