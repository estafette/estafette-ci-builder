#!{{.Shell}}
set -e

trap "kill -TERM $!" 0 1 2 15

{{range .Commands}}
{{.}} &
wait
{{end}}