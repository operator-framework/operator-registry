package appregistry

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAppregistry(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Appregistry Suite")
}
