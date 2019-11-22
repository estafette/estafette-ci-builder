#!{{.Shell}}
set -e

trap "kill $!" 15

{{range .Commands}}
{{.}} &
wait
{{end}}