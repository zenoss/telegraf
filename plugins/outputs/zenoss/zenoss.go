package zenoss

import (
	"context"
	"sort"
	"hash/fnv"
	"os"
	"crypto/tls"
	"encoding/json"
	"encoding/binary"
	"fmt"
	"log"
	"time"

	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/outputs"
	zenoss "github.com/zenoss/zenoss-protobufs/go/cloud/data_receiver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

const (
	zenossAPIKeyHeader = "zenoss-api-key"
	zenossAddress      = "api.zenoss.io:443"

	zenossSourceTypeField  = "source-type"
	zenossNameField        = "name"
	zenossDescriptionField = "description"
	zenossSourceField      = "source"

	zenossSourceType = "telegraf.output.zenoss"
)

var sampleConfig = `
## Zenoss API Key
api_key = "secrete-key" # required

#zenoss grpc endpoint
#address = https://api.zenoss.io/

#stdout_client = false
`

// Zenoss telgrap output type
type Zenoss struct {
	APIKey          string `toml:"api_key"`
	Address         string
	StdoutClient    bool
	client          zenossClient
	defaultMetaData map[string]string
	defaultDimensions map[string]string
}

type zenossClient interface {
	Connect(string) error
	Close() error
	PutMetrics(ctx context.Context, in *zenoss.Metrics, opts ...grpc.CallOption) (*zenoss.StatusResult, error)
	PutModels(ctx context.Context, in *zenoss.Models, opts ...grpc.CallOption) (*zenoss.ModelStatusResult, error)
}

var _ telegraf.Output = &Zenoss{}

//Connect to the output
func (z *Zenoss) Connect() error {
	if z.StdoutClient {
		z.client = &zClientTest{}
	} else {
		z.client = &zClient{}
	}
	return z.client.Connect(z.Address)
}

//Close any connections
func (z *Zenoss) Close() error {
	return z.client.Close()
}

//Description of the output
func (*Zenoss) Description() string {
	return "Configuration for Zenoss API to send metrics to "
}

//SampleConfig for the output
func (*Zenoss) SampleConfig() string {
	return sampleConfig
}

//Write to metrics Zenoss
func (z *Zenoss) Write(metrics []telegraf.Metric) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, zenossAPIKeyHeader, z.APIKey)

	db := newDataBuilder(z.defaultDimensions, z.defaultMetaData)
	for _, metric := range metrics {
		db.AddMetric(metric)
	}
	zMetrics, zModels := db.ZenossData()
	metricResult := make(chan error)
	go func() {
		defer close(metricResult)
		metricStatus, err := z.client.PutMetrics(ctx, &zenoss.Metrics{
			DetailedResponse: true,
			Metrics:          zMetrics,
		})
		if err != nil {
			log.Printf("ERROR: unable to send %d metrics: %v", len(zMetrics), err)
		} else {
			if metricStatus.GetFailed() > 0 {
				log.Printf("ERROR: failed sending %d of %d metrics", metricStatus.GetFailed(), len(zMetrics))
			}
			log.Printf("sent %v metrics", metricStatus.GetSucceeded())
		}
		metricResult <- err
	}()
	modelStatus, err := z.client.PutModels(ctx, &zenoss.Models{
		DetailedResponse: true,
		Models:           zModels,
	})
	if err != nil {
		log.Printf("ERROR: unable to send %d models: %v", len(zModels), err)
	} else {
		if modelStatus.GetFailed() > 0 {
			log.Printf("ERROR: failed sending %d of %d models", modelStatus.GetFailed(), len(zModels))
		}
		log.Printf("sent %d models", modelStatus.GetSucceeded())
	}
	metricErr := <-metricResult
	if metricErr != nil && err != nil {
		return fmt.Errorf("%s; %s", metricErr, err)
	}
	if metricErr != nil {
		return metricErr
	}
	return err
}

type zClient struct {
	conn   *grpc.ClientConn
	client zenoss.DataReceiverServiceClient
}

func (z *zClient) Connect(address string) error {
	//TODO: define and handle proxy options

	tlsOpt := grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{}))
	conn, err := grpc.Dial(address, tlsOpt)
	if err != nil {
		return err
	}
	z.conn = conn
	z.client = zenoss.NewDataReceiverServiceClient(conn)

	return nil
}
func (z *zClient) Close() error {
	return z.conn.Close()
}
func (z *zClient) PutMetrics(ctx context.Context, in *zenoss.Metrics, opts ...grpc.CallOption) (*zenoss.StatusResult, error) {
	return z.client.PutMetrics(ctx, in, opts...)
}

func (z *zClient) PutModels(ctx context.Context, in *zenoss.Models, opts ...grpc.CallOption) (*zenoss.ModelStatusResult, error) {
	return z.client.PutModels(ctx, in, opts...)
}

