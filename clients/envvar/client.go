package envvar

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/estafette/estafette-ci-builder/clients/obfuscation"
	contracts "github.com/estafette/estafette-ci-contracts"
	crypt "github.com/estafette/estafette-ci-crypt"
	manifest "github.com/estafette/estafette-ci-manifest"
	foundation "github.com/estafette/estafette-foundation"
)

// Client is the interface for getting, setting and retrieving ESTAFETTE_ environment variables
//go:generate mockgen -package=envvar -destination ./mock.go -source=client.go
type Client interface {
	GetCommandOutput(string, ...string) (string, error)
	SetEstafetteGlobalEnvvars() error
	SetEstafetteBuilderConfigEnvvars(builderConfig contracts.BuilderConfig) error
	SetEstafetteEventEnvvars(events []*manifest.EstafetteEvent) error
	InitGitSource() error
	InitGitOwner() error
	InitGitName() error
	InitGitFullName() error
	InitGitRevision() error
	InitGitBranch() error
	InitBuildDatetime() error
	InitBuildStatus() error
	InitLabels(manifest.EstafetteManifest) error
	CollectEstafetteEnvvars() map[string]string
	CollectEstafetteEnvvarsAndLabels(manifest.EstafetteManifest) map[string]string
	CollectGlobalEnvvars(manifest.EstafetteManifest) map[string]string
	UnsetEstafetteEnvvars()
	GetEstafetteEnv(string) string
	SetEstafetteEnv(string, string) error
	UnsetEstafetteEnv(string) error
	GetEstafetteEnvvarName(string) string
	OverrideEnvvars(...map[string]string) map[string]string
	DecryptSecret(string, string) string
	DecryptSecrets(map[string]string, string) map[string]string
	GetCiServer() string
	SetPipelineName(builderConfig contracts.BuilderConfig) error
	GetPipelineName() string
	GetWorkDir() string
	GetTempDir() string
	MakeDNSLabelSafe(string) string

	GetGitOrigin() (string, error)
	GetSourceFromOrigin(string) string
	GetOwnerFromOrigin(string) string
	GetNameFromOrigin(string) string
}

// NewClient returns a new envvar
func NewClient(prefix string, secretHelper crypt.SecretHelper, obfuscationService obfuscation.Service) (Client, error) {
	return &client{
		prefix:             prefix,
		ciServer:           os.Getenv("ESTAFETTE_CI_SERVER"),
		workDir:            os.Getenv("ESTAFETTE_WORKDIR"),
		tempDir:            os.Getenv("ESTAFETTE_TEMPDIR"),
		secretHelper:       secretHelper,
		obfuscationService: obfuscationService,
	}, nil
}

type client struct {
	prefix             string
	ciServer           string
	workDir            string
	tempDir            string
	secretHelper       crypt.SecretHelper
	obfuscationService obfuscation.Service
}

