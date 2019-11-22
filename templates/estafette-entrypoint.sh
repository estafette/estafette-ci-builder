#!{{.Shell}}
set -e

trap "kill $pid; wait; exit" 1 2 15

{{range .Commands}}
{{.}} &
pid=$!
wait $pid
{{end}}