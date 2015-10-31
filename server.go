// +build !appengine

package gottp

import (
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	traceback "gopkg.in/simversity/gotracer.v1"
	conf "gopkg.in/simversity/gottp.v3/conf"
)

var (
	SysInitChan = make(chan bool, 1)
	Tracer      traceback.Tracer
)

var (
	settings     conf.Config
	cleanupFuncs = []func(){}
)

func cleanAddr(addr string) {
	err := os.Remove(addr)
	if err != nil {
		log.Panic(err)
	}
}

func OnSysExit(cleanup func()) {
	cleanupFuncs = append(cleanupFuncs, cleanup)
}

func interrupt_cleanup(addr string) {
	sigchan := make(chan os.Signal, 10)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
	//NOTE: Capture every Signal right now.
	//signal.Notify(sigchan)

	s := <-sigchan
	log.Println("Exiting Program. Got Signal: ", s)

	if strings.Index(addr, "/") == 0 {
		// do last actions and wait for all write operations to end
		cleanAddr(addr)
	}

	if len(cleanupFuncs) > 0 {
		log.Println("Performing Cleanup routines")

		for i := range cleanupFuncs {
			cleanupFuncs[i]()
		}
	}

	shutdownWorker()

	os.Exit(0)
}

// parserCLL Pasres commandline arguments if any.
// Example: use -UNIX_SOCKET="127.0.0.1:8000" to change bind address
func parseCLI() {
	cfgPath, unixAddr := conf.CliArgs()
	settings.MakeConfig(cfgPath)

	if unixAddr != "" {
		settings.Gottp.Listen = unixAddr
	}
}

// MakeConfig take a Configurer and populates the settings in conf.Config
func MakeConfig(cfg conf.Configurer) {
	cfgPath, unixAddr := conf.CliArgs()
	cfg.MakeConfig(cfgPath)

	settings.Gottp = *cfg.GetGottpConfig()

	if unixAddr != "" {
		settings.Gottp.Listen = unixAddr
	}
}

func MakeExcpetionListener(settings *conf.Config) {
	Tracer = traceback.Tracer{
		Dummy:         settings.Gottp.EmailDummy,
		EmailHost:     settings.Gottp.EmailHost,
		EmailPort:     settings.Gottp.EmailPort,
		EmailPassword: settings.Gottp.EmailPassword,
		EmailUsername: settings.Gottp.EmailUsername,
		EmailSender:   settings.Gottp.EmailSender,
		EmailFrom:     settings.Gottp.EmailFrom,
		ErrorTo:       settings.Gottp.ErrorTo,
	}
}

// MakeServer takes a Configurer, and populates appropriate data structures
func MakeServer(cfg conf.Configurer) {
	MakeConfig(cfg)
	MakeExcpetionListener(&settings)

	makeServer()
}

// DefaultServer makes a server with default configuration,
// overriding commandline arguments
func DefaultServer() {
	parseCLI()
	makeServer()
}

// makeServer spawns server accoding to the configuration.
func makeServer() {
	addr := settings.Gottp.Listen

	SysInitChan <- true

	var serverError error
	if addr != "" {
		log.Println("Listening on " + addr)
	}

	go interrupt_cleanup(addr)

	if strings.Index(addr, "/") == 0 {
		listener, err := net.Listen("unix", addr)
		if err != nil {
			c, err := net.Dial("unix", addr)

			if c != nil {
				defer c.Close()
			}

			if err != nil {
				log.Println("The socket does not look like consumed. Erase ?")
				cleanAddr(addr)
				listener, err = net.Listen("unix", addr)
			} else {
				log.Fatal("Cannot start Server. Address is already in Use.", err)
				os.Exit(0)
			}
		}

		os.Chmod(addr, 0770)
		serverError = http.Serve(listener, nil)
	} else {
		serverError = http.ListenAndServe(addr, nil)
	}

	if serverError != nil {
		log.Println(serverError)
	}
}