func (c *client) GetCommandOutput(name string, arg ...string) (string, error) {

	out, err := exec.Command(name, arg...).Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

func (c *client) SetEstafetteGlobalEnvvars() (err error) {

	// initialize git source envvar
	err = c.InitGitSource()
	if err != nil {
		return err
	}

	// initialize git owner envvar
	err = c.InitGitOwner()
	if err != nil {
		return err
	}

	// initialize git name envvar
	err = c.InitGitName()
	if err != nil {
		return err
	}

	// initialize git full name envvar
	err = c.InitGitFullName()
	if err != nil {
		return err
	}

	// initialize git revision envvar
	err = c.InitGitRevision()
	if err != nil {
		return err
	}

	// initialize git branch envvar
	err = c.InitGitBranch()
	if err != nil {
		return err
	}

	// initialize build datetime envvar
	err = c.InitBuildDatetime()
	if err != nil {
		return err
	}

	// initialize build status envvar
	err = c.InitBuildStatus()
	if err != nil {
		return err
	}

	return
}

func (c *client) SetEstafetteBuilderConfigEnvvars(builderConfig contracts.BuilderConfig) (err error) {
	// set envvars that can be used by any container
	c.SetEstafetteEnv("ESTAFETTE_GIT_SOURCE", builderConfig.Git.RepoSource)
	c.SetEstafetteEnv("ESTAFETTE_GIT_OWNER", builderConfig.Git.RepoOwner)
	c.SetEstafetteEnv("ESTAFETTE_GIT_NAME", builderConfig.Git.RepoName)
	c.SetEstafetteEnv("ESTAFETTE_GIT_FULLNAME", fmt.Sprintf("%v/%v", builderConfig.Git.RepoOwner, builderConfig.Git.RepoName))

	c.SetEstafetteEnv("ESTAFETTE_GIT_BRANCH", builderConfig.Git.RepoBranch)
	c.SetEstafetteEnv("ESTAFETTE_GIT_REVISION", builderConfig.Git.RepoRevision)
	c.SetEstafetteEnv("ESTAFETTE_BUILD_VERSION", builderConfig.BuildVersion.Version)
	if builderConfig.BuildVersion != nil && builderConfig.BuildVersion.Major != nil {
		c.SetEstafetteEnv("ESTAFETTE_BUILD_VERSION_MAJOR", strconv.Itoa(*builderConfig.BuildVersion.Major))
	}
	if builderConfig.BuildVersion != nil && builderConfig.BuildVersion.Minor != nil {
		c.SetEstafetteEnv("ESTAFETTE_BUILD_VERSION_MINOR", strconv.Itoa(*builderConfig.BuildVersion.Minor))
	}
	if builderConfig.BuildVersion != nil && builderConfig.BuildVersion.AutoIncrement != nil {
		c.SetEstafetteEnv("ESTAFETTE_BUILD_VERSION_PATCH", strconv.Itoa(*builderConfig.BuildVersion.AutoIncrement))
	}
	if builderConfig.BuildVersion != nil && builderConfig.BuildVersion.Label != nil {
		c.SetEstafetteEnv("ESTAFETTE_BUILD_VERSION_LABEL", *builderConfig.BuildVersion.Label)
	}

	// set counters to enable release locking for older revisions inside extensions
	if builderConfig.BuildVersion != nil {
		c.SetEstafetteEnv("ESTAFETTE_BUILD_CURRENT_COUNTER", strconv.Itoa(builderConfig.BuildVersion.CurrentCounter))
		c.SetEstafetteEnv("ESTAFETTE_BUILD_MAX_COUNTER", strconv.Itoa(builderConfig.BuildVersion.MaxCounter))
		c.SetEstafetteEnv("ESTAFETTE_BUILD_MAX_COUNTER_CURRENT_BRANCH", strconv.Itoa(builderConfig.BuildVersion.MaxCounterCurrentBranch))
	}

	if builderConfig.ReleaseParams != nil {
		c.SetEstafetteEnv("ESTAFETTE_RELEASE_NAME", builderConfig.ReleaseParams.ReleaseName)
		c.SetEstafetteEnv("ESTAFETTE_RELEASE_ACTION", builderConfig.ReleaseParams.ReleaseAction)
		c.SetEstafetteEnv("ESTAFETTE_RELEASE_TRIGGERED_BY", builderConfig.ReleaseParams.TriggeredBy)
		// set ESTAFETTE_RELEASE_ID for backwards compatibility with extensions/slack-build-status
		c.SetEstafetteEnv("ESTAFETTE_RELEASE_ID", strconv.Itoa(builderConfig.ReleaseParams.ReleaseID))
	}
	if builderConfig.BuildParams != nil {
		// set ESTAFETTE_BUILD_ID for backwards compatibility with extensions/github-status and extensions/bitbucket-status and extensions/slack-build-status
		c.SetEstafetteEnv("ESTAFETTE_BUILD_ID", strconv.Itoa(builderConfig.BuildParams.BuildID))
	}

	// set ESTAFETTE_CI_SERVER_BASE_URL for backwards compatibility with extensions/github-status and extensions/bitbucket-status and extensions/slack-build-status
	if builderConfig.CIServer != nil {
		c.SetEstafetteEnv("ESTAFETTE_CI_SERVER_BASE_URL", builderConfig.CIServer.BaseURL)
	}

	return c.SetEstafetteEventEnvvars(builderConfig.Events)
}

func (c *client) SetEstafetteEventEnvvars(events []*manifest.EstafetteEvent) (err error) {

	if events != nil {
		for _, e := range events {
			if e == nil {
				continue
			}

			triggerFields := reflect.TypeOf(*e)
			triggerValues := reflect.ValueOf(*e)

			for i := 0; i < triggerFields.NumField(); i++ {

				triggerField := triggerFields.Field(i).Name
				triggerValue := triggerValues.Field(i)

				if triggerValue.Kind() != reflect.Ptr || triggerValue.IsNil() {
					continue
				}

				// dereference the pointer
				derefencedPointerValue := reflect.Indirect(triggerValue)

				triggerPropertyFields := derefencedPointerValue.Type()
				triggerPropertyValues := derefencedPointerValue

				for j := 0; j < triggerPropertyFields.NumField(); j++ {

					triggerPropertyField := triggerPropertyFields.Field(j).Name
					triggerPropertyValue := triggerPropertyValues.Field(j)

					envvarName := "ESTAFETTE_TRIGGER_" + foundation.ToUpperSnakeCase(triggerField) + "_" + foundation.ToUpperSnakeCase(triggerPropertyField)
					envvarValue := ""

					switch triggerPropertyValue.Kind() {
					case reflect.String:
						envvarValue = triggerPropertyValue.String()
					default:
						jsonValue, _ := json.Marshal(triggerPropertyValue.Interface())
						envvarValue = string(jsonValue)

						envvarValue = strings.Trim(envvarValue, "\"")
					}

					if e.Fired {
						c.SetEstafetteEnv(envvarName, envvarValue)
					}

					if e.Name != "" {
						// set envvar for named trigger/event, in order to have upstream pipelines and release when they're not fired as well
						envvarName := "ESTAFETTE_TRIGGER_" + foundation.ToUpperSnakeCase(e.Name) + "_" + foundation.ToUpperSnakeCase(triggerPropertyField)
						c.SetEstafetteEnv(envvarName, envvarValue)
					}
				}
			}
		}
	}

	return nil
}

func (c *client) GetGitOrigin() (string, error) {
	return c.GetCommandOutput("git", "config", "--get", "remote.origin.url")
}

func (c *client) InitGitSource() (err error) {
	if c.GetEstafetteEnv("ESTAFETTE_GIT_SOURCE") == "" {
		origin, err := c.GetGitOrigin()
		if err != nil {
			return err
		}
		source := c.GetSourceFromOrigin(origin)
		return c.SetEstafetteEnv("ESTAFETTE_GIT_SOURCE", source)
	}
	return
}

func (c *client) InitGitOwner() (err error) {
	if c.GetEstafetteEnv("ESTAFETTE_GIT_OWNER") == "" {
		origin, err := c.GetGitOrigin()
		if err != nil {
			return err
		}
		owner := c.GetOwnerFromOrigin(origin)
		return c.SetEstafetteEnv("ESTAFETTE_GIT_OWNER", owner)
	}
	return
}

func (c *client) InitGitName() (err error) {
	if c.GetEstafetteEnv("ESTAFETTE_GIT_NAME") == "" {
		origin, err := c.GetGitOrigin()
		if err != nil {
			return err
		}
		name := c.GetNameFromOrigin(origin)
		return c.SetEstafetteEnv("ESTAFETTE_GIT_NAME", name)
	}
	return
}

func (c *client) InitGitFullName() (err error) {
	if c.GetEstafetteEnv("ESTAFETTE_GIT_FULLNAME") == "" {
		origin, err := c.GetGitOrigin()
		if err != nil {
			return err
		}
		owner := c.GetOwnerFromOrigin(origin)
		name := c.GetNameFromOrigin(origin)
		return c.SetEstafetteEnv("ESTAFETTE_GIT_FULLNAME", fmt.Sprintf("%v/%v", owner, name))
	}
	return
}

func (c *client) SetPipelineName(builderConfig contracts.BuilderConfig) (err error) {

	if builderConfig.Git == nil {
		err = c.InitGitSource()
		if err != nil {
			return
		}

		err = c.InitGitOwner()
		if err != nil {
			return
		}

		err = c.InitGitName()
		if err != nil {
			return
		}

		return nil
	}

	err = c.SetEstafetteEnv("ESTAFETTE_GIT_SOURCE", builderConfig.Git.RepoSource)
	if err != nil {
		return
	}

	err = c.SetEstafetteEnv("ESTAFETTE_GIT_OWNER", builderConfig.Git.RepoOwner)
	if err != nil {
		return
	}

	err = c.SetEstafetteEnv("ESTAFETTE_GIT_NAME", builderConfig.Git.RepoName)
	if err != nil {
		return
	}

	return nil
}

func (c *client) GetPipelineName() string {

	source := c.GetEstafetteEnv("ESTAFETTE_GIT_SOURCE")
	owner := c.GetEstafetteEnv("ESTAFETTE_GIT_OWNER")
	name := c.GetEstafetteEnv("ESTAFETTE_GIT_NAME")

	if source == "" || owner == "" || name == "" {
		log.Fatal().Msg("Git environment variables have not been set yet, cannot resolve pipeline name")
	}

	return fmt.Sprintf("%v/%v/%v", source, owner, name)
}

func (c *client) GetSourceFromOrigin(origin string) string {

	re := regexp.MustCompile(`^(git@|https://)([^:\/]+)(:|/)([^\/]+)/([^\/]+)\.git`)
	match := re.FindStringSubmatch(origin)

	if len(match) < 6 {
		return ""
	}

	return match[2]
}

func (c *client) GetOwnerFromOrigin(origin string) string {

	re := regexp.MustCompile(`^(git@|https://)([^:\/]+)(:|/)([^\/]+)/([^\/]+)\.git`)
	match := re.FindStringSubmatch(origin)

	if len(match) < 6 {
		return ""
	}

	return match[4]
}

func (c *client) GetNameFromOrigin(origin string) string {

	re := regexp.MustCompile(`^(git@|https://)([^:\/]+)(:|/)([^\/]+)/([^\/]+)\.git`)
	match := re.FindStringSubmatch(origin)

	if len(match) < 6 {
		return ""
	}

	return match[5]
}

func (c *client) InitGitRevision() (err error) {
	if c.GetEstafetteEnv("ESTAFETTE_GIT_REVISION") == "" {
		revision, err := c.GetCommandOutput("git", "rev-parse", "HEAD")
		if err != nil {
			return err
		}
		return c.SetEstafetteEnv("ESTAFETTE_GIT_REVISION", revision)
	}
	return
}

func (c *client) InitGitBranch() (err error) {
	if c.GetEstafetteEnv("ESTAFETTE_GIT_BRANCH") == "" {
		branch, err := c.GetCommandOutput("git", "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return err
		}
		return c.SetEstafetteEnv("ESTAFETTE_GIT_BRANCH", branch)
	}
	return
}

func (c *client) InitBuildDatetime() (err error) {
	if c.GetEstafetteEnv("ESTAFETTE_BUILD_DATETIME") == "" {
		return c.SetEstafetteEnv("ESTAFETTE_BUILD_DATETIME", time.Now().UTC().Format(time.RFC3339))
	}
	return
}

func (c *client) InitBuildStatus() (err error) {
	return c.SetEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
}

func (c *client) InitLabels(m manifest.EstafetteManifest) (err error) {

	// set labels as envvars
	if m.Labels != nil && len(m.Labels) > 0 {
		for key, value := range m.Labels {
			envvarName := "ESTAFETTE_LABEL_" + foundation.ToUpperSnakeCase(key)
			err = c.SetEstafetteEnv(envvarName, value)
			if err != nil {
				return
			}
		}
	}

	return
}

func (c *client) CollectEstafetteEnvvarsAndLabels(m manifest.EstafetteManifest) (envvars map[string]string) {

	// set labels as envvars
	c.InitLabels(m)

	// return all envvars starting with ESTAFETTE_
	return c.CollectEstafetteEnvvars()
}

func (c *client) CollectEstafetteEnvvars() (envvars map[string]string) {

	// return all envvars starting with ESTAFETTE_
	envvars = map[string]string{}

	for _, e := range os.Environ() {
		kvPair := strings.SplitN(e, "=", 2)

		if len(kvPair) == 2 {
			envvarName := kvPair[0]
			envvarValue := kvPair[1]

			if strings.HasPrefix(envvarName, c.prefix) {
				envvars[envvarName] = envvarValue
			}
		}
	}

	return
}

func (c *client) CollectGlobalEnvvars(m manifest.EstafetteManifest) (envvars map[string]string) {

	envvars = map[string]string{}

	if m.GlobalEnvVars != nil {
		envvars = m.GlobalEnvVars
	}

	return
}

// only to be used from unit tests
func (c *client) UnsetEstafetteEnvvars() {

	envvarsToUnset := c.CollectEstafetteEnvvars()
	for key := range envvarsToUnset {
		c.UnsetEstafetteEnv(key)
	}
}

func (c *client) GetEstafetteEnv(key string) string {

	key = c.GetEstafetteEnvvarName(key)

	if strings.HasPrefix(key, c.prefix) {
		return os.Getenv(key)
	}

	return fmt.Sprintf("${%v}", key)
}

func (c *client) SetEstafetteEnv(key, value string) error {

	key = c.GetEstafetteEnvvarName(key)

	err := os.Setenv(key, value)
	if err != nil {
		return err
	}

	// set dns safe version
	err = os.Setenv(fmt.Sprintf("%v_DNS_SAFE", key), c.MakeDNSLabelSafe(value))
	if err != nil {
		return err
	}

	return nil
}

func (c *client) UnsetEstafetteEnv(key string) error {

	key = c.GetEstafetteEnvvarName(key)

	return os.Unsetenv(key)
}

func (c *client) GetEstafetteEnvvarName(key string) string {
	return strings.Replace(key, "ESTAFETTE_", c.prefix, -1)
}

func (c *client) OverrideEnvvars(envvarMaps ...map[string]string) (envvars map[string]string) {

	envvars = make(map[string]string)
	for _, envvarMap := range envvarMaps {
		if envvarMap != nil && len(envvarMap) > 0 {
			for k, v := range envvarMap {
				envvars[k] = v
			}
		}
	}

	return
}

func (c *client) DecryptSecret(encryptedValue, pipeline string) (decryptedValue string) {

	decryptedValue, err := c.secretHelper.DecryptAllEnvelopes(encryptedValue, pipeline)

	if err != nil {
		log.Warn().Err(err).Msg("Failed decrypting secret")
		return encryptedValue
	}

	return
}

func (c *client) DecryptSecrets(encryptedEnvvars map[string]string, pipeline string) (envvars map[string]string) {

	if encryptedEnvvars == nil || len(encryptedEnvvars) == 0 {
		return encryptedEnvvars
	}

	envvars = make(map[string]string)
	for k, v := range encryptedEnvvars {
		envvars[k] = c.DecryptSecret(v, pipeline)
	}

	return
}

func (c *client) GetCiServer() string {
	return c.ciServer
}

func (c *client) GetWorkDir() string {
	return c.workDir
}

func (c *client) GetTempDir() string {
	return c.tempDir
}

func (c *client) MakeDNSLabelSafe(value string) string {
	// in order for the label to be used as a dns label (part between dots) it should only use
	// lowercase letters, digits and hyphens and have a max length of 63 characters;
	// also it should start with a letter and not end in a hyphen

	// ensure the label is lowercase
	value = strings.ToLower(value)

	// replace all invalid characters with a hyphen
	reg := regexp.MustCompile(`[^a-z0-9-]+`)
	value = reg.ReplaceAllString(value, "-")

	// replace double hyphens with a single one
	value = strings.Replace(value, "--", "-", -1)

	// trim hyphens from start and end
	value = strings.Trim(value, "-")

	// ensure it starts with a letter, not a digit or hyphen
	reg = regexp.MustCompile(`^[0-9-]+`)
	value = reg.ReplaceAllString(value, "")

	if len(value) > 63 {
		value = value[:63]
	}

	// trim hyphens from start and end
	value = strings.Trim(value, "-")

	return value
}
