#!{{.Shell}}

set -ex

forward_sigterm() {
    echo "Received SIGTERM, forwarding to child processes..."
    trap - 2 3 15
    pkill -P $$
    sleep 1
    exit 0
}

# 2  - SIGINT
# 3  - SIGQUIT
# 15 - SIGTERM
trap "forward_sigterm" 2 3 15
trap "echo 2" 2
trap "echo 3" 3
trap "echo 15" 15
trap

{{range .Commands}}
    {{- .}} &
    wait $!
{{end}}