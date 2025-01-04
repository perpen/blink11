package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"syscall"
	"time"
)

var numCPU int
var debug bool

func main() {
	var hostname, server string
	var doNet, doCpu, doStdin bool
	var periodMs int
	var scripts arrayFlags

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
	flag.BoolVar(&doStdin, "stdin", false, "emit stats from stdin")
	flag.IntVar(&periodMs, "period-ms", 250, "how frequently to compute metrics")
	flag.Var(&scripts, "script", "script emitting metrics, can be used several times")
	flag.Parse()

	args := flag.Args()
	if len(args) != 0 {
		usage()
	}
	if server == "" || hostname == "" || !(doNet || doCpu || len(scripts) > 0) || periodMs <= 0 {
		usage()
	}

	metricsChan := make(chan string, 0)
	stopChan := make(chan bool, 0)
	for _, script := range scripts {
		go func() {
			err := runScript(script, metricsChan)
			if err != nil {
				log.Fatalf("runScript: %v", err)
			}
		}()
	}

	numCPU = runtime.NumCPU()
	fmt.Printf("hostname: %v\n", hostname)
	fmt.Printf("connecting to %s\n", server)

	for {
		c, err := net.Dial("tcp", server)
		if err != nil {
			if !retryableDialError(err) {
				log.Fatalf("%v\n", err)
			}
			time.Sleep(2 * time.Second)
			continue
		}
		fmt.Printf("connected to %s\n", server)
		c.Write([]byte(fmt.Sprintf("client %s\n", hostname)))
		routines := 0
		if doNet {
			routines++
			go emit(netStatsMsg, metricsChan, stopChan, periodMs)
		}
		if doCpu {
			routines++
			go emit(procStatsMsg, metricsChan, stopChan, periodMs)
		}
		go func() {
			for line := range metricsChan {
				line = strings.Replace(line, "%h", hostname, -1) + "\n"
				if debug {
					fmt.Printf("got metric: %s", line)
				}
				_, err := c.Write([]byte(line))
				if err != nil {
					break
				}
			}
			for i := 0; i < routines; i++ {
				stopChan <- true
			}
		}()
		for {
			msg, err := bufio.NewReader(c).ReadString('\n')
			if err != nil {
				fmt.Printf("disconnected\n")
				break
			}
			fmt.Printf("message from server: %v", msg)
			time.Sleep(time.Second)
		}
	}
}

func runScript(joinedArgv string, metricsChan chan string) error {
	argv := strings.Split(joinedArgv, " ")
	if debug {
		fmt.Printf("runScript: %v\n", argv)
	}
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
		metricsChan <- strings.TrimSpace(line)
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

func emit(compute func() string, metricsChan chan string, stopChan chan bool,
	delayMs int) {

	ticker := time.NewTicker(time.Duration(delayMs) * time.Millisecond)
	for {
		msg := compute()
		select {
		case metricsChan <- msg:
		default:
		}
		select {
		case <-stopChan:
			return
		case <-ticker.C:
		}
	}
}

func retryableDialError(err error) bool {
	if opErr, ok := err.(*net.OpError); ok {
		if syscallErr, ok := opErr.Err.(*os.SyscallError); ok {
			if syscallErr.Err == syscall.ECONNREFUSED {
				//fmt.Printf("connection refused\n")
				return true
			}
		}
	}
	return false
}

// https://gist.github.com/gammazero/525fdffd273450edbcf8baadf849333a
type arrayFlags []string

func (a *arrayFlags) String() string {
	return strings.Join(*a, ", ")
}
func (a *arrayFlags) Set(value string) error {
	*a = append(*a, value)
	return nil
}
