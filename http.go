package ipxe

import (
	"context"
	"net"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/go-logr/logr"
	"github.com/jacobweinstock/ipxe/binary"
	"inet.af/netaddr"
)

type HandleHTTP struct {
	log logr.Logger
}

func ListenAndServeHTTP(ctx context.Context, addr netaddr.IPPort, h *http.Server) error {
	conn, err := net.Listen("tcp", addr.String())
	if err != nil {
		return err
	}
	return ServeHTTP(ctx, conn, h)
}

func ServeHTTP(_ context.Context, conn net.Listener, h *http.Server) error {
	return h.Serve(conn)
}

func trimFirstRune(s string) string {
	_, i := utf8.DecodeRuneInString(s)
	return s[i:]
}

func (s HandleHTTP) Handler(w http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	host, port, _ := net.SplitHostPort(req.RemoteAddr)
	s.log = s.log.WithValues("host", host, "port", port)
	m := path.Dir(req.URL.Path)
	if strings.HasPrefix(m, "/") {
		m = trimFirstRune(path.Dir(req.URL.Path))
	}
	mac, _ := net.ParseMAC(m)
	s.log = s.log.WithValues("mac", mac)

	got := filepath.Base(req.URL.Path)
	file, found := binary.Files[got]
	if !found {
		s.log.Info("could not find file", "file", got)
		http.NotFound(w, req)
		return
	}
	b, err := w.Write(file)
	if err != nil {
		s.log.Error(err, "error serving file")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.log.Info("file served", "bytes sent", b, "content size", len(file), "file", got)
	w.WriteHeader(http.StatusOK)
}
