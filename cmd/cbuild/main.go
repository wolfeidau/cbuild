package main

import (
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/dustin/go-humanize"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	uuid "github.com/satori/go.uuid"
	"github.com/wolfeidau/cbuild/pkg/archive"
	"github.com/wolfeidau/cbuild/pkg/buildspec"
	"github.com/wolfeidau/cbuild/pkg/launcher"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	verbose          = kingpin.Flag("verbose", "Verbose mode.").Short('v').Bool()
	codebuildProject = kingpin.Flag("project", "Codebuild Project to use for builds.").Envar("BUILD_PROJECT_NAME").Required().String()
	sourceBucket     = kingpin.Flag("source-bucket", "Source bucket used to stage sources.").Envar("SOURCE_BUCKET").Required().String()
	specFile         = kingpin.Flag("spec", "Path to buildspec.yml to use when running build.").Envar("BUILD_SPEC_PATH").Default("./buildspec.yml").String()

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

	sourceID := uuid.NewV4()

	sess := session.Must(session.NewSession())

	// upload to s3
	// Create an uploader with the session and default options
	uploader := s3manager.NewUploader(sess)

	log.Info().Str("sourceBucket", *sourceBucket).Str("size", humanize.IBytes(uint64(archSize))).Msg("upload archive")

	// Upload the file to S3.
	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: sourceBucket,
		Key:    aws.String(sourceID.String() + ".zip"),
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
		ProjectName:  *codebuildProject,
		SourceID:     sourceID.String(),
		SourceBucket: *sourceBucket,
		Buildspec:    spec,
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
