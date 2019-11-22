#!{{.Shell}}
set -e

{{range .Commands}}
{{.}} &
pid=$!
trap "kill $pid; wait; exit" 1 2 15
wait $pid
{{end}}