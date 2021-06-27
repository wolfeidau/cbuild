package main

import (
	cryptorand "crypto/rand"
	"os"
	"path"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/dustin/go-humanize"
	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/wolfeidau/cbuild/pkg/archive"
	"github.com/wolfeidau/cbuild/pkg/buildspec"
	"github.com/wolfeidau/cbuild/pkg/launcher"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	verbose          = kingpin.Flag("verbose", "Verbose mode.").Short('v').Bool()
	codebuildProject = kingpin.Flag("project", "Codebuild Project to use for builds.").Envar("BUILD_PROJECT_NAME").Required().String()
	sourceBucket     = kingpin.Flag("source-bucket", "Source bucket used to stage sources.").Envar("SOURCE_BUCKET").Required().String()
	sourcePrefix     = kingpin.Flag("source-prefix", "Source bucket used to stage sources.").Envar("SOURCE_PREFIX").Default("source").String()
	specFile         = kingpin.Flag("spec", "Path to buildspec YAML to use when running build.").Envar("BUILD_SPEC_PATH").Default("./buildspec.yaml").String()

	version = "unknown"
)

func main() {

	kingpin.Version(version)
	kingpin.Parse()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	if *verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	log.Info().Msgf("using codebuild project %s", *codebuildProject)

	archSize, arch, err := archive.Build("")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to build archive")
	}

	sourceID := mustGenerate()

	sess := session.Must(session.NewSession())

	// upload to s3
	// Create an uploader with the session and default options
	uploader := s3manager.NewUploader(sess)

	log.Info().Str("sourceBucket", *sourceBucket).Str("size", humanize.IBytes(uint64(archSize))).Msg("upload archive")

	sourceArchivePath := path.Join(*sourcePrefix, *codebuildProject, sourceID.String()+".zip")

	// Upload the file to S3.
	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: sourceBucket,
		Key:    aws.String(sourceArchivePath),
		Body:   arch,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to upload source archive")
	}

	log.Info().Str("location", result.Location).Msg("uploaded to s3")

	log.Debug().Str("filename", arch.Name()).Msg("cleanup temp file")

	err = os.Remove(arch.Name())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to cleanup source archive")
	}

	// attempt to load spec file
	spec, err := buildspec.LoadSpec(*specFile)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load buildspec")
	}

	lc := launcher.New(sess)

	buildRes, err := lc.RunBuild(&launcher.RunBuildParams{
		ProjectName:   *codebuildProject,
		SourceArchive: sourceArchivePath,
		SourceBucket:  *sourceBucket,
		Buildspec:     spec,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to launch build")
	}

	quitCh := make(chan bool)

	go lc.ReadUntilClose(&launcher.GetLogsParams{
		CloudwatchGroupName:  buildRes.CloudwatchGroupName,
		CloudwatchStreamName: buildRes.CloudwatchStreamName,
	}, quitCh)

	waitRes, err := lc.WaitForTask(&launcher.WaitParams{BuildID: buildRes.BuildID})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to launch run")
	}

	log.Info().Str("BuildID", waitRes.BuildID).Msg("finished build")

	quitCh <- true
}

func mustGenerate() ulid.ULID {
	return ulid.MustNew(ulid.Timestamp(time.Now()), cryptorand.Reader)
}
