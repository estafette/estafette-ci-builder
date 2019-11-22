#!{{.Shell}}
set -ex

{{range .Commands}}
{{.}} &
pid=$!
trap "kill $pid" 1 2 15
wait $pid
{{end}}