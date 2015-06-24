package encoding_benchmark_test

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"unsafe"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/route-emitter/cfroutes"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	"github.com/encoding-benchmark"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type Encoder interface {
	Encode(interface{}) error
}

type Decoder interface {
	Decode(v interface{}) error
}

const iterations = 500

var desiredLRP = receptor.DesiredLRPCreateRequest{
	ProcessGuid: "some-guid",
	Domain:      "some-domain",
	RootFS:      "preloaded:some-rootfs",
	Instances:   5,
	Privileged:  true,

	LogGuid: "some-log-guid",

	Routes: cfroutes.CFRoutes{{Hostnames: []string{"some-host"}, Port: 8080}}.RoutingInfo(),
	Ports:  []uint16{8080},

	Setup: &models.DownloadAction{
		From: "http://some-url/v1/static/lrp.zip",
		To:   ".",
	},
	Action: &models.RunAction{
		Path: "bash",
		Args: []string{"server.sh"},
		Env:  []models.EnvironmentVariable{{"PORT", "8080"}},
	},
	Monitor: &models.RunAction{
		Path: "true",
	},
}

var protoDesired = encoding_benchmark.DesiredLRPCreateRequest{
	ProcessGuid: proto.String("some-guid"),
	Domain:      proto.String("some-domain"),
	RootFS:      proto.String("preloaded:some-rootfs"),
	Instances:   proto.Int32(5),
	Privileged:  proto.Bool(true),

	LogGuid: proto.String("some-log-guid"),

	Routes: []*encoding_benchmark.RouteEntry{{RouteType: proto.String("cf-route"), Data: []byte(`{"hostnames:["some-host"], port: 8080}`)}},
	Ports:  []uint32{8080},

	Setup: &encoding_benchmark.Action{
		DownloadAction: &encoding_benchmark.DownloadAction{
			From: proto.String("http://some-url/v1/static/lrp.zip"),
			To:   proto.String("."),
		}},
	Action: &encoding_benchmark.Action{RunAction: &encoding_benchmark.RunAction{
		Path: proto.String("bash"),
		Args: []string{"server.sh"},
		Env:  []*encoding_benchmark.EnvEntry{{Key: proto.String("PORT"), Value: proto.String("8080")}},
	}},
	Monitor: &encoding_benchmark.Action{RunAction: &encoding_benchmark.RunAction{
		Path: proto.String("true"),
	}},
}

var desiredLRPs []*receptor.DesiredLRPCreateRequest
var protoLRPs encoding_benchmark.DesiredLRPCreateRequests

var size = uint64(unsafe.Sizeof(desiredLRP))
var sizeInMB = float64(size) / (1024 * 1024)

var _ = Describe("Measurements", func() {
	var buff bytes.Buffer
	BeforeSuite(func() {
		for i := 0; i < 5000; i++ {
			newDesired := desiredLRP
			desiredLRPs = append(desiredLRPs, &newDesired)
			protoLRPs.Requests = append(protoLRPs.Requests, &protoDesired)
		}
	})

	AfterEach(func() {
		buff.Reset()
	})

	Measure("encoding/json", func(b Benchmarker) {
		encoder := json.NewEncoder(&buff)
		decoder := json.NewDecoder(&buff)

		measure(b, encoder, decoder)
	}, 10)

	Measure("encoding/gob", func(b Benchmarker) {
		gob.Register(&receptor.DesiredLRPCreateRequest{})
		gob.Register(&models.DownloadAction{})
		gob.Register(&models.RunAction{})
		encoder := gob.NewEncoder(&buff)
		decoder := gob.NewDecoder(&buff)

		measure(b, encoder, decoder)
	}, 10)

	Measure("encoding/protobuf", func(b Benchmarker) {
		measureProtobuf(b)
	}, 10)
})

