package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/sendgrid/sendgrid-go"
)

type SendgridStats []struct {
	Date  string `json:"date"`
	Stats []struct {
		Metrics struct {
			Blocks           int `json:"blocks"`
			BounceDrops      int `json:"bounce_drops"`
			Bounces          int `json:"bounces"`
			Clicks           int `json:"clicks"`
			Deferred         int `json:"deferred"`
			Delivered        int `json:"delivered"`
			InvalidEmails    int `json:"invalid_emails"`
			Opens            int `json:"opens"`
			Processed        int `json:"processed"`
			Requests         int `json:"requests"`
			SpamReportDrops  int `json:"spam_report_drops"`
			SpamReports      int `json:"spam_reports"`
			UniqueClicks     int `json:"unique_clicks"`
			UniqueOpens      int `json:"unique_opens"`
			UnsubscribeDrops int `json:"unsubscribe_drops"`
			Unsubscribes     int `json:"unsubscribes"`
		} `json:"metrics"`
	} `json:"stats"`
}

type SendgridQuery struct {
	Constant      float64 `json:"constant"`
	Datasource    string  `json:"datasource"`
	DatasourceID  int     `json:"datasourceId"`
	IntervalMs    int     `json:"intervalMs"`
	MaxDataPoints int     `json:"maxDataPoints"`
	OrgID         int     `json:"orgId"`
	QueryText     string  `json:"queryText"`
	RefID         string  `json:"refId"`
}

type SendgridPluginConfig struct {
  SendgridAPIKey string `json:"sendgridApiKey"`
}

// newSendgridDataSource returns datasource.ServeOpts.
func newSendgridDataSource() datasource.ServeOpts {
	// creates a instance manager for your plugin. The function passed
	// into `NewInstanceManger` is called when the instance is created
	// for the first time or when a datasource configuration changed.
	im := datasource.NewInstanceManager(newDataSourceInstance)

	apiKey := os.Getenv("SENDGRID_API_KEY")
	ds := &SendgridDataSource{
		im:             im,
		sendgridApiKey: apiKey,
		host:           "https://api.sendgrid.com",
	}

	return datasource.ServeOpts{
		QueryDataHandler:   ds,
		CheckHealthHandler: ds,
	}
}

// SendgridDataSource.... all things DD :)
type SendgridDataSource struct {
	// The instance manager can help with lifecycle management
	// of datasource instances in plugins. It's not a requirements
	// but a best practice that we recommend that you follow.
	im instancemgmt.InstanceManager

	// Sendgrid API key
	sendgridApiKey string
	host           string
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifer).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (td *SendgridDataSource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	log.DefaultLogger.Info("QueryData", "request", req)

  configBytes, _ := req.PluginContext.DataSourceInstanceSettings.JSONData.MarshalJSON()
  var config SendgridPluginConfig
  err := json.Unmarshal(configBytes, &config)
  if err != nil {
    return nil, err
  }
  td.sendgridApiKey = config.SendgridAPIKey

  log.DefaultLogger.Info("SG API KEY", "request", td.sendgridApiKey)

  // create response struct
	response := backend.NewQueryDataResponse()

	// loop over queries and execute them individually.
	for _, q := range req.Queries {
		res, err := td.query(ctx, q)
		if err != nil {
			return nil, err
		}

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = *res
	}

	return response, nil
}

type queryModel struct {
	Format string `json:"format"`
}

