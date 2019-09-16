package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	lru "github.com/hashicorp/golang-lru"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	uuid "github.com/satori/go.uuid"
	"github.com/wolfeidau/cbuild/pkg/archive"
	"github.com/wolfeidau/cbuild/pkg/buildspec"
	"github.com/wolfeidau/cbuild/pkg/config"
	"github.com/wolfeidau/cbuild/pkg/launcher"
)

func main() {

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	// zerolog.SetGlobalLevel(zerolog.InfoLevel)

	log.Info().Msg("building archive")

	arch, err := archive.Build("")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to build archive")
	}

	cfg, err := config.NewDefaultConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	sourceID := uuid.NewV4()

	sess := session.Must(session.NewSession())

	// reset the archive stream
	arch.Seek(0, io.SeekStart)

	// upload to s3
	// Create an uploader with the session and default options
	uploader := s3manager.NewUploader(sess)

	// Upload the file to S3.
	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(cfg.SourceBucket),
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
	spec, err := buildspec.LoadSpec()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load buildspec")
	}

	lc := launcher.New(sess, cfg)

	buildRes, err := lc.RunBuild(&launcher.RunBuildParams{
		ProjectName: cfg.BuildProjectArn,
		SourceID:    sourceID.String(),
		Buildspec:   spec,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to launch build")
	}

	quit := make(chan bool)

	go func() {

		var nextToken *string

		ch, err := lru.New(256)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create cache")
		}

		for {
			select {
			case <-quit:
				return
			default:
				// Do other stuff

				logsRes, err := lc.GetTaskLogs(&launcher.GetLogsParams{
					CloudwatchGroupName:  buildRes.CloudwatchGroupName,
					CloudwatchStreamName: buildRes.CloudwatchStreamName,
					NextToken:            nextToken,
				})
				if err != nil {
					log.Fatal().Err(err).Msg("failed to get logs build")
				}

				if aws.StringValue(nextToken) == aws.StringValue(logsRes.NextToken) {
					time.Sleep(2 * time.Second)
					continue
				}

				for _, ll := range logsRes.LogLines {

					msg := fmt.Sprintf("ts=%s msg=%s", ll.Timestamp.Format(time.RFC3339), ll.Message)

					if ok, _ := ch.ContainsOrAdd(msg, "test"); ok {
						time.Sleep(2 * time.Second)
						continue
					}
					fmt.Print(msg)
				}

				if len(logsRes.LogLines) == 0 {
					time.Sleep(1 * time.Second)
				}

			}
		}
	}()

	waitRes, err := lc.WaitForTask(&launcher.WaitParams{BuildID: buildRes.BuildID})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to launch run")
	}

	log.Info().Str("BuildID", waitRes.BuildID).Msg("finshed build")

	quit <- true
}
