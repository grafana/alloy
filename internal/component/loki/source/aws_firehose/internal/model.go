package internal

// FirehoseRequest implements AWS Firehose HTTP request format, according to the following appendix
// https://docs.aws.amazon.com/firehose/latest/dev/httpdeliveryrequestresponse.html#requestformat
type FirehoseRequest struct {
	RequestID string           `json:"requestId"`
	Timestamp int64            `json:"timestamp"`
	Records   []FirehoseRecord `json:"records"`
}

// FirehoseRecord is an envelope around a sole data record, received over Firehose HTTP API.
type FirehoseRecord struct {
	Data string `json:"data"`
}

// CloudwatchLogsRecord is an envelope around a series of logging events, according to
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/SubscriptionFilters.html#DestinationKinesisExample
type CloudwatchLogsRecord struct {
	// Owner is the AWS Account ID of the originating log data
	Owner string `json:"owner"`

	// LogGroup is the log group name of the originating log data
	LogGroup string `json:"logGroup"`

	// LogStream is the log stream of the originating log data
	LogStream string `json:"logStream"`

	// SubscriptionFilters is the list of subscription filter names
	// that matched with the originating log data
	SubscriptionFilters []string `json:"subscriptionFilters"`

	// MessageType describes the type of LogEvents this record carries.
	// Data messages will use the "DATA_MESSAGE" type. Sometimes CloudWatch
	// Logs may emit Kinesis Data Streams records with a "CONTROL_MESSAGE" type,
	// mainly for checking if the destination is reachable.
	MessageType string `json:"messageType"`

	// LogEvents contains the actual log data.
	LogEvents []CloudwatchLogEvent `json:"logEvents"`
}

// CloudwatchLogEvent is a single CloudWatch logging event.
type CloudwatchLogEvent struct {
	// ID is a unique id for each log event.
	ID string `json:"id"`

	Timestamp int64  `json:"timestamp"`
	Message   string `json:"message"`
}
