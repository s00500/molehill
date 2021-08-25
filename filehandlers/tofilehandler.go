package filehandlers

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	log "github.com/s00500/env_logger"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

const (
	forwardedTCPChannelType = "forwarded-tcpip"
)

type remoteForwardRequest struct {
	BindAddr string
	BindPort uint32
}

type remoteForwardSuccess struct {
	BindPort uint32
}

type remoteForwardCancelRequest struct {
	BindAddr string
	BindPort uint32
}

type remoteForwardChannelData struct {
	DestAddr   string
	DestPort   uint32
	OriginAddr string
	OriginPort uint32
}

type ForwardedTCPToFileHandler struct {
	forwards map[string]net.Listener
	sync.Mutex
}

const socketDir = "sockets"

// HandleSSHRequest handles ssh port forward to a file
func (h *ForwardedTCPToFileHandler) HandleSSHRequest(ctx ssh.Context, srv *ssh.Server, req *gossh.Request) (bool, []byte) {
	h.Lock()
	if h.forwards == nil {
		h.forwards = make(map[string]net.Listener)
	}
	h.Unlock()
	conn := ctx.Value(ssh.ContextKeyConn).(*gossh.ServerConn)
	switch req.Type {
	case "tcpip-forward":
		var reqPayload remoteForwardRequest
		if err := gossh.Unmarshal(req.Payload, &reqPayload); err != nil {
			log.Error("failed to parse forward request: ", err)
			return false, []byte{}
		}

		if srv.ReversePortForwardingCallback == nil || !srv.ReversePortForwardingCallback(ctx, reqPayload.BindAddr, reqPayload.BindPort) {
			return false, []byte("port forwarding is disabled")
		}
		addrLn := net.JoinHostPort(reqPayload.BindAddr, fmt.Sprint(reqPayload.BindPort))
		addr := net.JoinHostPort(reqPayload.BindAddr, strconv.Itoa(int(reqPayload.BindPort)))
		//username := ctx.Value(ssh.ContextKeyUser).(string)

		// if the address is localhost then we actually perform a listen!
		var ln net.Listener
		var err error
		var socketAddr string
		socketAddr = filepath.Join(socketDir, addrLn+".socket")

		if strings.HasPrefix(addr, "0.0.0.0:") {
			ln, err = net.Listen("tcp", addr)
			if err != nil {
				// TODO: log listen failure
				return false, []byte{}
			}
		} else {
			ln, err = net.Listen("unix", socketAddr)
			if err != nil {
				log.Errorf("Failed to listen on socket %s: %s", socketAddr, err)
				return false, []byte{}
			}
		}

		_, destPortStr, _ := net.SplitHostPort(addr) // Do not use the listener address here obviously
		destPort, _ := strconv.Atoi(destPortStr)

		h.Lock()
		h.forwards[socketAddr] = ln
		h.Unlock()
		go func() {
			<-ctx.Done()
			h.Lock()
			ln, ok := h.forwards[socketAddr]
			h.Unlock()
			if ok {
				ln.Close()
			}
		}()
		go func() {
			for {
				c, err := ln.Accept()
				if err != nil {
					log.Trace(err)
					break // Will also error
				}
				originAddr, orignPortStr, _ := net.SplitHostPort(c.RemoteAddr().String())
				originPort, _ := strconv.Atoi(orignPortStr)
				payload := gossh.Marshal(&remoteForwardChannelData{
					DestAddr:   reqPayload.BindAddr,
					DestPort:   uint32(destPort),
					OriginAddr: originAddr,
					OriginPort: uint32(originPort),
				})
				go func() {
					ch, reqs, err := conn.OpenChannel(forwardedTCPChannelType, payload)
					if err != nil {
						log.Tracef("failed to open channel for socket %s: %s", socketAddr, err) // happens everytime if destination is no reachable...
						c.Close()
						return
					}
					go gossh.DiscardRequests(reqs)
					go func() {
						defer ch.Close()
						defer c.Close()
						io.Copy(ch, c)
					}()
					go func() {
						defer ch.Close()
						defer c.Close()
						io.Copy(c, ch)
					}()
				}()
			}
			h.Lock()
			delete(h.forwards, socketAddr)
			h.Unlock()
		}()
		return true, gossh.Marshal(&remoteForwardSuccess{uint32(destPort)})

	case "cancel-tcpip-forward":
		var reqPayload remoteForwardCancelRequest
		if err := gossh.Unmarshal(req.Payload, &reqPayload); err != nil {
			log.Error("failed to parse cancel forward request: ", err)
			return false, []byte{}
		}

		//username := ctx.Value(ssh.ContextKeyUser).(string)
		addrLn := net.JoinHostPort(reqPayload.BindAddr, fmt.Sprint(reqPayload.BindPort))
		socketAddr := filepath.Join(socketDir, addrLn+".socket")

		h.Lock()
		ln, ok := h.forwards[socketAddr]
		h.Unlock()
		if ok {
			ln.Close()
		}
		if !strings.HasPrefix(addrLn, "0.0.0.0:") {
			log.Should(os.Remove(socketAddr))
		}

		return true, nil
	default:
		return false, nil
	}
}
