#!{{.Shell}}

set -e

forward_sigterm() {
    echo "Received SIGTERM, forwarding to child processes..."
    ps -o pid,ppid,user,comm
    trap - TERM
    pkill -P $$
}

trap forward_sigterm TERM

{{range .Commands}}
    {{- .}} &
    wait
{{end}}