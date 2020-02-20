package zenoss

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/golang/protobuf/jsonpb"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/outputs"
	serializer "github.com/influxdata/telegraf/plugins/serializers/json"
	"github.com/mitchellh/hashstructure"
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
#address = "api.zenoss.io:443"

#stdout_client = false
`
var processorConfigHeader = `

###############################################################################
#                            ZENOSS PROCESSOR PLUGINS                         #
###############################################################################

`

// Zenoss telgrap output type
type Zenoss struct {
	APIKey            string `toml:"api_key"`
	Address           string
	StdoutClient      bool
	client            zenossClient
	defaultMetaData   map[string]string
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
	defer logElapsed("Connect", time.Now())
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
	b := strings.Builder{}
	b.WriteString(sampleConfig)
	b.WriteString(processorConfigHeader)
	b.WriteString(vspherConfig)
	return b.String()
}

//Write to metrics Zenoss
func (z *Zenoss) Write(metrics []telegraf.Metric) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, zenossAPIKeyHeader, z.APIKey)

	db := newDataBuilder(z.defaultDimensions, z.defaultMetaData)
	for _, metric := range metrics {
		// mjson, _ := s.Serialize(metric)
		// log.Printf("write metric: %s", string(mjson))
		db.AddMetric(metric)
	}
	zMetrics, zModels := db.ZenossData()
	metricResult := make(chan error)
	go func() {
		defer logElapsed("send metrics", time.Now())
		defer close(metricResult)
		metricStatus, metricErr := z.client.PutMetrics(ctx, &zenoss.Metrics{
			DetailedResponse: true,
			Metrics:          zMetrics,
		})
		if metricErr != nil {
			log.Printf("ERROR: unable to send %d metrics: %v", len(zMetrics), metricErr)
		} else {
			if metricStatus.GetFailed() > 0 {
				log.Printf("ERROR: failed sending %d of %d metrics", metricStatus.GetFailed(), len(zMetrics))
			}
			log.Printf("sent %v metrics", metricStatus.GetSucceeded())
		}
		metricResult <- metricErr
	}()
	start := time.Now()
	modelStatus, err := z.client.PutModels(ctx, &zenoss.Models{
		DetailedResponse: true,
		Models:           zModels,
	})
	logElapsed("send models", start)
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
	// log.Println("put metrics...")
	// j, err := json.MarshalIndent(in, "", "  ")
	// if err != nil {
	// 	return nil, err
	// }
	// log.Println(string(j))
	return z.client.PutMetrics(ctx, in, opts...)
}

func (z *zClient) PutModels(ctx context.Context, in *zenoss.Models, opts ...grpc.CallOption) (*zenoss.ModelStatusResult, error) {
	// log.Println("put models...")
	// j, err := json.MarshalIndent(in, "", "  ")
	// if err != nil {
	// 	return nil, err
	// }
	// log.Println(string(j))
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
	m := &jsonpb.Marshaler{Indent: "  "}
	j, err := m.MarshalToString(in)
	if err != nil {
		return nil, err
	}
	log.Println(string(j))
	result := &zenoss.StatusResult{
		Succeeded: int32(len(in.Metrics)),
		Message:   "all good",
	}
	return result, nil
}

func (z *zClientTest) PutModels(ctx context.Context, in *zenoss.Models, opts ...grpc.CallOption) (*zenoss.ModelStatusResult, error) {
	m := &jsonpb.Marshaler{Indent: "  "}
	j, err := m.MarshalToString(in)
	if err != nil {
		return nil, err
	}
	log.Println(string(j))
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
	zDimOnly          bool
}

type modelBucket map[uint64]*zenoss.Model

func (mb modelBucket) Models() []*zenoss.Model {
	models := make([]*zenoss.Model, 0, len(mb))
	for _, v := range mb {
		models = append(models, v)
	}
	return models
}

type hashModel struct {
	Dimensions map[string]string
	MetaData   *structpb.Struct
}

func (mb modelBucket) Add(model *zenoss.Model) {
	hModel := hashModel{model.GetDimensions(), model.GetMetadataFields()}
	hash, err := hashstructure.Hash(hModel, nil)
	if err != nil {
		log.Printf("could not hash into bucket: ", err)
	}
	// log.Printf("Hash %d for %+v", hash, model)
	mb[hash] = model
}

func newDataBuilder(dimensions map[string]string, metadata map[string]string) *dataBuilder {
	return &dataBuilder{
		defaultDimensions: dimensions,
		defaultMetaData:   metadata,
		metrics:           []telegraf.Metric{},
		zDimOnly:          true,
	}
}

func (d *dataBuilder) AddMetric(metric telegraf.Metric) {
	d.metrics = append(d.metrics, metric)
}

var filteredMetrics = map[string]bool{}

func (d *dataBuilder) ZenossData() ([]*zenoss.Metric, []*zenoss.Model) {
	defer logElapsed("Databuilder ZenossData", time.Now())
	zMetrics := []*zenoss.Metric{}
	zModels := make(modelBucket)

	for _, metric := range d.metrics {
		timestamp := metric.Time().UnixNano() / 1e6

		hasZdim := false
		for k := range metric.Tags() {
			if strings.HasPrefix(k, "zdim") {
				hasZdim = true
				break
			}
		}
		s, err := serializer.NewSerializer(time.Millisecond)
		if err != nil {
			log.Printf("blam %s", err)
		}
		if _, ok := filteredMetrics[metric.Name()]; ok {
			// mjson, _ := s.Serialize(metric)
			// log.Printf("Dropping filtered metric %s", string(mjson))
			continue
		}
		if d.zDimOnly && !hasZdim {
			mjson, _ := s.Serialize(metric)
			log.Printf("Dropping metric withoug zdim: [%+v]\n", string(mjson))
			continue
		}
		// mjson, _ := s.Serialize(metric)
		// log.Printf("processing metric: [%+v]\n", string(mjson))

		zdim, zmeta := getZenossDimensionsAndMetaData(metric)
		//TODO: should sanitize tag keys and values
		dimensions := make(map[string]string, len(zdim)+len(d.defaultDimensions))
		for k, v := range d.defaultDimensions {
			dimensions[k] = v
		}
		for k, v := range zdim {
			dimensions[k] = v
		}
		metadata := make(map[string]string, len(zmeta)+len(d.defaultMetaData))
		for k, v := range d.defaultMetaData {
			metadata[k] = v
		}
		for k, v := range zmeta {
			metadata[k] = v
		}
		metadataPB := toStruct(metadata)
		for _, field := range metric.FieldList() {
			value, err := getFloat(field.Value)
			if err != nil {
				log.Printf("W! [outputs.zenoss] get float failed: %s", err)
				continue
			}

			name := fmt.Sprintf("%s.%s", metric.Name(), field.Key)
			zMetrics = append(zMetrics, &zenoss.Metric{
				Metric:         name,
				Timestamp:      timestamp,
				Dimensions:     dimensions,
				MetadataFields: toStruct(d.defaultMetaData),
				Value:          value,
			})

			zModels.Add(&zenoss.Model{
				Timestamp:      timestamp,
				Dimensions:     dimensions,
				MetadataFields: metadataPB,
			})

		}
	}
	return zMetrics, zModels.Models()
}

func getZenossDimensionsAndMetaData(m telegraf.Metric) (map[string]string, map[string]string) {
	// extract zdimensions if present
	zdim := map[string]string{}
	dim := map[string]string{}
	meta := map[string]string{}
	for k, v := range m.Tags() {
		// zdim_ prefix are explicitly set dimensions
		if strings.HasPrefix(k, "zdim_") {
			zdim[strings.TrimPrefix(k, "zdim_")] = v
		} else if k == "zname" {
			meta[zenossNameField] = v
		} else {
			dim[k] = v
		}
	}

	if len(zdim) > 0 {
		//we have zenoss dimensions identified, move all others to metadata
		for k, v := range dim {
			meta[k] = v
		}
	} else {
		zdim = dim
	}
	return zdim, meta
}
func toStruct(m map[string]string) *structpb.Struct {
	fields := make(map[string]*structpb.Value, len(m))
	for k, v := range m {
		if k == "impactToDimensions" {
			fields[k] = valueFromStringSlice([]string{v})
		} else {
			fields[k] = valueFromString(v)
		}
	}

	return &structpb.Struct{Fields: fields}
}

func valueFromStringSlice(ss []string) *structpb.Value {
	stringValues := make([]*structpb.Value, len(ss))
	for i, s := range ss {
		stringValues[i] = valueFromString(s)
	}
	return &structpb.Value{
		Kind: &structpb.Value_ListValue{
			ListValue: &structpb.ListValue{
				Values: stringValues,
			},
		},
	}
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

func logElapsed(msg string, start time.Time) {
	log.Printf("%s; Elapsed: %d ms", msg, time.Since(start).Milliseconds())
}

func init() {
	outputs.Add("zenoss", func() telegraf.Output {
		hostName, err := os.Hostname()
		if err != nil {
			hostName = "unknown"
		}
		hostName = fmt.Sprintf("zenoss.telegraf.%s", hostName)
		return &Zenoss{
			Address:           zenossAddress,
			defaultMetaData:   map[string]string{zenossSourceTypeField: zenossSourceType},
			defaultDimensions: map[string]string{zenossSourceField: hostName},
		}
	})
}
