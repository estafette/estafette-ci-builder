#!{{.Shell}}
set -ex

{{range .Commands}}
{{.}} &
pid=$!
trap "kill $pid" 0 1 2 15
wait $pid
{{end}}