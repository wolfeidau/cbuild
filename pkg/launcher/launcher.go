package launcher

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/codebuild"
	lru "github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/wolfeidau/cbuild/pkg/cwlogs"
)

// RunBuildParams used to launch Codebuild container based tasks
type RunBuildParams struct {
	ProjectName  string
	SourceID     string
	SourceBucket string
	Buildspec    *string // optional buildspec
}

type RunBuildResult struct {
	BuildID              string
	CloudwatchGroupName  string
	CloudwatchStreamName string
}

type WaitParams struct {
	BuildID string
}

type WaitResult struct {
	BuildID string
}

type GetLogsParams struct {
	CloudwatchGroupName  string
	CloudwatchStreamName string
	NextToken            *string
}

// GetLogsResult get logs task result for Codebuild
type GetLogsResult struct {
	LogLines  []*cwlogs.LogLine
	NextToken *string
}

// Launcher launch and monitor codebuild jobs
type Launcher struct {
	cbsvc        *codebuild.CodeBuild
	cwlogsReader cwlogs.LogsReader
}

// New create a new launcher
func New(sess *session.Session) *Launcher {
	return &Launcher{cbsvc: codebuild.New(sess), cwlogsReader: cwlogs.NewCloudwatchLogsReader()}
}

// RunBuild run a codebuild job
func (lc *Launcher) RunBuild(rb *RunBuildParams) (*RunBuildResult, error) {

	res, err := lc.cbsvc.StartBuild(&codebuild.StartBuildInput{
		ProjectName:            aws.String(rb.ProjectName),
		SourceTypeOverride:     aws.String(codebuild.SourceTypeS3),
		BuildspecOverride:      rb.Buildspec,
		SourceLocationOverride: aws.String(fmt.Sprintf("%s/%s", rb.SourceBucket, rb.SourceID+".zip")),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to start build")
	}

	buildID := aws.StringValue(res.Build.Id)
	logGroupName := fmt.Sprintf("/aws/codebuild/%s", rb.ProjectName)
	logStreamName := strings.Split(buildID, ":")[1]

	log.Info().
		Str("Id", buildID).
		Str("group", logGroupName).
		Str("stream", logStreamName).
		Msg("created build")

	return &RunBuildResult{
		BuildID:              buildID,
		CloudwatchGroupName:  logGroupName,
		CloudwatchStreamName: logStreamName,
	}, nil
}

// ReadUntilClose read logs until the supplied channel returns a value
func (lc *Launcher) ReadUntilClose(gtlp *GetLogsParams, quit chan bool) error {
	var nextToken *string

	ch, err := lru.New(1024)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create cache")
	}

	log.Info().Msg("reading logs for build")

	for {
		select {
		case <-quit:
			return nil
		default:
			// Do other stuff
			log.Debug().Msg("GetTaskLogs")
			logsRes, err := lc.GetTaskLogs(&GetLogsParams{
				CloudwatchGroupName:  gtlp.CloudwatchGroupName,
				CloudwatchStreamName: gtlp.CloudwatchStreamName,
				NextToken:            nextToken,
			})
			if err != nil {
				return errors.Wrap(err, "failed to get logs build")
			}

			log.Debug().Str("nextToken", aws.StringValue(logsRes.NextToken)).Msg("GetTaskLogs")

			if aws.StringValue(nextToken) == aws.StringValue(logsRes.NextToken) {
				log.Debug().Msg("Tokens Match")
				time.Sleep(2 * time.Second)
				continue
			}

			log.Debug().Int("count", len(logsRes.LogLines)).Msg("loglines returned")

			for _, ll := range logsRes.LogLines {

				msg := fmt.Sprintf("ts=%s msg=%s", ll.Timestamp.Format(time.RFC3339), ll.Message)

				if ok, _ := ch.ContainsOrAdd(msg, "test"); ok {
					log.Debug().Msg("skip")
					continue
				}
				fmt.Print(msg)
			}

			log.Debug().Msg("waiting")
			time.Sleep(5 * time.Second)

		}
	}

}

// GetTaskLogs get task logs
func (lc *Launcher) GetTaskLogs(gtlp *GetLogsParams) (*GetLogsResult, error) {

	res, err := lc.cwlogsReader.ReadLogs(&cwlogs.ReadLogsParams{
		GroupName:  gtlp.CloudwatchGroupName,
		StreamName: gtlp.CloudwatchStreamName,
		NextToken:  gtlp.NextToken,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve logs for build.")
	}

	return &GetLogsResult{
		LogLines:  res.LogLines,
		NextToken: res.NextToken,
	}, nil
}

// WaitForTask wait for task to complete
func (lc *Launcher) WaitForTask(wft *WaitParams) (*WaitResult, error) {

	log.Info().
		Str("Id", wft.BuildID).
		Msg("waiting for build to complete")

	params := &codebuild.BatchGetBuildsInput{
		Ids: []*string{aws.String(wft.BuildID)},
	}

	err := lc.waitUntilTasksStoppedWithContext(context.Background(), params)
	if err != nil {
		return nil, errors.Wrap(err, "failed to wait for build.")
	}

	return &WaitResult{BuildID: wft.BuildID}, nil
}

func (lc *Launcher) waitUntilTasksStoppedWithContext(ctx aws.Context, input *codebuild.BatchGetBuildsInput, opts ...request.WaiterOption) error {
	w := request.Waiter{
		Name:        "WaitUntilBuildsStopped",
		MaxAttempts: 100,
		Delay:       request.ConstantWaiterDelay(6 * time.Second),
		Acceptors: []request.WaiterAcceptor{
			{
				State:   request.SuccessWaiterState,
				Matcher: request.PathAllWaiterMatch, Argument: "builds[].buildComplete",
				Expected: true,
			},
		},
		NewRequest: func(opts []request.Option) (*request.Request, error) {
			var inCpy *codebuild.BatchGetBuildsInput
			if input != nil {
				tmp := *input
				inCpy = &tmp
			}
			req, _ := lc.cbsvc.BatchGetBuildsRequest(inCpy)
			req.SetContext(ctx)
			req.ApplyOptions(opts...)
			return req, nil
		},
	}
	w.ApplyOptions(opts...)

	return w.WaitWithContext(ctx)
}
