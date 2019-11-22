#!{{.Shell}}
set -e

trap 'echo "Forwarding SIGTERM..."; kill $pid' 15

{{range .Commands}}
{{.}} &
pid=$!
wait $pid
{{end}}