package infinity

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/utils"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/grafana/grafana-infinity-datasource/pkg/httpclient"
	ds "github.com/grafana/grafana-infinity-datasource/pkg/infinity"
	"github.com/grafana/grafana-infinity-datasource/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stormcat24/protodep/pkg/logger"
	"golang.org/x/oauth2"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	backendLog "github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

func init() {
	component.Register(component.Registration{
		Name:      "infinity",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type BasicAuthOptions struct {
	UserName string `alloy:"username,attr,optional"`
	Password string `alloy:"password,attr,optional"`
}

type APIKeyType string

const (
	APIKeyTypeHeader APIKeyType = "header"
	APIKeyTypeQuery  APIKeyType = "query"
)

// MarshalText implements encoding.TextMarshaler
func (a APIKeyType) MarshalText() (text []byte, err error) {
	return []byte(a), nil
}

// UnmarshalText implements encoding.TextUnmarshaler
func (a *APIKeyType) UnmarshalText(text []byte) error {
	str := string(text)
	switch str {
	case "header":
		*a = APIKeyTypeHeader
	case "query":
		*a = APIKeyTypeQuery
	default:
		return fmt.Errorf("unknown api key type: %s", str)
	}

	return nil
}

type APIKeyAuthOptions struct {
	Type  APIKeyType `alloy:"type,attr,optional"`
	Key   string     `alloy:"key,attr,optional"`
	Value string     `alloy:"value,attr,optional"`
}

type BearerTokenAuthOptions struct {
	Token string `alloy:"token,attr,optional"`
}

type AzureBlobCloudType string

const (
	AzureCloud             AzureBlobCloudType = "AzureCloud"
	AzureUSGovernmentCloud AzureBlobCloudType = "AzureUSGovernment"
	AzureChinaCloud        AzureBlobCloudType = "AzureChinaCloud"
)

// MarshalText implements encoding.TextMarshaler
func (a AzureBlobCloudType) MarshalText() (text []byte, err error) {
	return []byte(a), nil
}

// UnmarshalText implements encoding.TextUnmarshaler
func (a *AzureBlobCloudType) UnmarshalText(text []byte) error {
	str := string(text)
	switch str {
	case "AzureCloud":
		*a = AzureCloud
	case "AzureUSGovernment":
		*a = AzureUSGovernmentCloud
	case "AzureChina":
		*a = AzureChinaCloud
	default:
		return fmt.Errorf("unknown azure blob cloud type: %s", str)
	}

	return nil
}

type AzureBlobAuthOptions struct {
	CloudType   AzureBlobCloudType `alloy:"cloud_type,attr,optional"`
	AccountUrl  string             `alloy:"account_url,attr,optional"`
	AccountName string             `alloy:"account_name,attr,optional"`
	AccountKey  string             `alloy:"account_key,attr,optional"`
}

type AWSAuthOptions struct {
	Region    string `alloy:"region,attr,optional"`
	AccessKey string `alloy:"access_key,attr,optional"`
	SecretKey string `alloy:"secret_key,attr,optional"`
	Service   string `alloy:"service,attr,optional"`
}

type OAuth2Settings struct {
	Passthrough bool `alloy:"passthrough,attr,optional"`

	OAuth2Type    string           `alloy:"oauth2_type,attr,optional"`
	ClientID      string           `alloy:"client_id,attr,optional"`
	TokenURL      string           `alloy:"token_url,attr,optional"`
	Email         string           `alloy:"email,attr,optional"`
	PrivateKeyID  string           `alloy:"private_key_id,attr,optional"`
	Subject       string           `alloy:"subject,attr,optional"`
	Scopes        []string         `alloy:"scopes,attr,optional"`
	AuthStyle     oauth2.AuthStyle `alloy:"authStyle,attr,optional"`
	AuthHeader    string           `alloy:"authHeader,attr,optional"`
	TokenTemplate string           `alloy:"tokenTemplate,attr,optional"`
}

func (o *OAuth2Settings) Convert() models.OAuth2Settings {
	if o == nil {
		return models.OAuth2Settings{}
	}

	return models.OAuth2Settings{
		OAuth2Type:    o.OAuth2Type,
		ClientID:      o.ClientID,
		TokenURL:      o.TokenURL,
		Email:         o.Email,
		PrivateKeyID:  o.PrivateKeyID,
		Subject:       o.Subject,
		Scopes:        o.Scopes,
		AuthStyle:     o.AuthStyle,
		AuthHeader:    o.AuthHeader,
		TokenTemplate: o.TokenTemplate,
	}
}

type TLSConfig struct {
	InsecureSkipVerify bool   `alloy:"insecure_skip_verify,attr,optional"`
	ServerName         string `alloy:"server_name,attr,optional"`
	TLSClientAuth      bool   `alloy:"tls_client_auth,attr,optional"`
	TLSAuthWithCACert  bool   `alloy:"tls_auth_with_ca_cert,attr,optional"`
	TLSCACert          string `alloy:"tls_ca_cert,attr,optional"`
	TLSClientCert      string `alloy:"tls_client_cert,attr,optional"`
	TLSClientKey       string `alloy:"tls_client_key,attr,optional"`
}

type ProxyOptions struct {
	ProxyFromEnvironment bool   `alloy:"proxy_from_environment,attr,optional"`
	ProxyUrl             string `alloy:"proxy_url,attr,optional"`
	ProxyUserName        string `alloy:"proxy_username,attr,optional"`
	ProxyPassword        string `alloy:"proxy_password,attr,optional"`
}

type ForwardTo struct {
	Logs    []loki.LogsReceiver  `alloy:"logs,attr,optional"`
	Metrics []storage.Appendable `alloy:"metrics,attr,optional"`
}

// Arguments holds values which are used to configure the infinity component.
type Arguments struct {
	// Optional as we expose the content as an export as well, so users may not intent to forward telemetry directly
	ForwardTo ForwardTo `alloy:"forward_to,block,optional"`

	AzureBlobAuth   *AzureBlobAuthOptions   `alloy:"azure_blob_auth,block,optional"`
	BasicAuth       *BasicAuthOptions       `alloy:"basic_auth,block,optional"`
	APIKeyAuth      *APIKeyAuthOptions      `alloy:"api_key_auth,block,optional"`
	BearerTokenAuth *BearerTokenAuthOptions `alloy:"bearer_token_auth,block,optional"`
	AWSAuth         *AWSAuthOptions         `alloy:"aws_auth,block,optional"`
	OAuth2Settings  *OAuth2Settings         `alloy:"oauth2_settings,block,optional"`
	Proxy           *ProxyOptions           `alloy:"proxy,block,optional"`
	TLS             *TLSConfig              `alloy:"tls,block,optional"`

	CollectionInterval time.Duration `alloy:"collection_interval,attr,optional"`
	Query              Query         `alloy:"query,block"`

	TimeoutInSeconds int64 `alloy:"timeout_in_seconds,attr,optional"`

	// AllowedHosts              []string
	// ReferenceData             []models.RefData
	// CustomHealthCheckEnabled  bool
	// CustomHealthCheckUrl      string
	// UnsecuredQueryHandling    models.UnsecuredQueryHandlingMode
	// PathEncodedURLsEnabled    bool
	// IgnoreStatusCodeCheck     bool
	// AllowDangerousHTTPMethods bool
	// ProxyOpts is used for Secure Socks Proxy configuration
	// ProxyOpts httpclient.Options
	// Specific cookies included by Grafana for forwarding
	// KeepCookies []string
}

type QueryType models.QueryType

func (q QueryType) MarshalText() (text []byte, err error) {
	return []byte(q), nil
}

func (q *QueryType) UnmarshalText(text []byte) error {
	str := string(text)
	switch str {
	case "json":
		*q = QueryType(models.QueryTypeJSON)
	case "csv":
		*q = QueryType(models.QueryTypeCSV)
	case "tsv":
		*q = QueryType(models.QueryTypeTSV)
	case "xml":
		*q = QueryType(models.QueryTypeXML)
	case "graphql":
		*q = QueryType(models.QueryTypeGraphQL)
	case "html":
		*q = QueryType(models.QueryTypeHTML)
	case "uql":
		*q = QueryType(models.QueryTypeUQL)
	case "groq":
		*q = QueryType(models.QueryTypeGROQ)
	case "google-sheets":
		*q = QueryType(models.QueryTypeGSheets)
	default:
		return fmt.Errorf("unknown query type: %s", str)
	}

	return nil
}

type InfinityParser models.InfinityParser

func (p InfinityParser) MarshalText() (text []byte, err error) {
	return []byte(p), nil
}

func (p *InfinityParser) UnmarshalText(text []byte) error {
	str := string(text)
	switch str {
	case "simple":
		*p = InfinityParser(models.InfinityParserSimple)
	case "backend":
		*p = InfinityParser(models.InfinityParserBackend)
	case "jq-backend":
		*p = InfinityParser(models.InfinityParserJQBackend)
	case "uql":
		*p = InfinityParser(models.InfinityParserUQL)
	case "groq":
		*p = InfinityParser(models.InfinityParserGROQ)
	default:
		return fmt.Errorf("unknown infinity parser: %s", str)
	}

	return nil
}

type URLOptions struct {
	Method               string            `alloy:"method,attr"` // 'GET' | 'POST' | 'PATCH' | 'PUT | 'DELETE'
	Params               map[string]string `alloy:"params,attr,optional"`
	Headers              map[string]string `alloy:"headers,attr,optional"`
	Body                 string            `alloy:"data,attr,optional"`
	BodyType             string            `alloy:"body_type,attr,optional"`
	BodyContentType      string            `alloy:"body_content_type,attr,optional"`
	BodyForm             map[string]string `alloy:"body_form,attr,optional"`
	BodyGraphQLQuery     string            `alloy:"body_graphql_query,attr,optional"`
	BodyGraphQLVariables string            `alloy:"body_graphql_variables,attr,optional"`
}

type InfinityCSVOptions struct {
	Delimiter          string `alloy:"delimiter,attr,optional"`
	SkipEmptyLines     bool   `alloy:"skip_empty_lines,attr,optional"`
	SkipLinesWithError bool   `alloy:"skip_lines_with_error,attr,optional"`
	RelaxColumnCount   bool   `alloy:"relax_column_count,attr,optional"`
	Columns            string `alloy:"columns,attr,optional"`
	Comment            string `alloy:"comment,attr,optional"`
}

type InfinityJSONOptions struct {
	RootIsNotArray bool `alloy:"root_is_not_array,attr,optional"`
	ColumnNar      bool `alloy:"columnar,attr,optional"`
}

type InfinityColumn struct {
	Selector        string `alloy:"selector,attr,optional"`
	Text            string `alloy:"text,attr,optional"`
	Type            string `alloy:"type,attr,optional"` // "string" | "number" | "timestamp" | "timestamp_epoch" | "timestamp_epoch_s" | "boolean"
	TimeStampFormat string `alloy:"timestamp_format,attr,optional"`
}

type InfinityFilter struct {
	Field    string   `alloy:"field,attr,optional"`
	Operator string   `alloy:"operator,attr,optional"`
	Value    []string `alloy:"value,attr,optional"`
}

type InfinityDataOverride struct {
	Values   []string `alloy:"values,attr,optional"`
	Operator string   `alloy:"operator,attr,optional"`
	Override string   `alloy:"override,attr,optional"`
}

type Transformation models.Transformation

func (t Transformation) MarshalText() (text []byte, err error) {
	return []byte(t), nil
}

func (t *Transformation) UnmarshalText(text []byte) error {
	str := string(text)
	switch str {
	case "noop":
		*t = Transformation(models.NoOpTransformation)
	case "limit":
		*t = Transformation(models.LimitTransformation)
	case "filterExpression":
		*t = Transformation(models.FilterExpressionTransformation)
	case "summarize":
		*t = Transformation(models.SummarizeTransformation)
	case "computedColumn":
		*t = Transformation(models.ComputedColumnTransformation)
	default:
		return fmt.Errorf("unknown transformation: %s", str)
	}

	return nil
}

type TransformationItem struct {
	Type     Transformation `yaml:"type,attr,optional"`
	Disabled bool           `yaml:"disabled,attr,optional"`
	Limit    struct {
		LimitField int `yaml:"limitField,attr,optional"`
	} `yaml:"limit,block,optional"`
	FilterExpression struct {
		Expression string `yaml:"expression,attr,optional"`
	} `yaml:"filterExpression,block,optional"`
	Summarize struct {
		Expression string `yaml:"expression,attr,optional"`
		By         string `yaml:"by,attr,optional"`
		Alias      string `yaml:"alias,attr,optional"`
	} `yaml:"summarize,block,optional"`
	ComputedColumn struct {
		Expression string `yaml:"expression,attr,optional"`
		Alias      string `yaml:"alias,attr,optional"`
	} `yaml:"computedColumn,block,optional"`
}

type Query struct {
	// RefName             string                 `alloy:"reference_name,attr,optional"`
	Type                QueryType              `alloy:"type,attr"`   // 'json' | 'json-backend' | 'csv' | 'tsv' | 'xml' | 'graphql' | 'html' | 'uql' | 'groq' | 'series' | 'global' | 'google-sheets'
	Format              string                 `alloy:"format,attr"` // 'table' | 'timeseries' | 'logs' | 'dataframe' | 'as-is' | 'node-graph-nodes' | 'node-graph-edges'
	Source              string                 `alloy:"source,attr"` // 'url' | 'inline' | 'azure-blob' | 'reference' | 'random-walk' | 'expression'
	URL                 string                 `alloy:"url,attr,optional"`
	URLOptions          URLOptions             `alloy:"url_options,attr,optional"`
	Data                string                 `alloy:"data,attr,optional"`
	Parser              InfinityParser         `alloy:"parser,attr"` // 'simple' | 'backend' | 'jq-backend' | 'uql' | 'groq'
	FilterExpression    string                 `alloy:"filter_expression,attr,optional"`
	SummarizeExpression string                 `alloy:"summarize_expression,attr,optional"`
	SummarizeBy         string                 `alloy:"summarize_by,attr,optional"`
	SummarizeAlias      string                 `alloy:"summarize_alias,attr,optional"`
	UQL                 string                 `alloy:"uql,attr,optional"`
	GROQ                string                 `alloy:"groq,attr,optional"`
	CSVOptions          InfinityCSVOptions     `alloy:"csv_options,attr,optional"`
	JSONOptions         InfinityJSONOptions    `alloy:"json_options,attr,optional"`
	RootSelector        string                 `alloy:"root_selector,attr,optional"`
	Columns             []InfinityColumn       `alloy:"column,block,optional"`
	ComputedColumns     []InfinityColumn       `alloy:"computed_column,block,optional"`
	Filters             []InfinityFilter       `alloy:"filter,block,optional"`
	SeriesCount         int64                  `alloy:"series_count,attr,optional"`
	Expression          string                 `alloy:"expression,attr,optional"`
	Alias               string                 `alloy:"alias,attr,optional"`
	DataOverrides       []InfinityDataOverride `alloy:"data_override,block,optional"`
	// TODO - add google sheets support
	// Spreadsheet         string                 `json:"spreadsheet,omitempty"`
	// SheetName           string                 `json:"sheetName,omitempty"`
	// SheetRange          string                 `json:"sheetRange,omitempty"`

	AzBlobContainerName string               `alloy:"azure_container_name,attr,optional"`
	AzBlobName          string               `alloy:"azure_blob_name,attr,optional"`
	Transformations     []TransformationItem `alloy:"transformation,block,optional"`
}

func (q *Query) Convert() (models.Query, error) {
	query := models.Query{
		Type:                models.QueryType(q.Type),
		Format:              q.Format,
		Source:              q.Source,
		URL:                 q.URL,
		Data:                q.Data,
		Parser:              models.InfinityParser(q.Parser),
		FilterExpression:    q.FilterExpression,
		SummarizeExpression: q.SummarizeExpression,
		SummarizeBy:         q.SummarizeBy,
		SummarizeAlias:      q.SummarizeAlias,
		UQL:                 q.UQL,
		GROQ:                q.GROQ,
		CSVOptions: models.InfinityCSVOptions{
			Delimiter:          q.CSVOptions.Delimiter,
			SkipEmptyLines:     q.CSVOptions.SkipEmptyLines,
			SkipLinesWithError: q.CSVOptions.SkipLinesWithError,
			RelaxColumnCount:   q.CSVOptions.RelaxColumnCount,
			Columns:            q.CSVOptions.Columns,
			Comment:            q.CSVOptions.Comment,
		},
		JSONOptions: models.InfinityJSONOptions{
			RootIsNotArray: q.JSONOptions.RootIsNotArray,
			ColumnNar:      q.JSONOptions.ColumnNar,
		},
		RootSelector:        q.RootSelector,
		SeriesCount:         q.SeriesCount,
		Expression:          q.Expression,
		Alias:               q.Alias,
		DataOverrides:       []models.InfinityDataOverride{},
		AzBlobContainerName: q.AzBlobContainerName,
		AzBlobName:          q.AzBlobName,
	}

	for _, col := range q.Columns {
		query.Columns = append(query.Columns, models.InfinityColumn{
			Selector:        col.Selector,
			Text:            col.Text,
			Type:            col.Type,
			TimeStampFormat: col.TimeStampFormat,
		})
	}

	for _, col := range q.ComputedColumns {
		query.ComputedColumns = append(query.ComputedColumns, models.InfinityColumn{
			Selector:        col.Selector,
			Text:            col.Text,
			Type:            col.Type,
			TimeStampFormat: col.TimeStampFormat,
		})
	}

	for _, filter := range q.Filters {
		query.Filters = append(query.Filters, models.InfinityFilter{
			Field:    filter.Field,
			Operator: filter.Operator,
			Value:    filter.Value,
		})
	}

	for _, override := range q.DataOverrides {
		query.DataOverrides = append(query.DataOverrides, models.InfinityDataOverride{
			Values:   override.Values,
			Operator: override.Operator,
			Override: override.Override,
		})
	}

	for _, t := range q.Transformations {
		if t.Disabled {
			continue
		}

		tr := models.TransformationItem{
			Type: models.Transformation(t.Type),
		}

		switch tr.Type {
		case models.LimitTransformation:
			tr.Limit = struct {
				LimitField int `json:"limitField,omitempty"`
			}{
				LimitField: t.Limit.LimitField,
			}
		case models.FilterExpressionTransformation:
			tr.FilterExpression = struct {
				Expression string "json:\"expression,omitempty\""
			}{
				Expression: t.FilterExpression.Expression,
			}
		case models.SummarizeTransformation:
			tr.Summarize = struct {
				Expression string "json:\"expression,omitempty\""
				By         string "json:\"by,omitempty\""
				Alias      string "json:\"alias,omitempty\""
			}{
				Expression: t.Summarize.Expression,
				By:         t.Summarize.By,
				Alias:      t.Summarize.Alias,
			}
		case models.ComputedColumnTransformation:
			tr.ComputedColumn = struct {
				Expression string "json:\"expression,omitempty\""
				Alias      string "json:\"alias,omitempty\""
			}{
				Expression: t.ComputedColumn.Expression,
				Alias:      t.ComputedColumn.Alias,
			}
		}

		query.Transformations = append(query.Transformations, tr)
	}

	// URLOptions
	method := strings.ToUpper(strings.TrimSpace(q.URLOptions.Method))
	if method == "" {
		method = "GET"
	}
	if method != "GET" && method != "POST" && method != "PUT" && method != "PATCH" && method != "DELETE" {
		return query, fmt.Errorf("invalid method %s provided. supported methods are GET, POST, PUT, PATCH and DELETE", method)
	}
	query.URLOptions.Method = method
	query.URLOptions.Params = make([]models.URLOptionKeyValuePair, 0, len(q.URLOptions.Params))
	query.URLOptions.Headers = make([]models.URLOptionKeyValuePair, 0, len(q.URLOptions.Headers))
	query.URLOptions.Body = q.URLOptions.Body
	query.URLOptions.BodyType = q.URLOptions.BodyType
	query.URLOptions.BodyContentType = q.URLOptions.BodyContentType
	query.URLOptions.BodyForm = make([]models.URLOptionKeyValuePair, 0, len(q.URLOptions.BodyForm))
	query.URLOptions.BodyGraphQLQuery = q.URLOptions.BodyGraphQLQuery
	query.URLOptions.BodyGraphQLVariables = q.URLOptions.BodyGraphQLVariables

	for k, v := range q.URLOptions.Headers {
		query.URLOptions.Headers = append(query.URLOptions.Headers, models.URLOptionKeyValuePair{
			Key:   k,
			Value: v,
		})
	}

	for k, v := range q.URLOptions.Params {
		query.URLOptions.Params = append(query.URLOptions.Params, models.URLOptionKeyValuePair{
			Key:   k,
			Value: v,
		})
	}

	for k, v := range q.URLOptions.BodyForm {
		query.URLOptions.BodyForm = append(query.URLOptions.BodyForm, models.URLOptionKeyValuePair{
			Key:   k,
			Value: v,
		})
	}

	return query, nil
}

// DefaultArguments provides the default arguments for the infinity
// component.
var DefaultArguments = Arguments{
	CollectionInterval: 60 * time.Second,
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

func (a *Arguments) ConvertToInfinity() models.InfinitySettings {
	s := models.InfinitySettings{
		AuthenticationMethod: models.AuthenticationMethodNone,
		// URL:                  a.URL,
		// CustomHeaders:        a.CustomHeaders,
		// SecureQueryFields:    a.SecureQueryFields,
	}

	if a.TLS != nil {
		s.TLSAuthWithCACert = a.TLS.TLSAuthWithCACert
		s.TLSCACert = a.TLS.TLSCACert
		s.TLSClientCert = a.TLS.TLSClientCert
		s.TLSClientKey = a.TLS.TLSClientKey
		s.InsecureSkipVerify = a.TLS.InsecureSkipVerify
		s.ServerName = a.TLS.ServerName
		s.TLSClientAuth = a.TLS.TLSClientAuth
	}

	if a.Proxy != nil {
		if a.Proxy.ProxyFromEnvironment == true {
			s.ProxyType = models.ProxyTypeEnv
		} else {
			s.ProxyType = models.ProxyTypeUrl
			s.ProxyUrl = a.Proxy.ProxyUrl
			s.ProxyUserName = a.Proxy.ProxyUserName
			s.ProxyUserPassword = a.Proxy.ProxyPassword
		}
	}

	if a.BasicAuth != nil {
		s.AuthenticationMethod = models.AuthenticationMethodBasic
		s.BasicAuthEnabled = true
		s.UserName = a.BasicAuth.UserName
		s.Password = a.BasicAuth.Password
	} else if a.BearerTokenAuth != nil {
		s.AuthenticationMethod = models.AuthenticationMethodBearerToken
		s.BearerToken = a.BearerTokenAuth.Token
	} else if a.OAuth2Settings != nil {
		if a.OAuth2Settings.Passthrough {
			s.AuthenticationMethod = models.AuthenticationMethodForwardOauth
		} else {
			s.AuthenticationMethod = models.AuthenticationMethodOAuth
			s.OAuth2Settings = a.OAuth2Settings.Convert()
		}
	} else if a.APIKeyAuth != nil {
		s.AuthenticationMethod = models.AuthenticationMethodApiKey
		s.ApiKeyType = string(a.APIKeyAuth.Type)
		s.ApiKeyKey = a.APIKeyAuth.Key
		s.ApiKeyValue = a.APIKeyAuth.Value
	} else if a.AWSAuth != nil {
		s.AuthenticationMethod = models.AuthenticationMethodAWS
		s.AWSSettings = models.AWSSettings{
			Region:   a.AWSAuth.Region,
			Service:  a.AWSAuth.Service,
			AuthType: models.AWSAuthTypeKeys,
		}
		s.AWSAccessKey = a.AWSAuth.AccessKey
		s.AWSSecretKey = a.AWSAuth.SecretKey
	} else if a.AzureBlobAuth != nil {
		s.AuthenticationMethod = models.AuthenticationMethodAzureBlob
		s.AzureBlobAccountKey = a.AzureBlobAuth.AccountKey
		s.AzureBlobAccountName = a.AzureBlobAuth.AccountName
		s.AzureBlobAccountUrl = a.AzureBlobAuth.AccountUrl
		s.AzureBlobCloudType = string(a.AzureBlobAuth.CloudType)
	}

	return s
}

// Exports holds values which are exported by the infinity component.
type Exports struct {
	Content alloytypes.OptionalSecret `alloy:"content,attr"`
}

// Component implements the infinity component.
type Component struct {
	opts component.Options

	mut  sync.Mutex
	args Arguments

	healthMut sync.RWMutex
	health    component.Health

	metricsFanout *prometheus.Fanout

	infinitySettings *models.InfinitySettings
	infinityQuery    *models.Query
	client           *http.Client
	azClient         *azblob.Client

	collectTick *time.Ticker
}

var (
	_ component.Component       = (*Component)(nil)
	_ component.HealthComponent = (*Component)(nil)
)

// New creates a new infinity component.
func New(o component.Options, args Arguments) (*Component, error) {
	service, err := o.GetServiceData(labelstore.ServiceName)
	if err != nil {
		return nil, err
	}
	ls := service.(labelstore.LabelStore)
	c := &Component{
		opts:          o,
		metricsFanout: prometheus.NewFanout(nil, o.ID, o.Registerer, ls, prometheus.NoopMetadataStore{}),
		collectTick:   time.NewTicker(args.CollectionInterval),
	}

	backend.Logger = loggerHandler{
		Logger: o.Logger,
		Lvl:    backendLog.Debug,
	}

	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		c.mut.Lock()
		defer c.mut.Unlock()
		if c.collectTick != nil {
			c.collectTick.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.collectTick.C:
			c.mut.Lock()

			// TODO support multiple queries w/ inlines/etc
			output, status, err := c.getResults(ctx)
			if err != nil {
				c.setHealth(component.Health{
					Health:     component.HealthTypeUnhealthy,
					Message:    err.Error(),
					UpdateTime: time.Now(),
				})
				level.Error(c.opts.Logger).Log("msg", "infinity component query failed", "status", status, "output", output, "error", err)
				c.mut.Unlock()
				continue
			}

			if status >= 200 && status < 300 {
				c.setHealth(component.Health{
					Health:     component.HealthTypeHealthy,
					Message:    fmt.Sprintf("Last query was successful. Status code: %d", status),
					UpdateTime: time.Now(),
				})
				// level.Debug(c.opts.Logger).Log("msg", "infinity component query successful", "status", status, "output", output)
			} else {
				c.setHealth(component.Health{
					Health:     component.HealthTypeUnhealthy,
					Message:    fmt.Sprintf("Last query was unsuccessful. Status code: %d", status),
					UpdateTime: time.Now(),
				})
				level.Error(c.opts.Logger).Log("msg", "infinity component query failed", "status", status, "output", output)
				c.mut.Unlock()
				continue
			}

			frame, err := c.parseResponse(ctx, output)
			// b, _ := frame.MarshalJSON()
			// level.Debug(c.opts.Logger).Log("msg", "infinity component parsed response", "frame", string(b))

			if frame.Meta.Type.IsLogs() {
				logs, err := convertFrameToLokiEntries(frame)
				if err != nil {
					level.Error(c.opts.Logger).Log("msg", "error converting frame to loki entries", "error", err)
					c.mut.Unlock()
					continue
				}
				for _, r := range c.args.ForwardTo.Logs {
					if r == nil {
						continue
					}
					for _, l := range logs {
						r.Chan() <- l
					}
				}
				level.Debug(c.opts.Logger).Log("msg", "forwarded logs", "count", len(logs))
			} else if frame.Meta.Type.IsTimeSeries() || frame.Meta.Type.IsNumeric() {
				var metrics []sample
				var err error
				if frame.Meta.Type.IsTimeSeries() {
					metrics, err = c.convertTimeSeriesFrameToSamples(frame)
				} else {
					metrics, err = c.convertNumericFrameToSamples(frame, time.Now())
				}
				if err != nil {
					level.Error(c.opts.Logger).Log("msg", "error converting frame to metrics", "error", err)
					c.mut.Unlock()
					continue
				}
				if len(metrics) == 0 {
					level.Debug(c.opts.Logger).Log("msg", "no metrics parsed from the response")
					c.mut.Unlock()
					continue
				}

				app := c.metricsFanout.Appender(ctx)
				appended := map[uint64]struct{}{}
				for _, m := range metrics {
					// level.Debug(c.opts.Logger).Log("msg", "forwarding metric", "labels", fmt.Sprintf("%v", m.labels), "timestamp", m.timestamp, "value", m.value)
					// Avoid appending same metric multiple times in one batch as it can cause errors in some other components like otelcol.prometheus.receiver
					hash := m.labels.Hash()
					if _, ok := appended[hash]; ok {
						err := app.Commit()
						if err != nil {
							level.Error(c.opts.Logger).Log("msg", "error committing to appender", "error", err)
						}
						app = c.metricsFanout.Appender(ctx)
						appended = map[uint64]struct{}{}
					}
					_, err := app.Append(0, m.labels, m.timestamp.Unix(), m.value)
					if err != nil {
						level.Error(c.opts.Logger).Log("msg", "error appending metric", "error", err)
						app.Rollback()
						break
					}
					appended[hash] = struct{}{}
					if err = app.Commit(); err != nil {
						level.Error(c.opts.Logger).Log("msg", "error committing to appender", "error", err)
						break
					}
				}
				level.Debug(c.opts.Logger).Log("msg", "forwarded metrics", "count", len(metrics))
			} else {
				level.Info(c.opts.Logger).Log("msg", "unsupported frame type", "type", frame.Meta.Type)
			}
			c.mut.Unlock()
		}
	}
}

func (c *Component) getResults(ctx context.Context) (any, int, error) {
	if c.args.Query.Source == "azure-blob" {
		return c.getAzureBlob(ctx)
	} else {
		return c.getHTTPResponse(ctx)
	}
}

func (c *Component) getAzureBlob(ctx context.Context) (any, int, error) {
	if strings.TrimSpace(c.args.Query.AzBlobContainerName) == "" || strings.TrimSpace(c.args.Query.AzBlobName) == "" {
		return nil, http.StatusBadRequest, errors.New("invalid/empty container name/blob name")
	}
	if c.azClient == nil {
		return nil, http.StatusInternalServerError, errors.New("invalid azure blob client")
	}
	blobDownloadResponse, err := c.azClient.DownloadStream(ctx, strings.TrimSpace(c.args.Query.AzBlobContainerName), strings.TrimSpace(c.args.Query.AzBlobName), nil)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	reader := blobDownloadResponse.Body
	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("error reading blob content. %s", err)
	}
	bodyBytes = removeBOMContent(bodyBytes)
	if ds.CanParseAsJSON(models.QueryType(c.args.Query.Type), http.Header{}) {
		var out any
		err := json.Unmarshal(bodyBytes, &out)
		if err != nil {
			logger.Error("error un-marshaling blob content", "error", err.Error())
		}
		return out, http.StatusOK, err
	}
	return string(bodyBytes), http.StatusOK, nil
}

func (c *Component) getHTTPResponse(ctx context.Context) (any, int, error) {
	body := ds.GetQueryBody(ctx, *c.infinityQuery)
	headers := map[string]string{}
	req, err := ds.GetRequest(ctx, nil, *c.infinitySettings, body, *c.infinityQuery, headers, true)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("error preparing request. %w", err)
	}
	if req == nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("error preparing request. invalid request constructed")
	}
	startTime := time.Now()
	if !ds.CanAllowURL(req.URL.String(), c.infinitySettings.AllowedHosts) {
		//logger.Debug("url is not in the allowed list. make sure to match the base URL with the settings", "url", req.URL.String())
		return nil, http.StatusUnauthorized, models.ErrInvalidConfigHostNotAllowed
	}
	//logger.Debug("requesting URL", "host", req.URL.Hostname(), "url_path", req.URL.Path, "method", req.Method, "type", query.Type)
	res, err := c.client.Do(req)
	duration := time.Since(startTime)
	level.Debug(c.opts.Logger).Log("msg", "received response", "host", req.URL.Hostname(), "url_path", req.URL.Path, "method", req.Method, "type", c.infinityQuery.Type, "duration_ms", duration.Milliseconds())
	if res != nil {
		defer func() {
			if err := res.Body.Close(); err != nil {
				logger.Warn("error closing response body", "error", err.Error())
			}
		}()
	}
	if err != nil {
		if res != nil {
			level.Debug(c.opts.Logger).Log("msg", "error getting response from server", "url", req.URL.String(), "method", req.Method, "error", err.Error(), "status code", res.StatusCode)
			// Infinity can query anything and users are responsible for ensuring that endpoint/auth is correct
			// therefore any incoming error is considered downstream
			return nil, res.StatusCode, fmt.Errorf("error getting response from %s", req.URL.String())
		}
		if errors.Is(err, context.Canceled) {
			level.Debug(c.opts.Logger).Log("msg", "request cancelled", "url", req.URL.String(), "method", req.Method)
			return nil, http.StatusInternalServerError, err
		}
		level.Debug(c.opts.Logger).Log("msg", "error getting response from server. no response received", "url", req.URL.String(), "error", err.Error())
		return nil, http.StatusInternalServerError, fmt.Errorf("error getting response from url %s. no response received. Error: %w", req.URL.String(), err)
	}
	if res == nil {
		level.Debug(c.opts.Logger).Log("msg", "invalid response from server and also no error", "url", req.URL.String(), "method", req.Method)
		return nil, http.StatusInternalServerError, fmt.Errorf("invalid response received for the URL %s", req.URL.String())
	}
	if res.StatusCode >= http.StatusBadRequest && !c.infinitySettings.IgnoreStatusCodeCheck {
		err = fmt.Errorf("%w\nstatus code : %s", models.ErrUnsuccessfulHTTPResponseStatus, res.Status)
		// Infinity can query anything and users are responsible for ensuring that endpoint/auth is correct
		// therefore any incoming error is considered downstream
		return nil, res.StatusCode, err
	}
	bodyBytes, err := getBodyBytes(res)
	if err != nil {
		return nil, res.StatusCode, err
	}
	if len(bodyBytes) == 0 {
		return nil, res.StatusCode, fmt.Errorf("empty response body received for the URL %s", req.URL.String())
	}
	bodyBytes = removeBOMContent(bodyBytes)
	if ds.CanParseAsJSON(c.infinityQuery.Type, res.Header) {
		var out any
		err := json.Unmarshal(bodyBytes, &out)
		if err != nil {
			err = fmt.Errorf("%w. %w", models.ErrParsingResponseBodyAsJson, err)
			//logger.Debug("error un-marshaling JSON response", "url", url, "error", err.Error())
		}
		return out, res.StatusCode, err
	}
	return string(bodyBytes), res.StatusCode, err
}

