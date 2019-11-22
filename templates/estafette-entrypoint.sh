#!{{.Shell}}
set -e

sigterm_handler() {
    echo "Received SIGTERM, forwarding to forked processes"
    ps aux
}

trap 'kill ${!}; sigterm_handler' 15 # SIGTERM

{{range .Commands}}
    {{- .}}
{{end}}