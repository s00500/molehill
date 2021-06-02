package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"molehill/filehandlers"
	"net"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"
	log "github.com/s00500/env_logger"
	"github.com/s00500/store"

	"github.com/gliderlabs/ssh"
)

//go:generate sh injectGitVars.sh

type Config struct {
	Runaddress string
	Users      []UserConfig
}

type UserConfig struct {
	Name            string
	Password        string
	PublicKey       string
	AllowedBinds    []string
	AllowedConnects []string
}

var configMu sync.RWMutex
var config Config = Config{
	Runaddress: ":2222",
	Users: []UserConfig{
		{
			Name:            "lukas",
			Password:        "lukas", // empty means autogenerate ? not sure
			PublicKey:       "AAAAB3NzaC1yc2EAAAADAQABAAABAQDLdQry15RLpQ7/uPHFb79ToEs7fLy27J1jgNHTdrGn9HPRSS0Xcup34x6gdX/UG+APO2n87Xz6fOwLEd7ORCrITlUy0sh26lOFhGO+hRcQHrh2bmF6c4CIO8VH1AZc/EN6x9BTQJS3ridLBggspomLVHXwCmKhmpvUT8EynSbm8mYS1CR0XNu1T1yVdYQ0jYPUA5er8OxZNuOhMuO4iQEEplJoZv8zyKy9QW1aGREOEgQK9l0iLaGXqSlEqgcBLmdJKSTZ5OaM+kF0wcGylRRTXntJM/N0xH3U0pYaiqM6isAwKHVuXcu/IMI4XboVUVZlbcqoPde7t5xHUsLiIYGb",
			AllowedBinds:    []string{"127.0.0.1:1", "localhost:1"},
			AllowedConnects: []string{"127.0.0.1:1", "localhost:1"},
		},
		{
			Name:            "andrii",
			PublicKey:       "AAAAB3NzaC1yc2EAAAADAQABAAABgQC3nMQPNE6pXBGa8O2LBMma1FFEMgmm6VXVRUeeKNGDZF3XM6e0sP/Q0NmhYDX+JoZ4Eswyi3pyF1LPjA1Z6rcvFms+ifPNJfKUoo7XewRWOX8kQAsOJKFfwBatkqT+8whau6YnsQzFoFMt/5aeIqc6iMM+63Lxwo9uDDehMesPIb576je40SVrdMn7vIZy88s0Jwwfy91jvULkCygf4E1KXIfyIeLIKLKUPypXleXGvUwclnqdrQmyPWq1cUXx1vU4iNGe0CfTjXOrsvquNTQV8lJbn17fQKax5a6TFgCIfPbgy+W4G9yo5vZOlLHA5lIvRoNf0hNqSPP6f9wMp4R4WK1ecDQuLU1kLfAcZA6T5tRUCyBblaiMPrDcH2dBjHFjysJ+vOCFSPDWjHp6Sj/Gs66bbEg6AzXEiLEXqDqjlgaE3V2V3B5tfFiu6gPmmgGhAcWrYTQoNDrPRfQb5ZerVGyYlvrY06BfdwTyMahKNqA9P0EJ1fb7L4+C/yNtWok=",
			AllowedBinds:    []string{"127.0.0.1:1", "localhost:1"},
			AllowedConnects: []string{"127.0.0.1:1", "localhost:1"},
		},
	},
}

const configPath string = ""

func main() {
	log.Infof("Starting ssh server version %s (%s)", gitTag, gitRevision)
	os.RemoveAll("sockets")
	log.MustFatal(os.MkdirAll("sockets", 0755))

	// Load config
	configMu.Lock()
	store.Load("config.yml", &config)
	configMu.Unlock()

	// watch config as well

	log.Should(startWatcher())

	forwardHandler := &filehandlers.ForwardedTCPToFileHandler{}

	server := ssh.Server{
		Addr: config.Runaddress,
		PublicKeyHandler: ssh.PublicKeyHandler(func(ctx ssh.Context, key ssh.PublicKey) bool {
			configMu.RLock()
			for _, user := range config.Users {
				if user.Name == ctx.User() {
					if user.PublicKey == "" {
						continue
					}

					decoded, err := base64.StdEncoding.DecodeString(user.PublicKey)
					if log.Should(log.Wrap(err, "on parsing public key from user %s", user.Name)) {
						continue
					}

					userKey, err := ssh.ParsePublicKey(decoded)
					if !log.Should(log.Wrap(err, "on parsing public key from user %s", user.Name)) {
						configMu.RUnlock()
						return ssh.KeysEqual(userKey, key)
					}
				}
			}
			configMu.RUnlock()
			return false
		}),
		PasswordHandler: ssh.PasswordHandler(func(ctx ssh.Context, password string) bool {
			configMu.RLock()
			for _, user := range config.Users {
				if user.Name == ctx.User() {
					if user.Password == "" {
						continue
					}
					configMu.RUnlock()
					return user.Password == password
				}
			}
			configMu.RUnlock()
			return false
		}),
		Handler: ssh.Handler(func(s ssh.Session) {
			io.WriteString(s, "Only remote forwarding available...\n")
		}),
		LocalPortForwardingCallback: ssh.LocalPortForwardingCallback(func(ctx ssh.Context, destinationHost string, destinationPort uint32) bool {
			log.Println("attempt to GRAB", destinationHost, destinationPort, "for user", ctx.User(), ctx.RemoteAddr(), "granted")

			configMu.RLock()
			for _, user := range config.Users {
				if user.Name != ctx.User() {
					continue
				}

				for _, allowed := range user.AllowedConnects {
					if allowed == destinationHost+":*" || allowed == net.JoinHostPort(destinationHost, fmt.Sprint(destinationPort)) {
						configMu.RUnlock()
						return true
					}
				}
				break
			}
			configMu.RUnlock()
			return false
		}),
		// What the providers do:
		ReversePortForwardingCallback: ssh.ReversePortForwardingCallback(func(ctx ssh.Context, host string, port uint32) bool {
			log.Infoln("attempt to BIND", host, port, "for user", ctx.User(), ctx.RemoteAddr(), "...")
			log.Info("Searching for ", net.JoinHostPort(host, fmt.Sprint(port)))

			configMu.RLock()
			for _, user := range config.Users {
				if user.Name != ctx.User() {
					continue
				}

				for _, allowed := range user.AllowedBinds {
					if allowed == host+":*" || allowed == net.JoinHostPort(host, fmt.Sprint(port)) {
						configMu.RUnlock()
						log.Info("OK")
						return true
					}
				}
				break
			}
			configMu.RUnlock()
			log.Warn("Failed")
			return false
		}),
		RequestHandlers: map[string]ssh.RequestHandler{
			"tcpip-forward":        forwardHandler.HandleSSHRequest,
			"cancel-tcpip-forward": forwardHandler.HandleSSHRequest,
		},
		ChannelHandlers: map[string]ssh.ChannelHandler{
			"direct-tcpip": filehandlers.DirectTCPIPHandler,
		},
	}

	server.SetOption(ssh.HostKeyFile("hostkeys/server_id_rsa"))

	log.Fatal(server.ListenAndServe())
}

func startWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	err = watcher.Add(configPath)
	if err != nil {
		return err
	}

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Info("Reloading config...")
					configMu.Lock()
					store.Load("config.yml", &config)
					configMu.Unlock()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Errorln("in fs watcher:", err)
			}
		}
	}()
	return nil
}
