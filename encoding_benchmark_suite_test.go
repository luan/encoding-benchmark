package encoding_benchmark_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestEncodingBenchmark(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EncodingBenchmark Suite")
}
