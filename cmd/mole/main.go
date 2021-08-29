package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/s00500/env_logger"
	"github.com/s00500/store"
	flag "github.com/spf13/pflag"
)

type Config struct {
	Configs []NamedConfig
}

type NamedConfig struct {
	Name        string
	Connections []ConnectionConfig
}

type ConnectionConfig struct {
	IsPublish       bool // else is Grab
	User            string
	ServerAndPort   string
	ServerReference string
	LocalIPandPort  string
	AutoReconnect   bool
}

// Flags: pflag --publish host:port --connect host:port --reconnect

//var publish *string = flag.String("publish", "localhost:8080", "the local port you want to publish")
//var connect *string = flag.String("connect", "server:1", "the local port you want to publish")
//var server *string = flag.String("server", "lbsfilm.at", "the molehil server")
//var serveruser *string = flag.String("user", "lukas", "the molehil server username")
//var serverport *int = flag.Int("port", 2222, "the molehill servers port")

var createNew *bool = flag.BoolP("create", "c", false, "create a new configuration using the wizard")
var listConfigs *bool = flag.BoolP("list", "l", false, "list available configs")

func main() {
	// Parse Flags
	flag.Parse()
	/*
		// Setup Logger
		logger := &logrus.Logger{
			Out:       colorable.NewColorableStdout(),
			Level:     logrus.InfoLevel,
			Formatter: &cliformatter.Formatter{},
		}

		debugConfig, _ := os.LookupEnv("LOG")
		log.ConfigureAllLoggers(logger, debugConfig)
	*/

	// Read Config
	dirname, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	config := new(Config)
	err = os.MkdirAll(filepath.Join(dirname, ".config", "mole"), 0755)
	log.Must(err)
	err = store.Load(filepath.Join(dirname, ".config", "mole", "config.yml"), &config)
	log.MustFatal(err)

	if *createNew {
		log.Error("wizard not implemented yet")
	}

	if *listConfigs {
		for id, cfg := range config.Configs {
			log.Info(fmt.Sprint(id+1), " ", cfg.Name)
		}
	}

	args := flag.Args()
	wg := new(sync.WaitGroup)
	for id, cName := range args {
		// get config
		var cfg *NamedConfig
		for _, c := range config.Configs {
			if c.Name == cName {
				cfg = &c
				break
			}
			if fmt.Sprint(id+1) == cName {
				cfg = &c
				break
			}
		}
		if cfg == nil {
			log.Warnln("Config", cName, "was not found")
			continue
		}

		log.Info("Connecting ", cfg.Name)
		connectConfig(cfg, wg)
	}

	wg.Wait()
}

func connectConfig(cfg *NamedConfig, wg *sync.WaitGroup) {
	for id, endpoint := range cfg.Connections {
		wg.Add(1)
		go connectEndpoint(&endpoint, fmt.Sprintf("%s:%d", cfg.Name, id), wg)
	}
}

func connectEndpoint(ep *ConnectionConfig, name string, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		args := []string{"-N"} // "-L", *publish + ":mole:1", *server, "-p", *serveruser + "@" + fmt.Sprint(*serverport)
		if ep.IsPublish {
			args = append(args, "-R")
		} else {
			args = append(args, "-L")
		}

		if ep.ServerReference == "" || ep.LocalIPandPort == "" || ep.ServerAndPort == "" {
			log.Warn("Missing Values on config ", name)
			break
		}

		// build command thing
		refParts := strings.Split(ep.ServerReference, ":")
		if len(refParts) > 2 {
			log.Warn("Invalid ServerReference on config ", name)
			break
		}

		if len(refParts) != 2 {
			refParts = append(refParts, "1")
		}

		localParts := strings.Split(ep.LocalIPandPort, ":")
		if len(localParts) > 2 {
			log.Warn("Invalid LocalIPandPort on config ", name)
			break
		}

		if len(localParts) != 2 {
			log.Warn("Missing port in LocalIPandPort on config ", name)
			break
		}

		args = append(args, fmt.Sprintf("%s:%s:%s:%s", localParts[0], localParts[1], refParts[0], refParts[1]))

		// build serverpath
		serverParts := strings.Split(ep.ServerAndPort, ":")
		if len(serverParts) > 2 {
			log.Warn("Invalid serverpath on config ", name)
			break
		}

		if len(serverParts) == 2 {
			args = append(args, "-p", serverParts[1])
		}

		args = append(args, fmt.Sprintf("%s@%s", ep.User, serverParts[0]))

		log.Info(args)

		cmd := exec.Command("ssh", args...)

		stdOut, err := cmd.StdoutPipe() // TODO: handle
		log.Should(err)
		logger := log.GetLoggerForPrefix(name)
		stdOutScanner := bufio.NewScanner(stdOut)
		go func() {
			for stdOutScanner.Scan() {
				logger.Info(stdOutScanner.Text())
			}
		}()

		stdErr, err := cmd.StderrPipe() // TODO: handle
		log.Should(err)
		stdErrScanner := bufio.NewScanner(stdErr)
		go func() {
			for stdErrScanner.Scan() {
				logger.Error(stdErrScanner.Text())
			}
		}()

		log.Should(cmd.Start())

		log.Should(cmd.Wait())
		if !ep.AutoReconnect {
			break
		}
		time.Sleep(time.Second)
		log.Info("Reconnecting ", name)
	}
}
