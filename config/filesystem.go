package config

import (
	"fmt"
	"github.com/spf13/afero"
	"net/url"
)

func (c *Config) PolicyStorage() (afero.Fs, error) {
	return c.Filesystem(c.Policies.Url)
}

func (c *Config) Filesystem(storageUrl string) (afero.Fs, error) {
	location, err := url.Parse(storageUrl)
	if err != nil {
		return nil, err
	}

	switch location.Scheme {
	case "os":
		return afero.NewBasePathFs(afero.NewOsFs(), location.Path), nil
	default:
		return nil, fmt.Errorf("unsupported storage scheme: %s", location.Scheme)
	}
}
