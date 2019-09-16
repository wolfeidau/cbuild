package buildspec

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/wolfeidau/cbuild/pkg/fileutil"
)

// LoadSpec load the spec file if it is exists
func LoadSpec() (*string, error) {
	if fileutil.Exists("buildspec.yml") {
		data, err := ioutil.ReadFile("buildspec.yml")
		if err != nil {
			return nil, errors.Wrap(err, "failed to load buildspec file")
		}

		str := string(data)

		log.Debug().Str("data", str).Msg("loaded spec")

		return &str, nil
	}

	return nil, nil
}
