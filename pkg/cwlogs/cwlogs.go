package cwlogs

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// LogLine logs data
type LogLine struct {
	Timestamp time.Time `json:"timestamp,omitempty"`
	Message   string    `json:"message,omitempty"`
}

// ReadLogsParams read cloudwatch logs parameters
type ReadLogsParams struct {
	GroupName  string  `json:"group_name,omitempty" jsonschema:"required"`
	StreamName string  `json:"stream_name,omitempty" jsonschema:"required"`
	NextToken  *string `json:"next_token,omitempty"`
}

// ReadLogsResult read cloudwatch logs result
type ReadLogsResult struct {
	LogLines  []*LogLine `json:"log_lines,omitempty"`
	NextToken *string    `json:"next_token,omitempty"`
}

// LogsReader logs reader
type LogsReader interface {
	ReadLogs(*ReadLogsParams) (*ReadLogsResult, error)
}

// CloudwatchLogsReader cloudwatch log reader which uploads chunk of log data to buildkite
type CloudwatchLogsReader struct {
	cwlogsSvc cloudwatchlogsiface.CloudWatchLogsAPI
}

// NewCloudwatchLogsReader read all the things
func NewCloudwatchLogsReader(cfgs ...*aws.Config) *CloudwatchLogsReader {
	sess := session.Must(session.NewSession(cfgs...))
	return &CloudwatchLogsReader{
		cwlogsSvc: cloudwatchlogs.New(sess),
	}
}

// ReadLogs this reads a page of logs from cloudwatch and returns a token which will access the next page
func (cwlr *CloudwatchLogsReader) ReadLogs(rlr *ReadLogsParams) (*ReadLogsResult, error) {

	log.Debug().Str("GroupName", rlr.GroupName).Str("StreamName", rlr.StreamName).Msg("read logs")

	getlogsInput := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(rlr.GroupName),
		LogStreamName: aws.String(rlr.StreamName),
		NextToken:     rlr.NextToken,
	}

	getlogsResult, err := cwlr.cwlogsSvc.GetLogEvents(getlogsInput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			log.Debug().Err(err).Str("code", aerr.Code()).Msg("GetLogEvents failed")
			switch aerr.Code() {
			case cloudwatch.ErrCodeResourceNotFoundException:
				return &ReadLogsResult{LogLines: []*LogLine{}}, nil
			}
		}

		return nil, errors.Wrap(err, "failed to read logs from codebuild cloudwatch log group")
	}

	logLines := make([]*LogLine, len(getlogsResult.Events))

	for n, event := range getlogsResult.Events {
		logLines[n] = &LogLine{Message: aws.StringValue(event.Message), Timestamp: aws.MillisecondsTimeValue(event.Timestamp)}
	}

	nextTokenResult := getlogsResult.NextForwardToken

	log.Debug().Str("NextToken", aws.StringValue(nextTokenResult)).Msg("retrieved logs")

	return &ReadLogsResult{NextToken: nextTokenResult, LogLines: logLines}, nil
}