func measure(b Benchmarker, encoder Encoder, decoder Decoder) {
	runtime := b.Time("overall", func() {
		encodingRuntime := b.Time("encoding", func() {
			for i := 0; i < iterations; i++ {
				err := encoder.Encode([]*receptor.DesiredLRPCreateRequest{&desiredLRP})
				Expect(err).NotTo(HaveOccurred())
			}
		})

		b.RecordValue("encoding-throughput (MB)", iterations*sizeInMB/encodingRuntime.Seconds())

		decodingRuntime := b.Time("decoding", func() {
			for i := 0; i < iterations; i++ {
				var decoded []*receptor.DesiredLRPCreateRequest
				err := decoder.Decode(&decoded)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		b.RecordValue("decoding-throughput (MB)", iterations*sizeInMB/decodingRuntime.Seconds())

		//Expect(*decoded[0]).To(Equal(desiredLRP))

		encodingRuntime = b.Time("encoding-high-volume", func() {
			err := encoder.Encode(desiredLRPs)
			Expect(err).NotTo(HaveOccurred())
		})

		b.RecordValue("encoding-high-volume-throughput (MB)", float64(len(desiredLRPs))*sizeInMB/encodingRuntime.Seconds())

		decodingRuntime = b.Time("decoding", func() {
			var decoded []*receptor.DesiredLRPCreateRequest
			err := decoder.Decode(&decoded)
			Expect(err).NotTo(HaveOccurred())
		})

		//Expect(len(decoded)).to(equal(len(desiredlrps)))
		//Expect(*decoded[0]).to(equal(desiredlrp))
		b.RecordValue("decoding-high-volume-throughput (MB)", float64(len(desiredLRPs))*sizeInMB/decodingRuntime.Seconds())
	})

	b.RecordValue("overall-throughput (MB)", 2*(iterations+float64(len(desiredLRPs)))*sizeInMB/runtime.Seconds())
}

func measureProtobuf(b Benchmarker) {
	bits, err := proto.Marshal(&protoDesired)
	Expect(err).NotTo(HaveOccurred())
	var foobar encoding_benchmark.DesiredLRPCreateRequest
	err = proto.Unmarshal(bits, &foobar)
	Expect(err).NotTo(HaveOccurred())
	Expect(foobar).To(Equal(protoDesired))

	runtime := b.Time("overall", func() {
		encodingRuntime := b.Time("encoding", func() {
			for i := 0; i < iterations; i++ {
				_, err := proto.Marshal(&protoDesired)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		b.RecordValue("encoding-throughput (MB)", iterations*sizeInMB/encodingRuntime.Seconds())

		bits, _ := proto.Marshal(&protoDesired)
		decodingRuntime := b.Time("decoding", func() {
			for i := 0; i < iterations; i++ {
				err := proto.Unmarshal(bits, &encoding_benchmark.DesiredLRPCreateRequest{})
				Expect(err).NotTo(HaveOccurred())
			}
		})

		b.RecordValue("decoding-throughput (MB)", iterations*sizeInMB/decodingRuntime.Seconds())

		//Expect(*decoded[0]).To(Equal(desiredLRP))

		encodingRuntime = b.Time("encoding-high-volume", func() {
			_, err := proto.Marshal(&protoLRPs)
			Expect(err).NotTo(HaveOccurred())
		})

		b.RecordValue("encoding-high-volume-throughput (MB)", float64(len(desiredLRPs))*sizeInMB/encodingRuntime.Seconds())

		bits, _ = proto.Marshal(&protoLRPs)
		decodingRuntime = b.Time("decoding", func() {
			err := proto.Unmarshal(bits, &encoding_benchmark.DesiredLRPCreateRequests{})
			Expect(err).NotTo(HaveOccurred())
		})

		//Expect(len(decoded)).to(equal(len(desiredlrps)))
		//Expect(*decoded[0]).to(equal(desiredlrp))
		b.RecordValue("decoding-high-volume-throughput (MB)", float64(len(desiredLRPs))*sizeInMB/decodingRuntime.Seconds())
	})

	b.RecordValue("overall-throughput (MB)", 2*(iterations+float64(len(desiredLRPs)))*sizeInMB/runtime.Seconds())
}