func getBodyBytes(res *http.Response) ([]byte, error) {
	if res == nil || res.Body == nil {
		return nil, errors.New("invalid/empty response received from underlying API")
	}
	if strings.EqualFold(res.Header.Get("Content-Encoding"), "gzip") {
		reader, err := gzip.NewReader(res.Body)
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = reader.Close()
		}()
		return io.ReadAll(reader)
	}
	return io.ReadAll(res.Body)
}

// https://stackoverflow.com/questions/31398044/got-error-invalid-character-%C3%AF-looking-for-beginning-of-value-from-json-unmar
func removeBOMContent(input []byte) []byte {
	return bytes.TrimPrefix(input, []byte("\xef\xbb\xbf"))
}

func (c *Component) parseResponse(ctx context.Context, response any) (*data.Frame, error) {
	var frame *data.Frame
	var err error
	if c.infinityQuery.Type == models.QueryTypeJSON || c.infinityQuery.Type == models.QueryTypeGraphQL {
		level.Debug(c.opts.Logger).Log("msg", "parsing response as JSON")
		if frame, err = ds.GetJSONBackendResponse(ctx, response, *c.infinityQuery); err != nil {
			return frame, err
		}
	}
	if c.infinityQuery.Type == models.QueryTypeCSV || c.infinityQuery.Type == models.QueryTypeTSV {
		level.Debug(c.opts.Logger).Log("msg", "parsing response as CSV/TSV")
		if responseString, ok := response.(string); ok {
			if frame, err = ds.GetCSVBackendResponse(ctx, responseString, *c.infinityQuery); err != nil {
				return frame, err
			}
		}
	}
	if c.infinityQuery.Type == models.QueryTypeXML || c.infinityQuery.Type == models.QueryTypeHTML {
		level.Debug(c.opts.Logger).Log("msg", "parsing response as XML/HTML")
		if responseString, ok := response.(string); ok {
			if frame, err = ds.GetXMLBackendResponse(ctx, responseString, *c.infinityQuery); err != nil {
				return frame, err
			}
		}
	}

	frame, err = ds.PostProcessFrame(ctx, frame, *c.infinityQuery)
	if err != nil {
		return nil, err
	}

	c.checkFrameMetadata(frame)

	return frame, nil
}

