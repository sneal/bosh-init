package tarball_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

func TestTarball(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tarball Suite")
}