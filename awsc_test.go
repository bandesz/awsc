package main_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
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
		awsProfile string
	)

	BeforeSuite(func() {
		Expect(os.Getenv("TOTP_SECRET")).ToNot(BeEmpty(), "TOTP_SECRET must be defined")

		awsProfile = os.Getenv("AWS_PROFILE")
		if awsProfile == "" {
			awsProfile = "default"
		}

		var err error
		cacheDir, err = ioutil.TempDir("", "awsc-test-")
		Expect(err).ToNot(HaveOccurred())

		mfaTokenCode, err := totp.GenerateCode(os.Getenv("TOTP_SECRET"), time.Now())
		Expect(err).ToNot(HaveOccurred())

		out, err := exec.Command(
			"awsc",
			"-c", cacheDir,
			"auth",
			"--token-code", mfaTokenCode,
			"--aws-profile", awsProfile,
		).Output()
		expectCmdToSucceed(out, err)
	})

	AfterSuite(func() {
		os.RemoveAll(cacheDir)
	})

	Describe("the version command", func() {
		It("should return the version number", func() {
			out, err := exec.Command("awsc", "version").Output()
			expectCmdToSucceed(out, err)

			Expect(string(out)).To(Equal(fmt.Sprintf("awsc %s\n", awsc.Version)))
		})
	})

	Describe("the auth command", func() {

		Describe("the json file", func() {

			It("should be created", func() {
				Expect(fmt.Sprintf("%s/%s.json", cacheDir, awsProfile)).To(BeARegularFile())
			})

			It("should contain the AWS credentials", func() {
				sessionJSON, err := ioutil.ReadFile(fmt.Sprintf("%s/%s.json", cacheDir, awsProfile))
				Expect(err).ToNot(HaveOccurred())

				session := map[string]interface{}{}
				err = json.Unmarshal(sessionJSON, &session)
				Expect(err).ToNot(HaveOccurred())

				Expect(session).To(HaveKey("AccessKeyId"))
				Expect(session).To(HaveKey("SecretAccessKey"))
				Expect(session).To(HaveKey("SessionToken"))
			})

		})

		Describe("the env file", func() {

			It("should be created", func() {
				Expect(fmt.Sprintf("%s/%s.env", cacheDir, awsProfile)).To(BeARegularFile())
			})

			It("should expose the AWS credentials as env variables", func() {
				out, err := exec.Command(
					"sh", "-c",
					fmt.Sprintf(". %s && env", fmt.Sprintf("%s/%s.env", cacheDir, awsProfile)),
				).Output()
				expectCmdToSucceed(out, err)

				Expect(string(out)).To(MatchRegexp("AWS_ACCESS_KEY_ID=.+\n"))
				Expect(string(out)).To(MatchRegexp("AWS_SECRET_ACCESS_KEY=.+\n"))
				Expect(string(out)).To(MatchRegexp("AWS_SESSION_TOKEN=.+\n"))
				Expect(string(out)).To(MatchRegexp("AWS_SECURITY_TOKEN=.+\n"))
			})

		})

		Describe("the wrapper script", func() {
			It("should be created", func() {
				Expect(fmt.Sprintf("%s/%s", cacheDir, awsProfile)).To(BeARegularFile())
			})

			It("should pass the AWS credentials to the command", func() {
				out, err := exec.Command("sh", "-c", fmt.Sprintf("%s/%s", cacheDir, awsProfile), "env").Output()
				expectCmdToSucceed(out, err)

				Expect(string(out)).To(MatchRegexp("AWS_ACCESS_KEY_ID=.+\n"))
				Expect(string(out)).To(MatchRegexp("AWS_SECRET_ACCESS_KEY=.+\n"))
				Expect(string(out)).To(MatchRegexp("AWS_SESSION_TOKEN=.+\n"))
				Expect(string(out)).To(MatchRegexp("AWS_SECURITY_TOKEN=.+\n"))
			})

			It("should pass the CLI params and ENV vars correctly", func() {
				cmd := fmt.Sprintf(
					`ENV_VAR_1=x %s/%s ENV_VAR_2=y ./scripts/check_args.sh "a" "b c"`,
					cacheDir,
					awsProfile,
				)
				out, err := exec.Command("sh", "-c", cmd).Output()
				expectCmdToSucceed(out, err)

				Expect(string(out)).To(Equal("a,b c,x,y\n"))
			})
		})
	})

})
