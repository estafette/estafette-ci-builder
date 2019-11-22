#!{{.Shell}}

set -e

forward_sigterm() {
    echo "Received SIGTERM, forwarding to forked processes..."
    trap - 15
    kill -- -$$
}

trap forward_sigterm 15

{{range .Commands}}
    {{- .}}
{{end}}