#!{{.Shell}}

set -e

sigterm_handler() {
    echo "Received SIGTERM, forwarding to forked processes"
    trap - 15
    kill -- -$$ # Sends SIGTERM to child/sub processes
}

trap sigterm_handler 15

{{range .Commands}}
    {{- .}}
{{end}}