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

// UnmarshalYAML customizes unmarshalling an EstafetteVersion
func (version *EstafetteVersion) UnmarshalYAML(unmarshal func(interface{}) error) (err error) {

	var aux struct {
		SemVer *EstafetteSemverVersion `yaml:"semver"`
		Custom *EstafetteCustomVersion `yaml:"custom"`
	}

	// unmarshal to auxiliary type
	if err := unmarshal(&aux); err != nil {
		return err
	}

	// map auxiliary properties
	version.SemVer = aux.SemVer
	version.Custom = aux.Custom

	// set default property values
	version.setDefaults()

	return nil
}

// setDefaults sets default values for properties of EstafetteVersion if not defined
func (version *EstafetteVersion) setDefaults() {
	if version.Custom == nil && version.SemVer == nil {
		version.SemVer = &EstafetteSemverVersion{}
	}

	// if version is semver set defaults
	if version.SemVer != nil {
		if version.SemVer.Patch == "" {
			version.SemVer.Patch = "{{auto}}"
		}
		if version.SemVer.LabelTemplate == "" {
			version.SemVer.LabelTemplate = "{{branch}}"
		}
		if len(version.SemVer.ReleaseBranch.Values) == 0 {
			version.SemVer.ReleaseBranch.Values = []string{"master"}
		}
	}

	// if version is custom set defaults
	if version.Custom != nil {
		if version.Custom.LabelTemplate == "" {
			version.Custom.LabelTemplate = "{{revision}}"
		}
	}
}

// Version returns the version number as a string
func (version *EstafetteVersion) Version(params EstafetteVersionParams) string {
	if version.Custom != nil {
		return version.Custom.Version(params)
	}
	if version.SemVer != nil {
		return version.SemVer.Version(params)
	}
	return ""
}

// EstafetteCustomVersion represents a custom version using a template
type EstafetteCustomVersion struct {
	LabelTemplate string `yaml:"labelTemplate"`
}

// Version returns the version number as a string
func (v *EstafetteCustomVersion) Version(params EstafetteVersionParams) string {
	return parseTemplate(v.LabelTemplate, params.GetFuncMap())
}

// EstafetteSemverVersion represents semantic versioning (http://semver.org/)
type EstafetteSemverVersion struct {
	Major         int                 `yaml:"major"`
	Minor         int                 `yaml:"minor"`
	Patch         string              `yaml:"patch"`
	LabelTemplate string              `yaml:"labelTemplate"`
	ReleaseBranch StringOrStringArray `yaml:"releaseBranch"`
}

// Version returns the version number as a string
func (v *EstafetteSemverVersion) Version(params EstafetteVersionParams) string {

	patchWithLabel := v.GetPatchWithLabel(params)

	return fmt.Sprintf("%v.%v.%v", v.Major, v.Minor, patchWithLabel)
}

// GetPatchWithLabel returns the formatted patch and label
func (v *EstafetteSemverVersion) GetPatchWithLabel(params EstafetteVersionParams) string {

	patch := v.GetPatch(params)
	label := v.GetLabel(params)

	return fmt.Sprintf("%v%v", patch, label)
}

// GetPatch returns the formatted patch
func (v *EstafetteSemverVersion) GetPatch(params EstafetteVersionParams) string {

	return parseTemplate(v.Patch, params.GetFuncMap())
}

// GetLabel returns the formatted label
func (v *EstafetteSemverVersion) GetLabel(params EstafetteVersionParams) string {

	label := ""
	if !v.ReleaseBranch.Contains(params.Branch) {
		label = fmt.Sprintf("-%v", parseTemplate(v.LabelTemplate, params.GetFuncMap()))
	}

	return label
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
