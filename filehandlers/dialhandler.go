package filehandlers

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gliderlabs/ssh"
	log "github.com/s00500/env_logger"
	gossh "golang.org/x/crypto/ssh"
)

// direct-tcpip data struct as specified in RFC4254, Section 7.2
type localForwardChannelData struct {
	DestAddr string
	DestPort uint32

	OriginAddr string
	OriginPort uint32
}

// DirectTCPIPHandler can be enabled by adding it to the server's
// ChannelHandlers under direct-tcpip.
func DirectTCPIPHandler(srv *ssh.Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx ssh.Context) {
	d := localForwardChannelData{}
	if err := gossh.Unmarshal(newChan.ExtraData(), &d); err != nil {
		err := newChan.Reject(gossh.ConnectionFailed, "error parsing forward data: "+err.Error())
		log.Should(err)
		return
	}

	if srv.LocalPortForwardingCallback == nil || !srv.LocalPortForwardingCallback(ctx, d.DestAddr, d.DestPort) {
		err := newChan.Reject(gossh.Prohibited, "port forwarding is disabled BITCH")
		log.Should(err)
		return
	}

	dest := net.JoinHostPort(d.DestAddr, strconv.FormatInt(int64(d.DestPort), 10))
	dest = strings.ReplaceAll(dest, "/", "")
	dest = strings.ReplaceAll(dest, "..", "")

	file := filepath.Join(socketDir, dest)
	file += ".socket"

	if _, err := os.Stat(file); os.IsNotExist(err) {
		err := newChan.Reject(gossh.ConnectionFailed, "Not available")
		log.Should(err)
		return
	}

	var dialer net.Dialer
	dconn, err := dialer.DialContext(ctx, "unix", file)
	if err != nil {
		err := newChan.Reject(gossh.ConnectionFailed, err.Error())
		log.Should(err)
		return
	}

	ch, reqs, err := newChan.Accept()
	if err != nil {
		dconn.Close()
		return
	}
	go gossh.DiscardRequests(reqs)

	go func() {
		defer ch.Close()
		defer dconn.Close()
		_, err := io.Copy(ch, dconn)
		log.Should(err)
	}()
	go func() {
		defer ch.Close()
		defer dconn.Close()
		_, err := io.Copy(dconn, ch)
		log.Should(err)
	}()
}