var _ zenossClient = &zClient{}

type zClientTest struct {
}

func (z *zClientTest) Connect(address string) error {
	return nil
}
func (z *zClientTest) Close() error {
	return nil
}
func (z *zClientTest) PutMetrics(ctx context.Context, in *zenoss.Metrics, opts ...grpc.CallOption) (*zenoss.StatusResult, error) {
	j, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return nil, err
	}
	result := &zenoss.StatusResult{
		Succeeded: int32(len(in.Metrics)),
		Message:   "all good",
	}
	return result, nil
}

func (z *zClientTest) PutModels(ctx context.Context, in *zenoss.Models, opts ...grpc.CallOption) (*zenoss.ModelStatusResult, error) {
	j, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return nil, err
	}
	result := &zenoss.ModelStatusResult{
		Succeeded: int32(len(in.Models)),
		Message:   "all good",
	}
	return result, nil
}

type dataBuilder struct {
	defaultDimensions map[string]string
	defaultMetaData   map[string]string
	metrics           []telegraf.Metric
}

type modelBucket map[uint64]*zenoss.Model

func (mb modelBucket) Models()[]*zenoss.Model{
	models := []*zenoss.Model{}
	for _, v := range mb{
		models = append(models, v)
	}
	return models
}

func (mb modelBucket) Add(model *zenoss.Model){
	h:= fnv.New64a()
	b := make([]byte,8)
	binary.LittleEndian.PutUint64(b, uint64(model.Timestamp))
	h.Write(b)
	h.Write( []byte("\n"))	
	dims := model.GetDimensions()
	keys := []string{}
	for key := range dims{
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys{
		h.Write( []byte(key))
		h.Write( []byte("\n"))
		v := dims[key]
		h.Write( []byte(v))
		h.Write( []byte("\n"))
	}
	mb[h.Sum64()] = model	
}


func newDataBuilder(dimensions map[string]string, metadata map[string]string) *dataBuilder {
	return &dataBuilder{
		defaultDimensions: dimensions,
		defaultMetaData:   metadata,
		metrics:           []telegraf.Metric{},
	}
}

func (d *dataBuilder) AddMetric(metric telegraf.Metric) {
	d.metrics = append(d.metrics, metric)
}

func (d *dataBuilder) ZenossData() ([]*zenoss.Metric, []*zenoss.Model) {
	zMetrics := []*zenoss.Metric{}
	zModels := make(modelBucket)

	for _, metric := range d.metrics {
		timestamp := metric.Time().UnixNano() / 1e6
		for _, field := range metric.FieldList() {
			value, err := getFloat(field.Value)
			if err != nil {
				log.Printf("W! [outputs.zenoss] get float failed: %s", err)
				continue
			}
			//TODO: should sanitize tag keys and values
			dimensions := make(map[string]string, len(metric.Tags()))
			for k, v := range d.defaultDimensions {
				dimensions[k] = v
			}
			for k, v := range metric.Tags() {
				dimensions[k] = v
			}

			metadata := make(map[string]string)
			for k, v := range d.defaultMetaData {
				metadata[k] = v
			}
			metadataPB := toStruct(metadata)
			name := fmt.Sprintf("%s.%s", metric.Name(), field.Key)
			zMetrics = append(zMetrics, &zenoss.Metric{
				Metric:         name,
				Timestamp:      timestamp,
				Dimensions:     dimensions,
				MetadataFields: metadataPB,
				Value:          value,
			})

			metadata[zenossNameField] = name
			zModels.Add( &zenoss.Model{
				Timestamp:      timestamp,
				Dimensions:     dimensions,
				MetadataFields: metadataPB,
			})

		}
	}
	return zMetrics, zModels.Models()
}

func toStruct(m map[string]string) *structpb.Struct {
	fields := map[string]*structpb.Value{}

	for k, v := range m {
		fields[k] = valueFromString(v)
	}

	return &structpb.Struct{Fields: fields}
}

func valueFromString(s string) *structpb.Value {
	return &structpb.Value{
		Kind: &structpb.Value_StringValue{
			StringValue: s,
		},
	}
}

func getFloat(val interface{}) (float64, error) {
	switch v := val.(type) {
	case int64:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case float64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("unsupported metric value: [%s] of type [%T]", v, v)
	}
}
func init() {
	outputs.Add("zenoss", func() telegraf.Output {
		hostName, err := os.Hostname()
		if err != nil{
			hostName = "unknown"
		}
		hostName = fmt.Sprintf("zenoss.telegraf.%s", hostName)
		return &Zenoss{
			Address:         zenossAddress,
			defaultMetaData: map[string]string{zenossSourceTypeField: zenossSourceType},
			defaultDimensions: map[string]string{zenossSourceField: hostName},
		}
	})
}
