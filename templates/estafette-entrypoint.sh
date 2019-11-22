#!{{.Shell}}
set -e

{{range .Commands}}
{{.}} &
trap "kill $!" 15
wait
{{end}}