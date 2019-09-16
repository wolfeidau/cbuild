package archive

import (
	"archive/zip"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/helm/helm/pkg/ignore"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/wolfeidau/cbuild/pkg/fileutil"
)

// DefaultIgnoreFileName default file name for the ignore file used
const DefaultIgnoreFileName = ".cbuildignore"

// Build build the archive file while ignoring if required.
func Build(ignoreFile string) (*os.File, error) {

	if ignoreFile == "" {
		ignoreFile = DefaultIgnoreFileName
	}

	var (
		err error
	)

	rules := &ignore.Rules{}

	if fileutil.Exists(ignoreFile) {
		rules, err = ignore.ParseFile(ignoreFile)
		if err != nil {
			return nil, errors.Wrap(err, "failed to ignore config")
		}
	}

	tmpfile, err := ioutil.TempFile("", "example.*.zip")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create temp file")
	}

	log.Info().Str("name", tmpfile.Name()).Msg("created temp file")

	w := zip.NewWriter(tmpfile)

	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Info().Msgf("prevent panic by handling failure accessing a path %q: %v", path, err)
			return err
		}

		if rules.Ignore(path, info) {
			// log.Info().Msgf("ignored file or dir: %q", path)
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !info.IsDir() {

			log.Debug().Str("path", path).Msgf("added file")

			f, err := w.Create(path)
			if err != nil {
				return err
			}

			data, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			_, err = f.Write(data)
			if err != nil {
				return err
			}

		}

		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to walking path")
	}

	err = w.Close()
	if err != nil {
		return nil, errors.Wrap(err, "failed to close buffer")
	}

	return tmpfile, nil
}
