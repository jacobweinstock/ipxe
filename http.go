package ipxe

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-logr/logr"
	"github.com/jacobweinstock/ipxe/binary"
	"inet.af/netaddr"
)

type server struct {
	backend Reader
	log     logr.Logger
}

func ListenAndServe(ctx context.Context, l logr.Logger, b Reader, addr netaddr.IPPort, _ time.Duration) error {
	router := http.NewServeMux()
	s := server{backend: b, log: l}
	l.V(0).Info("serving http", "addr", addr)
	router.HandleFunc("/", s.serveFile)

	srv := http.Server{
		Addr:    addr.String(),
		Handler: router,
	}
	errChan := make(chan error)
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			errChan <- err
		}
		errChan <- nil
	}()

	var err error
	select {
	case <-ctx.Done():
		err = srv.Shutdown(ctx)
	case e := <-errChan:
		err = e
	}
	return err
}

func trimFirstRune(s string) string {
	_, i := utf8.DecodeRuneInString(s)
	return s[i:]
}

func (s server) serveFile(w http.ResponseWriter, req *http.Request) {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		s.log.Error(fmt.Errorf("%s: not allowed", req.RemoteAddr), "could not get your IP address")
	}

	m := path.Dir(req.URL.Path)
	if strings.HasPrefix(m, "/") {
		m = trimFirstRune(path.Dir(req.URL.Path))
	}
	mac, err := net.ParseMAC(m)
	if err != nil {
		s.log.Info("could not parse mac from request URI", "err", err.Error())
	}
	s.log = s.log.WithValues("mac", mac, "host", host)
	allowed, err := s.backend.Allowed(context.TODO(), net.ParseIP(host), mac)
	if err != nil {
		// TODO(jacobweinstock): connections errors should probably be 500 but not found errors should be 403
		http.Error(w, "error talking with backend", http.StatusInternalServerError)
		s.log.Error(err, "error talking with backend")
		return
	}
	if !allowed {
		http.Error(w, "not allowed", http.StatusForbidden)
		s.log.Error(fmt.Errorf("%s: not allowed", req.RemoteAddr), "reported as not allowed")
		return
	}
	got := filepath.Base(req.URL.Path)
	file := binary.Files[got]
	if _, err := w.Write(file); err != nil {
		s.log.Error(err, "error serving file")
	}
}
