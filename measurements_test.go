package encoding_benchmark_test

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"unsafe"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/route-emitter/cfroutes"
	"github.com/cloudfoundry-incubator/runtime-schema/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

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

var desiredLRPs []*receptor.DesiredLRPCreateRequest

var size = uint64(unsafe.Sizeof(desiredLRP))
var sizeInMB = float64(size) / (1024 * 1024)

var _ = Describe("Measurements", func() {
	var buff bytes.Buffer
	BeforeSuite(func() {
		for i := 0; i < 5000; i++ {
			newDesired := desiredLRP
			desiredLRPs = append(desiredLRPs, &newDesired)
		}
	})

	AfterEach(func() {
		buff.Reset()
	})

	Measure("encoding/json", func(b Benchmarker) {
		encoder := json.NewEncoder(&buff)
		decoder := json.NewDecoder(&buff)

		measure(b,
			func(v []*receptor.DesiredLRPCreateRequest) {
				err := encoder.Encode(v)
				Expect(err).NotTo(HaveOccurred())
			},
			func() []*receptor.DesiredLRPCreateRequest {
				var decoded []*receptor.DesiredLRPCreateRequest
				err := decoder.Decode(&decoded)
				Expect(err).NotTo(HaveOccurred())
				return decoded
			},
		)
	}, 10)

	Measure("encoding/gob", func(b Benchmarker) {
		gob.Register(&receptor.DesiredLRPCreateRequest{})
		gob.Register(&models.DownloadAction{})
		gob.Register(&models.RunAction{})
		encoder := gob.NewEncoder(&buff)
		decoder := gob.NewDecoder(&buff)

		measure(b,
			func(v []*receptor.DesiredLRPCreateRequest) {
				err := encoder.Encode(v)
				Expect(err).NotTo(HaveOccurred())
			},
			func() []*receptor.DesiredLRPCreateRequest {
				var decoded []*receptor.DesiredLRPCreateRequest
				err := decoder.Decode(&decoded)
				Expect(err).NotTo(HaveOccurred())
				return decoded
			},
		)
	}, 10)
})

func measure(b Benchmarker, encode func([]*receptor.DesiredLRPCreateRequest), decode func() []*receptor.DesiredLRPCreateRequest) {
	runtime := b.Time("overall", func() {
		encodingRuntime := b.Time("encoding", func() {
			for i := 0; i < iterations; i++ {
				encode([]*receptor.DesiredLRPCreateRequest{&desiredLRP})
			}
		})

		b.RecordValue("encoding-throughput (MB)", iterations*sizeInMB/encodingRuntime.Seconds())

		var decoded []*receptor.DesiredLRPCreateRequest
		decodingRuntime := b.Time("decoding", func() {
			for i := 0; i < iterations; i++ {
				decoded = decode()
			}
		})

		b.RecordValue("decoding-throughput (MB)", iterations*sizeInMB/decodingRuntime.Seconds())

		Expect(*decoded[0]).To(Equal(desiredLRP))

		encodingRuntime = b.Time("encoding-high-volume", func() {
			encode(desiredLRPs)
		})

		b.RecordValue("encoding-high-volume-throughput (MB)", float64(len(desiredLRPs))*sizeInMB/encodingRuntime.Seconds())

		decodingRuntime = b.Time("decoding", func() {
			decoded = decode()
		})

		Expect(len(decoded)).To(Equal(len(desiredLRPs)))
		Expect(*decoded[0]).To(Equal(desiredLRP))
		b.RecordValue("decoding-high-volume-throughput (MB)", float64(len(desiredLRPs))*sizeInMB/decodingRuntime.Seconds())
	})

	b.RecordValue("overall-throughput (MB)", 2*(iterations+float64(len(desiredLRPs)))*sizeInMB/runtime.Seconds())
}
