package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/manifoldco/promptui"
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
		newCfg := new(NamedConfig)

		// get new configs name
		promptString := promptui.Prompt{
			Label:    "Enter a name for the new configuration",
			Validate: validateEmpty,
		}

		result, err := promptString.Run()
		log.MustFatal(log.Wrap(err, "input error"))
		newCfg.Name = result

	createEndpoint:

		newEndpoint := new(ConnectionConfig)

		prompt := promptui.Select{
			Label: "Are you trying to publish a port to the server ? or do you want to access a port on the server with your local machine ?",
			Items: []string{"Publish to server", "Access with local machine"},
		}
		_, result, err = prompt.Run()
		log.MustFatal(log.Wrap(err, "input error"))

		if result == "Publish to server" {
			newEndpoint.IsPublish = true
		}

		// get local
		promptString = promptui.Prompt{
			Label:    "Enter the local interface and port to use",
			Validate: validateLocal,
		}

		result, err = promptString.Run()
		log.MustFatal(log.Wrap(err, "input error"))
		newEndpoint.LocalIPandPort = result

		// get server reference
		promptString = promptui.Prompt{
			Label:    "Enter the server reference",
			Validate: validateServerRef,
		}

		result, err = promptString.Run()
		log.MustFatal(log.Wrap(err, "input error"))
		newEndpoint.ServerReference = result

		// get autoreconnect
		prompt = promptui.Select{
			Label: "Should this endpoint automatically reconnect ?",
			Items: []string{"Yes", "No"},
		}
		_, result, err = prompt.Run()
		log.MustFatal(log.Wrap(err, "input error"))
		newEndpoint.AutoReconnect = result == "Yes"

		// get server
		defaultServer := "molehill.example.com:2222"
		if len(newCfg.Connections) > 0 {
			defaultServer = newCfg.Connections[0].ServerAndPort
		} else if len(config.Configs) > 0 && len(config.Configs[0].Connections) > 0 {
			defaultServer = config.Configs[0].Connections[0].ServerAndPort
		}
		promptString = promptui.Prompt{
			Label:    "Enter the molehill server address",
			Default:  defaultServer,
			Validate: validateServerRef, // same as for the reference
		}
		result, err = promptString.Run()
		log.MustFatal(log.Wrap(err, "input error"))
		newEndpoint.ServerAndPort = result

		userName, err := user.Current()
		log.MustFatal(err)

		// get server user
		defaultUser := userName.Name
		if len(newCfg.Connections) > 0 {
			defaultUser = newCfg.Connections[0].User
		} else if len(config.Configs) > 0 && len(config.Configs[0].Connections) > 0 {
			defaultUser = config.Configs[0].Connections[0].User
		}
		promptString = promptui.Prompt{
			Label:    "Enter the molehill server user",
			Default:  defaultUser,
			Validate: validateEmpty,
		}
		result, err = promptString.Run()
		log.MustFatal(log.Wrap(err, "input error"))
		newEndpoint.User = result

		newCfg.Connections = append(newCfg.Connections, *newEndpoint)

		// more ?
		prompt = promptui.Select{
			Label: "Do you want to add more endpoints to " + newCfg.Name + " ?",
			Items: []string{"No", "Yes"},
		}
		_, result, err = prompt.Run()
		log.MustFatal(log.Wrap(err, "input error"))
		if result == "Yes" {
			goto createEndpoint
		}

		config.Configs = append(config.Configs, *newCfg)

		err = store.Save(filepath.Join(dirname, ".config", "mole", "config.yml"), config)
		log.MustFatal(err)
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
			if !stringIsPort(ep.LocalIPandPort) {
				log.Warn("Invalid LocalIPandPort on config ", name)
				break
			}
			args = append(args, fmt.Sprintf("%s:%s:%s", ep.LocalIPandPort, refParts[0], refParts[1]))
		} else {
			args = append(args, fmt.Sprintf("%s:%s:%s:%s", localParts[0], localParts[1], refParts[0], refParts[1]))
		}

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
		if log.Should(err) {
			continue
		}
		logger := log.GetLoggerForPrefix(name)
		stdOutScanner := bufio.NewScanner(stdOut)
		go func() {
			for stdOutScanner.Scan() {
				logger.Info(stdOutScanner.Text())
			}
		}()

		stdErr, err := cmd.StderrPipe() // TODO: handle
		if log.Should(err) {
			continue
		}
		stdErrScanner := bufio.NewScanner(stdErr)
		go func() {
			for stdErrScanner.Scan() {
				logger.Error(stdErrScanner.Text())
			}
		}()

		if log.Should(cmd.Start()) {
			time.Sleep(time.Second)
			continue
		}

		log.Should(cmd.Wait())
		if !ep.AutoReconnect {
			break
		}
		time.Sleep(time.Second)
		log.Info("Reconnecting ", name)
	}
}

func validateEmpty(input string) error {
	if input == "" {
		return fmt.Errorf("empty string")
	}
	return nil
}

func validateLocal(input string) error {
	// could be host, and port or just port

	if input == "" {
		return fmt.Errorf("empty string")
	}

	if strings.ContainsAny(input, " /") {
		return fmt.Errorf("invalid characters")
	}

	if strings.Contains(input, ":") {
		parts := strings.Split(input, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid port")
		}
		i, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid port")
		}
		if i <= 0 || i > 65535 {
			return fmt.Errorf("invalid port range")
		}
	} else {
		// now it must be a valid port
		if !stringIsPort(input) {
			return fmt.Errorf("invalid port")
		}
	}

	return nil
}

func validateServerRef(input string) error {
	// could be host, and port or just host

	if input == "" {
		return fmt.Errorf("empty string")
	}

	if strings.ContainsAny(input, " /") {
		return fmt.Errorf("invalid characters")
	}

	if strings.Contains(input, ":") {
		parts := strings.Split(input, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid port")
		}
		if !stringIsPort(parts[1]) {
			return fmt.Errorf("invalid port")
		}
	}

	return nil
}

func stringIsPort(input string) bool {
	i, err := strconv.ParseInt(input, 10, 64)
	if err != nil {
		return false
	}
	if i <= 0 || i > 65535 {
		return false
	}
	return true
}