func (td *SendgridDataSource) query(ctx context.Context, query backend.DataQuery) (*backend.DataResponse, error) {
	// Unmarshal the json into our queryModel
	var qm queryModel

	queryBytes, _ := query.JSON.MarshalJSON()
	var sgQuery SendgridQuery
	err := json.Unmarshal(queryBytes, &sgQuery)
	if err != nil {
		// empty response? or real error? figure out later.
		return nil, err
	}

	response := backend.DataResponse{}
	response.Error = json.Unmarshal(query.JSON, &qm)
	if response.Error != nil {
		return nil, err
	}

	// Log a warning if `Format` is empty.
	if qm.Format == "" {
		log.DefaultLogger.Warn("format is empty. defaulting to time series")
	}

	request := sendgrid.GetRequest(td.sendgridApiKey, "/v3/stats", td.host)
	request.Method = "GET"
	queryParams := make(map[string]string)
	queryParams["aggregated_by"] = "day"
	queryParams["limit"] = "1"
	queryParams["start_date"] = query.TimeRange.From.UTC().Format("2006-01-02")
	queryParams["end_date"] = query.TimeRange.To.UTC().Add(-24*time.Hour).Format("2006-01-02")
	queryParams["offset"] = "1"
	request.QueryParams = queryParams
	resp, err := sendgrid.API(request)
	if err != nil {
		log.DefaultLogger.Error("Cannot query sendgrid : %s", err.Error())
		return nil, err
	}

  vv := fmt.Sprintf("SG REQ %v", request)
  log.DefaultLogger.Info(vv)

	var sgStats SendgridStats
	_ = json.Unmarshal([]byte(resp.Body), &sgStats)

	fmt.Printf("stats %v\n", sgStats)

  vv = fmt.Sprintf("SG STATS %v", sgStats)
  log.DefaultLogger.Info(vv)

	// create data frame response
	frame := data.NewFrame("response")

	// generate time slice.
	times := []time.Time{}
	blocks := []int64{}
	bounceDrops := []int64{}
	bounces := []int64{}
	clicks := []int64{}
	deferred := []int64{}
	delivered := []int64{}
	invalidEmails := []int64{}
	opens := []int64{}
	processed := []int64{}
	requests := []int64{}
	spamReportDrops := []int64{}
	spamReports := []int64{}
	uniqueClicks := []int64{}
	uniqueOpens := []int64{}
	unsubscribeDrops := []int64{}
	unsubscribes := []int64{}

	//query.
	for _, res := range sgStats {
		t, _ := time.Parse("2006-01-02", res.Date)
		times = append(times, t)

    requests = append(requests, int64(res.Stats[0].Metrics.Requests))
		blocks = append(blocks, int64(res.Stats[0].Metrics.Blocks))
    bounceDrops = append(bounceDrops, int64(res.Stats[0].Metrics.BounceDrops))
    bounces = append(bounces, int64(res.Stats[0].Metrics.Bounces))
    clicks = append(clicks, int64(res.Stats[0].Metrics.Clicks))
    deferred = append(deferred, int64(res.Stats[0].Metrics.Deferred))
    delivered = append(delivered, int64(res.Stats[0].Metrics.Delivered))
    invalidEmails = append(invalidEmails, int64(res.Stats[0].Metrics.InvalidEmails))
    opens = append(opens, int64(res.Stats[0].Metrics.Opens))
    processed = append(processed, int64(res.Stats[0].Metrics.Processed))

    spamReportDrops = append(spamReportDrops, int64(res.Stats[0].Metrics.SpamReportDrops))
    spamReports = append(spamReports, int64(res.Stats[0].Metrics.SpamReports))
    uniqueClicks = append(uniqueClicks, int64(res.Stats[0].Metrics.UniqueClicks))
    uniqueOpens = append(uniqueOpens, int64(res.Stats[0].Metrics.UniqueOpens))
    unsubscribeDrops = append(unsubscribeDrops, int64(res.Stats[0].Metrics.UnsubscribeDrops))
    unsubscribes = append(unsubscribes, int64(res.Stats[0].Metrics.Unsubscribes))
	}

	// add the time dimension
	frame.Fields = append(frame.Fields,
		data.NewField("time", nil, times),
	)
  frame.Fields = append(frame.Fields,
    data.NewField("processed", nil,processed),
  )
  frame.Fields = append(frame.Fields,
    data.NewField("opens", nil,opens),
  )

	frame.Fields = append(frame.Fields,
		data.NewField("bounceDrops", nil,bounceDrops),
	)

  frame.Fields = append(frame.Fields,
    data.NewField("bounces", nil,bounces),
  )

  frame.Fields = append(frame.Fields,
    data.NewField("clicks", nil,clicks),
  )

  frame.Fields = append(frame.Fields,
    data.NewField("deferred", nil,deferred),
  )

  frame.Fields = append(frame.Fields,
    data.NewField("delivered", nil,delivered),
  )

  frame.Fields = append(frame.Fields,
    data.NewField("invalidEmails", nil,invalidEmails),
  )

  frame.Fields = append(frame.Fields,
    data.NewField("requests", nil,requests),
  )
  frame.Fields = append(frame.Fields,
    data.NewField("spamReportDrops", nil,spamReportDrops),
  )
  frame.Fields = append(frame.Fields,
    data.NewField("spamReports", nil,spamReports),
  )
  frame.Fields = append(frame.Fields,
    data.NewField("uniqueClicks", nil,uniqueClicks),
  )
  frame.Fields = append(frame.Fields,
    data.NewField("uniqueOpens", nil,uniqueOpens),
  )
  frame.Fields = append(frame.Fields,
    data.NewField("unsubscribeDrops", nil,unsubscribeDrops),
  )
  frame.Fields = append(frame.Fields,
    data.NewField("unsubscribes", nil,unsubscribes),
  )

	// add the frames to the response
	response.Frames = append(response.Frames, frame)

  vv = fmt.Sprintf("RESP calc %v", response.Frames)
  log.DefaultLogger.Info(vv)

  return &response, nil
}

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
func (td *SendgridDataSource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {

	rawJson, _ := req.PluginContext.DataSourceInstanceSettings.JSONData.MarshalJSON()

	v := fmt.Sprintf("ZZZZZZZZZZZ heath config %s", string(rawJson))
	log.DefaultLogger.Info(v)

	var status = backend.HealthStatusOk
	var message = "Data source is working"

	if rand.Int()%2 == 0 {
		status = backend.HealthStatusError
		message = "randomized error"
	}

	return &backend.CheckHealthResult{
		Status:  status,
		Message: message,
	}, nil
}

type instanceSettings struct {
	httpClient *http.Client
}

func newDataSourceInstance(setting backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {

	log.DefaultLogger.Info("YYYYYYYYYYYYYYYYYYYYYYYYYYYYYY")
	log.DefaultLogger.Info("settings", "settings", setting)

	fmt.Printf("settings %v\n", setting)
	return &instanceSettings{
		httpClient: &http.Client{},
	}, nil
}

func (s *instanceSettings) Dispose() {
	// Called before creatinga a new instance to allow plugin authors
	// to cleanup.
}
