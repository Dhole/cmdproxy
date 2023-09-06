package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"os"
	"os/exec"
	"time"
)

type DataType int

const (
	DataTypeStdin  DataType = 0
	DataTypeStdout DataType = 1
	DataTypeStderr DataType = 2
)

type LogData struct {
	Type DataType
	Data []byte
}

// Run a command and log all stdin, stdout and stderr to a log file, while
// exposing stdin, stdout and stderr transparently.

func writeLn(writer io.Writer, data []byte) error {
	_, err := writer.Write(data)
	if err != nil {
		return err
	}
	_, err = writer.Write([]byte("\n"))
	return err
}

func main() {
	command := flag.String("cmd", "", "Command to run under the proxy")
	logPath := flag.String("logPath", "", "Path to log file")
	flag.Parse()

	if *command == "" || *logPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	logFile, err := os.Create(*logPath)
	if err != nil {
		log.Fatal(err)
	}

	cmd := exec.Command("sh", "-c", *command)
	cmdStdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	cmdStdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	cmdStderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}

	stdinScanner := bufio.NewScanner(os.Stdin)
	cmdStdoutScanner := bufio.NewScanner(cmdStdout)
	cmdStderrScanner := bufio.NewScanner(cmdStderr)
	logCh := make(chan LogData)
	go func() {
		for stdinScanner.Scan() {
			data := stdinScanner.Bytes()
			logCh <- LogData{DataTypeStdin, data}
			err := writeLn(cmdStdin, data)
			if err != nil {
				log.Fatal(err)
			}
		}
		cmdStdin.Close()
	}()
	go func() {
		for cmdStdoutScanner.Scan() {
			data := cmdStdoutScanner.Bytes()
			logCh <- LogData{DataTypeStdout, data}
			err := writeLn(os.Stdout, data)
			if err != nil {
				log.Fatal(err)
			}
		}
	}()
	go func() {
		for cmdStderrScanner.Scan() {
			data := cmdStderrScanner.Bytes()
			logCh <- LogData{DataTypeStderr, data}
			err := writeLn(os.Stderr, data)
			if err != nil {
				log.Fatal(err)
			}
		}
	}()
	cmd.Start()

	for logData := range logCh {
		now := time.Now()
		_, err := logFile.WriteString(now.Format(time.StampMilli))
		if err != nil {
			log.Fatal(err)
		}
		switch logData.Type {
		case DataTypeStdin:
			_, err = logFile.WriteString(" > ")
		case DataTypeStdout:
			_, err = logFile.WriteString(" < ")
		case DataTypeStderr:
			_, err = logFile.WriteString(" E ")
		}
		if err != nil {
			log.Fatal(err)
		}
		logFile.Write(logData.Data)
		logFile.Write([]byte("\n"))
	}
	cmd.Wait()
}
