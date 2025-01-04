package main

import (
	"bufio"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/lmittmann/tint"
)

var numCPU int
var debug bool
var logger *slog.Logger

func main() {
	var hostname, server string
	var doNet, doCpu, doDisk, doStdin bool
	var periodMs int
	var emitters arrayFlags

	usage := func() {
		basename := path.Base(os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), ""+
			"Usage: %s [OPTION]... \n"+
			"Options:\n",
			basename)
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), ""+
			"At least one of of the emitter options must be provided.\n")
		os.Exit(2)
	}
	flag.BoolVar(&debug, "debug", false, "log metrics to stdout")
	flag.StringVar(&server, "server", "", "server address")
	flag.StringVar(&hostname, "hostname", "", "this client's hostname")
	flag.BoolVar(&doNet, "net", false, "emit network stats")
	flag.BoolVar(&doCpu, "cpu", false, "emit cpu stats")
	flag.BoolVar(&doDisk, "disk", false, "emit disk stats")
	flag.BoolVar(&doStdin, "stdin", false, "forwards messages received on stdin")
	flag.IntVar(&periodMs, "period-ms", 100, "how frequently to compute metrics")
	flag.Var(&emitters, "emitter", "program emitting metrics, can be used multiple times")
	flag.Parse()

	args := flag.Args()
	if len(args) != 0 {
		usage()
	}
	if server == "" || hostname == "" ||
		!(doNet || doCpu || doDisk || doStdin || len(emitters) > 0) || periodMs <= 0 {
		usage()
	}

	logLevel := slog.LevelInfo
	if debug {
		logLevel = slog.LevelDebug
	}
	logger = slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		Level: logLevel,
		// TimeFormat: time.Kitchen,
		NoColor: true,
	}))

	messageChan := make(chan string)
	stopChan := make(chan bool)

	for _, emitter := range emitters {
		go func() {
			err := startEmitter(emitter, messageChan)
			if err != nil {
				logger.Error("startEmitter", "err", err)
				os.Exit(1)
			}
		}()
	}

	if doStdin {
		go func() {
			logger.Info("reading from stdin")
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Split(bufio.ScanLines)
			for scanner.Scan() {
				line := scanner.Text()
				logger.Debug("stdin", "line", line)
				messageChan <- line
			}
			if err := scanner.Err(); err != nil {
				logger.Error("stdin scanner", "err", err)
			}
		}()
	}

	numCPU = runtime.NumCPU()
	logger.Info("starting", "hostname", hostname)
	logger.Info("connecting", "server", server)

	for {
		c, err := net.Dial("tcp", server)
		if err != nil {
			logger.Error("Dial", "error", err)
			time.Sleep(2 * time.Second)
			continue
		}
		logger.Info("connected")
		c.Write([]byte(fmt.Sprintf("client %s\n", hostname)))
		routinesCount := 0
		if doNet {
			routinesCount++
			go emit(&NetStater{}, messageChan, stopChan, periodMs)
		}
		if doCpu {
			routinesCount++
			go emit(&ProcStater{}, messageChan, stopChan, periodMs)
		}
		if doDisk {
			routinesCount++
			go emit(&DiskStater{}, messageChan, stopChan, periodMs)
		}
		go func() {
			for line := range messageChan {
				line = strings.Replace(line, "%h", hostname, -1)
				_, err := c.Write([]byte(line))
				if err != nil {
					break
				}
			}
			for range routinesCount {
				stopChan <- true
			}
		}()
		for {
			msg, err := bufio.NewReader(c).ReadString('\n')
			if err != nil {
				logger.Error("disconnected", "err", err)
				break
			}
			logger.Info("server message", "msg", msg)
			time.Sleep(time.Second)
		}
	}
}

func startEmitter(joinedArgv string, metricsChan chan string) error {
	argv := strings.Split(joinedArgv, " ")
	logger.Debug("startEmitter", "argv", argv)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	err = cmd.Start()
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanLines)
	for {
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		metricsChan <- strings.TrimSpace(line) + "\n"
	}
	if err = scanner.Err(); err != nil {
		return err
	}
	err = cmd.Wait()
	if err != nil {
		return err
	}
	return nil
}

func emit(stater Stater, metricsChan chan string, stopChan chan bool,
	delayMs int) {

	stater.Init()
	ticker := time.NewTicker(time.Duration(delayMs) * time.Millisecond)
	for {
		msg := stater.Message()
		logger.Debug("emit", "msg", msg)
		metricsChan <- msg
		select {
		case <-stopChan:
			return
		case <-ticker.C:
		}
	}
}

// func retryableDialError(err error) bool {
// 	if opErr, ok := err.(*net.OpError); ok {
// 		if syscallErr, ok := opErr.Err.(*os.SyscallError); ok {
// 			if syscallErr.Err == syscall.ECONNREFUSED ||
// 				syscallErr.Err == syscall.ECONNRESET ||
// 				syscallErr.Err == syscall.ENETUNREACH {
// 				return true
// 			}
// 			logger.Info("retryableDialError", "errno", syscallErr.Err)
// 		}
// 	}
// 	return false
// }

// https://gist.github.com/gammazero/525fdffd273450edbcf8baadf849333a
type arrayFlags []string

func (a *arrayFlags) String() string {
	return strings.Join(*a, ", ")
}
func (a *arrayFlags) Set(value string) error {
	*a = append(*a, value)
	return nil
}
