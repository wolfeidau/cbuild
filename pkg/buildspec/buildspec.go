package buildspec

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/wolfeidau/cbuild/pkg/fileutil"
)

// LoadSpec load the spec file if it is exists
func LoadSpec(specPath string) (*string, error) {

	log.Info().Str("specPath", specPath).Msg("loading buildspec")

	if !fileutil.Exists(specPath) {
		log.Warn().Str("specPath", specPath).Msg("buildspec.yml not found.")
		return nil, nil
	}

	data, err := ioutil.ReadFile(specPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load buildspec file")
	}

	str := string(data)

	log.Debug().Str("data", str).Msg("loaded spec")

	return &str, nil

}