func (c *Component) checkFrameMetadata(frame *data.Frame) {
	if frame == nil {
		return
	}

	if frame.Meta.Type.IsKnownType() {
		// type is already set
		return
	}

	switch c.infinityQuery.Format {
	case "logs":
		frame.Meta.Type = data.FrameTypeLogLines
		frame.Meta.TypeVersion = data.FrameTypeVersion{0, 0}
	case "timeseries":
		schema := frame.TimeSeriesSchema()
		switch schema.Type {
		case data.TimeSeriesTypeLong:
			frame.Meta.Type = data.FrameTypeTimeSeriesLong
			frame.Meta.TypeVersion = data.FrameTypeVersion{0, 0}
		case data.TimeSeriesTypeWide:
			frame.Meta.Type = data.FrameTypeTimeSeriesWide
			frame.Meta.TypeVersion = data.FrameTypeVersion{0, 0}
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()
	c.args = newArgs

	c.collectTick.Reset(c.args.CollectionInterval)

	c.metricsFanout.UpdateChildren(c.args.ForwardTo.Metrics)

	settings := newArgs.ConvertToInfinity()
	err := settings.Validate()
	if err != nil {
		return err
	}

	c.infinitySettings = &settings

	query, err := newArgs.Query.Convert()
	if err != nil {
		return fmt.Errorf("error converting query settings. %w", err)
	}

	c.infinityQuery = &query

	ctx := context.Background()
	client, err := httpclient.GetHTTPClient(ctx, settings)
	if err != nil {
		return err
	}

	if settings.AuthenticationMethod == models.AuthenticationMethodAzureBlob {
		azClient, err := createAzureBlobClient(settings)
		if err != nil {
			return err
		}

		c.azClient = azClient
	}

	c.client = client
	return nil
}

func createAzureBlobClient(settings models.InfinitySettings) (*azblob.Client, error) {
	cred, err := azblob.NewSharedKeyCredential(settings.AzureBlobAccountName, settings.AzureBlobAccountKey)
	if err != nil {
		return nil, errors.New("invalid azure blob credentials")
	}
	clientUrl := "https://%s.blob.core.windows.net/"
	if settings.AzureBlobAccountUrl != "" {
		clientUrl = settings.AzureBlobAccountUrl
	}
	if strings.Contains(clientUrl, "%s") {
		clientUrl = fmt.Sprintf(clientUrl, settings.AzureBlobAccountName)
	}
	azClient, err := azblob.NewClientWithSharedKeyCredential(clientUrl, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating azure blob client. %s", err)
	}
	if azClient == nil {
		return nil, errors.New("invalid/empty azure blob client")
	}

	return azClient, nil
}

// CurrentHealth implements component.HealthComponent.
func (c *Component) CurrentHealth() component.Health {
	c.healthMut.RLock()
	defer c.healthMut.RUnlock()
	return c.health
}

func (c *Component) setHealth(h component.Health) {
	c.healthMut.Lock()
	defer c.healthMut.Unlock()
	c.health = h
}

// See https://grafana.github.io/dataplane/logs#loglines
func convertFrameToLokiEntries(frame *data.Frame) ([]loki.Entry, error) {
	if frame == nil {
		return nil, errors.New("invalid/empty frame provided")
	}
	var entries []loki.Entry
	// The first field with name "timestamp" is a time
	var timestampField *data.Field
	// The first field with name "body" is a log body
	var bodyField *data.Field
	// The first field with name "severity" is a log level
	var severityField *data.Field
	// The first field with name "labels" is a set of labels in json.RawMessage encoding
	var labelsField *data.Field
	// The first field with name "structured_metadata" is a set of structured metadata in json.RawMessage encoding
	// This is not according to the spec, but it seems like a reasonable extension
	var structuredMetadataField *data.Field

	for _, f := range frame.Fields {
		if f.Name == "timestamp" && timestampField == nil {
			timestampField = f
		} else if f.Name == "body" && bodyField == nil {
			bodyField = f
		} else if f.Name == "severity" && severityField == nil {
			severityField = f
		} else if f.Name == "labels" && labelsField == nil {
			labelsField = f
		} else if f.Name == "structured_metadata" && structuredMetadataField == nil {
			structuredMetadataField = f
		}
	}

	var err error
	for i := 0; i < frame.Rows(); i++ {
		var entry loki.Entry
		if timestampField != nil {
			if v, ok := timestampField.At(i).(time.Time); ok {
				entry.Timestamp = v
			} else {
				errors.Join(err, fmt.Errorf("invalid timestamp field at row %d: %v", i, v))
				continue
			}
		}
		if bodyField != nil {
			if v, ok := bodyField.At(i).(string); ok {
				entry.Line = v
			} else {
				errors.Join(err, fmt.Errorf("invalid body field at row %d: %v", i, v))
				continue
			}
		}
		if labelsField != nil {
			if v, ok := labelsField.At(i).(json.RawMessage); ok {
				var m map[string]string
				if err := json.Unmarshal(v, &m); err != nil {
					errors.Join(err, fmt.Errorf("invalid labels field at row %d: %v", i, err))
				} else {
					entry.Labels = utils.ToLabelSet(m)
				}
			} else {
				errors.Join(err, fmt.Errorf("invalid labels field at row %d: %v", i, v))
			}
		}
		if structuredMetadataField != nil {
			if v, ok := structuredMetadataField.At(i).(json.RawMessage); ok {
				var m map[string]any
				if err := json.Unmarshal(v, &m); err != nil {
					errors.Join(err, fmt.Errorf("invalid structured_metadata field at row %d: %v", i, err))
				} else {
					for k, v := range m {
						v, err := json.Marshal(v)
						if err != nil {
							errors.Join(err, fmt.Errorf("invalid structured_metadata field at row %d: %v", i, err))
							continue
						}
						entry.StructuredMetadata = append(entry.StructuredMetadata, push.LabelAdapter{Name: k, Value: string(v)})
					}
				}
			} else {
				errors.Join(err, fmt.Errorf("invalid structured_metadata field at row %d: %v", i, v))
			}
		}
		if severityField != nil {
			if v, ok := severityField.At(i).(string); ok {
				entry.Labels[model.LabelName("severity")] = model.LabelValue(v)
			} else {
				errors.Join(err, fmt.Errorf("invalid severity field at row %d: %v", i, v))
			}
		}
		entries = append(entries, entry)
	}
	return entries, err
}

type sample struct {
	labels    labels.Labels
	timestamp time.Time
	value     float64
}

// See https://grafana.github.io/dataplane/timeseries/
func (c *Component) convertTimeSeriesFrameToSamples(frame *data.Frame) ([]sample, error) {
	if frame == nil {
		return nil, errors.New("invalid/empty frame provided")
	}
	var entries []sample
	var errs error

	baseLabels := make(map[string]string, len(frame.Fields))
	baseLabels[model.JobLabel] = c.opts.ID
	baseLabels[model.InstanceLabel] = c.opts.ID

	switch frame.Meta.Type {
	case data.FrameTypeTimeSeriesLong:
		for i := 0; i < frame.Rows(); i++ {
			var t time.Time

			for _, f := range frame.Fields {
				if (f.Type() == data.FieldTypeTime || f.Type() == data.FieldTypeNullableTime) && t.IsZero() {
					ts, err := parseTimestampFromField(f, i)
					if err != nil {
						errors.Join(errs, err)
						continue
					}
					t = *ts
				} else if f.Type() == data.FieldTypeString || f.Type() == data.FieldTypeNullableString {
					v, err := parseStringFromField(f, i)
					if err != nil {
						errors.Join(errs, err)
						continue
					}
					baseLabels[f.Name] = *v
				}
			}

			if t.IsZero() {
				errors.Join(errs, errors.New("missing time field"))
				break
			}

			for _, f := range frame.Fields {
				if f.Type() == data.FieldTypeFloat64 || f.Type() == data.FieldTypeNullableFloat64 {
					s, err := parseFloatSampleFromField(f, i, t, baseLabels, false)
					if err != nil {
						errors.Join(errs, err)
						continue
					}
					if s != nil {
						entries = append(entries, *s)
					}
				}
			}
		}
	case data.FrameTypeTimeSeriesWide, data.FrameTypeTimeSeriesMulti:
		var tsField *data.Field
		for _, f := range frame.Fields {
			if f.Type() == data.FieldTypeTime || f.Type() == data.FieldTypeNullableTime {
				tsField = f
				break
			}
		}
		if tsField == nil {
			return nil, errors.New("missing time field")
		}
		for i := 0; i < frame.Rows(); i++ {
			timestamp, err := parseTimestampFromField(tsField, i)
			if err != nil {
				errors.Join(errs, err)
				continue
			}
			for _, f := range frame.Fields {
				if f.Type() == data.FieldTypeFloat64 || f.Type() == data.FieldTypeNullableFloat64 {
					s, err := parseFloatSampleFromField(f, i, *timestamp, baseLabels, true)
					if err != nil {
						errors.Join(errs, err)
						continue
					}
					if s != nil {
						entries = append(entries, *s)
					}
				}
			}
		}
	}
	return entries, errs
}

// See https://grafana.github.io/dataplane/numeric
func (c *Component) convertNumericFrameToSamples(frame *data.Frame, timestamp time.Time) ([]sample, error) {
	if frame == nil {
		return nil, errors.New("invalid/empty frame provided")
	}
	var entries []sample
	var errs error

	baseLabels := make(map[string]string, len(frame.Fields))
	baseLabels[model.JobLabel] = c.opts.ID
	baseLabels[model.InstanceLabel] = c.opts.ID

	switch frame.Meta.Type {
	case data.FrameTypeNumericLong:
		for i := 0; i < frame.Rows(); i++ {
			// Parse all the labels first
			for _, f := range frame.Fields {
				if f.Type() == data.FieldTypeString || f.Type() == data.FieldTypeNullableString {
					v, err := parseStringFromField(f, i)
					if err != nil {
						errors.Join(errs, err)
						continue
					}
					baseLabels[f.Name] = *v
				}
			}

			// Iterate again for all value fields
			for _, f := range frame.Fields {
				if f.Type() == data.FieldTypeFloat64 || f.Type() == data.FieldTypeNullableFloat64 {
					s, err := parseFloatSampleFromField(f, i, timestamp, baseLabels, false)
					if err != nil {
						errors.Join(errs, err)
						continue
					}
					if s != nil {
						entries = append(entries, *s)
					}
				}
			}
		}
	case data.FrameTypeNumericWide, data.FrameTypeNumericMulti:
		for i := 0; i < frame.Rows(); i++ {
			for _, f := range frame.Fields {
				if f.Type() == data.FieldTypeFloat64 || f.Type() == data.FieldTypeNullableFloat64 {
					s, err := parseFloatSampleFromField(f, i, timestamp, baseLabels, true)
					if err != nil {
						errors.Join(errs, err)
						continue
					}
					if s != nil {
						entries = append(entries, *s)
					}
				}
			}
		}
	}
	return entries, errs
}

func parseFloatSampleFromField(f *data.Field, row int, t time.Time, baseLabels map[string]string, useFieldLabels bool) (*sample, error) {
	var value *float64
	if f.Type() == data.FieldTypeFloat64 {
		if v, ok := f.At(row).(float64); ok {
			value = &v
		} else {
			return nil, fmt.Errorf("invalid float64 field at row %d: %v", row, f.At(row))
		}
	} else if f.Type() == data.FieldTypeNullableFloat64 {
		if v, ok := f.At(row).(*float64); ok && v != nil {
			value = v
		} else if !ok {
			return nil, fmt.Errorf("invalid nullable float64 field at row %d: %v", row, f.At(row))
		}
	}
	if value == nil {
		return nil, errors.New("field did not contain value")
	}
	var l labels.Labels
	if useFieldLabels {
		l = labels.FromMap(f.Labels)
		for k, v := range baseLabels {
			l = append(l, labels.Label{
				Name:  k,
				Value: v,
			})
		}
	} else {
		l = labels.FromMap(baseLabels)
	}
	l = append(l, labels.Label{
		Name:  model.MetricNameLabel,
		Value: f.Name,
	})

	s := sample{
		value:     *value,
		timestamp: t,
		labels:    l,
	}
	return &s, nil
}

func parseTimestampFromField(f *data.Field, row int) (*time.Time, error) {
	if f.Type() == data.FieldTypeTime {
		if v, ok := f.At(row).(time.Time); ok {
			return &v, nil
		} else {
			return nil, fmt.Errorf("invalid time field at row %d: %v", row, f.At(row))
		}
	} else if f.Type() == data.FieldTypeNullableTime {
		if v, ok := f.At(row).(*time.Time); ok && v != nil {
			return v, nil
		} else if !ok {
			return nil, fmt.Errorf("invalid nullable time field at row %d: %v", row, f.At(row))
		}
	}
	return nil, errors.New("field did not contain time")
}

func parseStringFromField(f *data.Field, row int) (*string, error) {
	if f.Type() == data.FieldTypeString {
		if v, ok := f.At(row).(string); ok {
			return &v, nil
		} else {
			return nil, fmt.Errorf("invalid string field at row %d: %v", row, f.At(row))
		}
	} else if f.Type() == data.FieldTypeNullableString {
		if v, ok := f.At(row).(*string); ok && v != nil {
			return v, nil
		} else if !ok {
			return nil, fmt.Errorf("invalid nullable string field at row %d: %v", row, f.At(row))
		}
	}
	return nil, errors.New("field did not contain string")
}
