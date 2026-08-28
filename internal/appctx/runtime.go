package appctx

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/basecamp/cli/profile"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/auth"
	"github.com/gobijan/usetix-cli/internal/config"
	"github.com/gobijan/usetix-cli/internal/output"
)

type GlobalFlags struct {
	Agent   bool
	APIURL  string
	Count   bool
	IDsOnly bool
	JSON    bool
	Profile string
	Quiet   bool
	Styled  bool
}

type Runtime struct {
	Version    string
	Flags      *GlobalFlags
	Getenv     config.GetenvFunc
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	HTTPClient *http.Client

	configDirectory string
	credentials     *auth.Store
	profiles        *profile.Store
}

type Target struct {
	ProfileName   string `json:"profile,omitempty"`
	APIURL        string `json:"api_url"`
	CredentialKey string `json:"-"`
}

func NewRuntime(version string, flags *GlobalFlags, getenv config.GetenvFunc, stdin io.Reader, stdout, stderr io.Writer, httpClient *http.Client, configDirectory string) (*Runtime, error) {
	if configDirectory == "" {
		var err error
		configDirectory, err = config.Dir(getenv)
		if err != nil {
			return nil, fmt.Errorf("resolve config directory: %w", err)
		}
	}
	return &Runtime{
		Version:         version,
		Flags:           flags,
		Getenv:          getenv,
		Stdin:           stdin,
		Stdout:          stdout,
		Stderr:          stderr,
		HTTPClient:      httpClient,
		configDirectory: configDirectory,
	}, nil
}

func (runtime *Runtime) ConfigDirectory() string {
	return runtime.configDirectory
}

func (runtime *Runtime) ProfileStore() *profile.Store {
	if runtime.profiles == nil {
		runtime.profiles = profile.NewStore(config.File(runtime.configDirectory))
	}
	return runtime.profiles
}

func (runtime *Runtime) CredentialStore() *auth.Store {
	if runtime.credentials == nil {
		runtime.credentials = auth.NewStore(runtime.configDirectory)
	}
	return runtime.credentials
}

func (runtime *Runtime) ResolveTarget() (Target, error) {
	profiles, defaultProfile, err := runtime.ProfileStore().List()
	if err != nil {
		return Target{}, fmt.Errorf("load profiles: %w", err)
	}

	requestedProfile := strings.TrimSpace(runtime.Flags.Profile)
	environmentProfile := strings.TrimSpace(runtime.Getenv(config.ProfileEnv))
	if len(profiles) == 0 && (requestedProfile != "" || environmentProfile != "") {
		name := requestedProfile
		if name == "" {
			name = environmentProfile
		}
		return Target{}, output.ErrUsageHint("profile not found: "+name, "Create it with: usetix profile create "+name)
	}

	profileName, err := profile.Resolve(profile.ResolveOptions{
		FlagValue:      requestedProfile,
		EnvVar:         environmentProfile,
		DefaultProfile: defaultProfile,
		Profiles:       profiles,
	})
	if err != nil {
		return Target{}, output.ErrUsageHint(err.Error(), "Select one with --profile or run: usetix profile use NAME")
	}

	profileAPIURL := ""
	if profileName != "" {
		profileAPIURL = profiles[profileName].BaseURL
	}
	apiURL := config.ResolveAPIURL(runtime.Flags.APIURL, runtime.Getenv(config.APIURLEnv), profileAPIURL)
	if err := config.ValidateAPIURL(apiURL); err != nil {
		return Target{}, output.ErrUsage("invalid API URL: " + err.Error())
	}

	return Target{
		ProfileName:   profileName,
		APIURL:        apiURL,
		CredentialKey: profile.CredentialKey(profileName, apiURL),
	}, nil
}

func (runtime *Runtime) Token(target Target) (string, string, error) {
	if token := strings.TrimSpace(runtime.Getenv(config.TokenEnv)); token != "" {
		return token, "environment", nil
	}

	credentials, err := runtime.CredentialStore().Load(target.CredentialKey)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			return "", "", &output.Error{
				Code:    "auth_required",
				Message: "Not authenticated",
				Hint:    "Run: usetix auth login",
			}
		}
		return "", "", fmt.Errorf("load credentials: %w", err)
	}
	return credentials.Token, credentialStoreName(runtime.CredentialStore()), nil
}

func (runtime *Runtime) APIClient() (*api.Client, Target, error) {
	target, err := runtime.ResolveTarget()
	if err != nil {
		return nil, Target{}, err
	}
	token, _, err := runtime.Token(target)
	if err != nil {
		return nil, Target{}, err
	}
	client, err := api.New(target.APIURL, token, runtime.Version, api.WithHTTPClient(runtime.HTTPClient))
	if err != nil {
		return nil, Target{}, err
	}
	return client, target, nil
}

func (runtime *Runtime) OutputFormat() output.Format {
	switch {
	case runtime.Flags.Quiet:
		return output.FormatQuiet
	case runtime.Flags.IDsOnly:
		return output.FormatIDs
	case runtime.Flags.Count:
		return output.FormatCount
	case runtime.Flags.Styled:
		return output.FormatStyled
	case runtime.Flags.JSON || runtime.Flags.Agent:
		return output.FormatJSON
	default:
		return output.FormatAuto
	}
}

func (runtime *Runtime) Output() *output.Writer {
	return output.New(runtime.OutputFormat(), runtime.Stdout)
}

func credentialStoreName(store *auth.Store) string {
	if store.UsingKeyring() {
		return "system keyring"
	}
	return "credentials file"
}
