package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/basecamp/cli/credstore"
	"github.com/zalando/go-keyring"
)

var ErrNotFound = errors.New("credentials not found")

type Credentials struct {
	Token string `json:"token"`
}

type Store struct {
	credentials *credstore.Store
}

func NewStore(configDirectory string) *Store {
	return &Store{credentials: credstore.NewStore(credstore.StoreOptions{
		ServiceName:   "usetix",
		DisableEnvVar: "USETIX_NO_KEYRING",
		FallbackDir:   configDirectory,
	})}
}

func (store *Store) Load(key string) (Credentials, error) {
	data, err := store.credentials.Load(key)
	if err != nil {
		if missingCredential(err) {
			return Credentials{}, ErrNotFound
		}
		return Credentials{}, err
	}

	var credentials Credentials
	if err := json.Unmarshal(data, &credentials); err != nil {
		return Credentials{}, fmt.Errorf("decode stored credentials: %w", err)
	}
	if strings.TrimSpace(credentials.Token) == "" {
		return Credentials{}, errors.New("stored credentials do not contain a token")
	}
	return credentials, nil
}

func (store *Store) Save(key string, credentials Credentials) error {
	if strings.TrimSpace(credentials.Token) == "" {
		return errors.New("token must not be empty")
	}
	data, err := json.Marshal(credentials)
	if err != nil {
		return err
	}
	return store.credentials.Save(key, data)
}

func (store *Store) Delete(key string) error {
	if _, err := store.Load(key); err != nil {
		return err
	}
	return store.credentials.Delete(key)
}

func (store *Store) UsingKeyring() bool {
	return store.credentials.UsingKeyring()
}

func (store *Store) FallbackWarning() string {
	return store.credentials.FallbackWarning()
}

func missingCredential(err error) bool {
	return errors.Is(err, keyring.ErrNotFound) || strings.Contains(err.Error(), "credentials not found")
}
