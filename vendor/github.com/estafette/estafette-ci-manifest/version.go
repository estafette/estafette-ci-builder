package manifest

import (
	"bytes"
	"fmt"
	"html/template"
)

// EstafetteVersion is the object that determines how version numbers are generated
type EstafetteVersion struct {
	SemVer *EstafetteSemverVersion `yaml:"semver,omitempty"`
	Custom *EstafetteCustomVersion `yaml:"custom,omitempty"`
}

// Version returns the version number as a string
func (v *EstafetteVersion) Version(params EstafetteVersionParams) string {
	if v.Custom != nil {
		return v.Custom.Version(params)
	}
	if v.SemVer != nil {
		return v.SemVer.Version(params)
	}
	return ""
}

// EstafetteCustomVersion represents a custom version using a template
type EstafetteCustomVersion struct {
	LabelTemplate string `yaml:"labelTemplate,omitempty"`
}

// Version returns the version number as a string
func (v *EstafetteCustomVersion) Version(params EstafetteVersionParams) string {
	return parseTemplate(v.LabelTemplate, params.GetFuncMap())
}

// EstafetteSemverVersion represents semantic versioning (http://semver.org/)
type EstafetteSemverVersion struct {
	Major         int    `yaml:"major,omitempty"`
	Minor         int    `yaml:"minor,omitempty"`
	Patch         string `yaml:"patch,omitempty"`
	LabelTemplate string `yaml:"labelTemplate,omitempty"`
	ReleaseBranch string `yaml:"releaseBranch,omitempty"`
}

// Version returns the version number as a string
func (v *EstafetteSemverVersion) Version(params EstafetteVersionParams) string {

	patch := parseTemplate(v.Patch, params.GetFuncMap())

	label := ""
	if params.Branch != v.ReleaseBranch {
		label = fmt.Sprintf("-%v", parseTemplate(v.LabelTemplate, params.GetFuncMap()))
	}

	return fmt.Sprintf("%v.%v.%v%v", v.Major, v.Minor, patch, label)
}

// EstafetteVersionParams contains parameters used to generate a version number
type EstafetteVersionParams struct {
	AutoIncrement int
	Branch        string
	Revision      string
}

// GetFuncMap returns EstafetteVersionParams as a function map for use in templating
func (p *EstafetteVersionParams) GetFuncMap() template.FuncMap {

	return template.FuncMap{
		"auto":     func() string { return fmt.Sprint(p.AutoIncrement) },
		"branch":   func() string { return p.Branch },
		"revision": func() string { return p.Revision },
	}
}

func parseTemplate(templateText string, funcMap template.FuncMap) string {
	tmpl, err := template.New("version").Funcs(funcMap).Parse(templateText)
	if err != nil {
		return err.Error()
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, nil)
	if err != nil {
		return err.Error()
	}

	return buf.String()
}
