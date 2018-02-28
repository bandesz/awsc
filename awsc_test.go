package main_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opsidian/awsc/awsc"
	"github.com/pquerna/otp/totp"
)

var pwd string

func init() {
	var err error
	if pwd, err = os.Getwd(); err != nil {
		panic(err)
	}
}

func expectCmdToSucceed(out []byte, err error) {
	var stdErr string
	if exitErr, ok := err.(*exec.ExitError); ok {
		stdErr = string(exitErr.Stderr)
	}
	Expect(err).ToNot(
		HaveOccurred(),
		fmt.Sprintf("Stdout: %s\nStderr: %s\n", string(out), stdErr),
	)
}

var _ = Describe("AWS companion", func() {

	var (
		cacheDir   string
		executable string
	)

	BeforeSuite(func() {
		Expect(os.Getenv("AWS_ACCESS_KEY_ID")).ToNot(BeEmpty(), "AWS_ACCESS_KEY_ID must be defined")
		Expect(os.Getenv("AWS_SECRET_ACCESS_KEY")).ToNot(BeEmpty(), "AWS_SECRET_ACCESS_KEY must be defined")
		Expect(os.Getenv("TOTP_SECRET")).ToNot(BeEmpty(), "TOTP_SECRET must be defined")

		executable = fmt.Sprintf("%s/bin/awsc-%s.%s.amd64", pwd, awsc.Version, runtime.GOOS)

		var err error
		cacheDir, err = ioutil.TempDir("", "awsc-test-")
		Expect(err).ToNot(HaveOccurred())

		mfaTokenCode, err := totp.GenerateCode(os.Getenv("TOTP_SECRET"), time.Now())
		Expect(err).ToNot(HaveOccurred())

		authCmd := exec.Command(
			executable,
			"-c", cacheDir,
			"auth",
			"--token-code", mfaTokenCode,
		)
		authCmd.Env = []string{
			"AWS_PROFILE=awsc-test",
			fmt.Sprintf("AWS_REGION=%s", os.Getenv("AWS_REGION")),
			fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", os.Getenv("AWS_ACCESS_KEY_ID")),
			fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", os.Getenv("AWS_SECRET_ACCESS_KEY")),
		}
		out, err := authCmd.Output()
		expectCmdToSucceed(out, err)
	})

	AfterSuite(func() {
		os.RemoveAll(cacheDir)
	})

	Describe("the version command", func() {
		It("should return the version number", func() {
			out, err := exec.Command(executable, "version").Output()
			expectCmdToSucceed(out, err)
			Expect(string(out)).To(Equal(fmt.Sprintf("awsc %s\n", awsc.Version)))
		})
	})

	Describe("the auth command", func() {
		It("should create helper files", func() {
			Expect(filepath.Join(cacheDir, "awsc-test.json")).To(BeARegularFile())
			Expect(filepath.Join(cacheDir, "awsc-test.env")).To(BeARegularFile())
			Expect(filepath.Join(cacheDir, "awsc-test")).To(BeARegularFile())
		})
	})

	Describe("the auth wrapper script", func() {
		It("the env file should contain the AWS credentials", func() {
			out, err := exec.Command(
				"sh", "-c",
				fmt.Sprintf("source %s && env", filepath.Join(cacheDir, "awsc-test.env")),
			).Output()
			expectCmdToSucceed(out, err)
			Expect(string(out)).To(MatchRegexp("AWS_ACCESS_KEY_ID=.+\n"))
			Expect(string(out)).To(MatchRegexp("AWS_SECRET_ACCESS_KEY=.+\n"))
			Expect(string(out)).To(MatchRegexp("AWS_SESSION_TOKEN=.+\n"))
			Expect(string(out)).To(MatchRegexp("AWS_SECURITY_TOKEN=.+\n"))
		})

		It("should pass the AWS credentials as environment variables", func() {
			out, err := exec.Command("sh", "-c", filepath.Join(cacheDir, "awsc-test"), "env").Output()
			Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Output was: %s\n", out))
			Expect(string(out)).To(MatchRegexp("AWS_ACCESS_KEY_ID=.+\n"))
			Expect(string(out)).To(MatchRegexp("AWS_SECRET_ACCESS_KEY=.+\n"))
			Expect(string(out)).To(MatchRegexp("AWS_SESSION_TOKEN=.+\n"))
			Expect(string(out)).To(MatchRegexp("AWS_SECURITY_TOKEN=.+\n"))
		})

		It("the json file should contain the AWS session", func() {
			sessionJSON, err := ioutil.ReadFile(filepath.Join(cacheDir, "awsc-test.json"))
			Expect(err).ToNot(HaveOccurred())

			session := map[string]interface{}{}
			err = json.Unmarshal(sessionJSON, &session)
			Expect(err).ToNot(HaveOccurred())

			Expect(session).To(HaveKey("AccessKeyId"))
			Expect(session).To(HaveKey("SecretAccessKey"))
			Expect(session).To(HaveKey("SessionToken"))
		})
	})

})
