#!{{.Shell}}
set -ex

{{range .Commands}}
{{.}} &
trap "echo 'Trapped signal...'; kill $!" 0 1 2 15
wait
{{end}}