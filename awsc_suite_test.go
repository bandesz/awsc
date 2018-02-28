package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAwsc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Awsc Suite")
}
