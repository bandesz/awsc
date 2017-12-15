package main

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/opsidian/awsc/cli/command"
)

func main() {
	signalChan := make(chan os.Signal)
	signal.Notify(signalChan, syscall.SIGINT)

	terminalState, _ := terminal.GetState(int(syscall.Stdin))

	go func() {
		<-signalChan
		// make sure we restore the terminal state
		if terminalState != nil {
			terminal.Restore(int(syscall.Stdin), terminalState)
		}
		os.Exit(130)
	}()

	command.Execute()
}
