package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/jacobweinstock/ipxe-bin/backend"
	"github.com/jacobweinstock/ipxe-bin/binary"
)

type server struct {
	backend backend.Reader
	log     logr.Logger
}

func ListenAndServe(ctx context.Context, l logr.Logger, b backend.Reader, addr string) error {
	router := http.NewServeMux()
	s := server{backend: b, log: l}
	l.V(0).Info("serving http", "addr", addr)
	router.HandleFunc("/undionly.kpxe", s.serveFile)
	router.HandleFunc("/ipxe.efi", s.serveFile)
	router.HandleFunc("/snp.efi", s.serveFile)
	srv := http.Server{
		Addr:    addr,
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

func (s server) serveFile(w http.ResponseWriter, req *http.Request) {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		s.log.V(0).Error(fmt.Errorf("%s: not allowed", req.RemoteAddr), "could not get you IP address")
	}
	allowed, err := s.backend.Allowed(context.TODO(), net.ParseIP(host))
	if err != nil {
		// TODO(jacobweinstock): connections errors should probably be 500 but not found errors should be 403
		http.Error(w, "error talking with backend", http.StatusInternalServerError)
		s.log.V(0).Error(err, "error talking with backend")
		return
	}
	if !allowed {
		http.Error(w, "not allowed", http.StatusForbidden)
		s.log.V(0).Error(fmt.Errorf("%s: not allowed", req.RemoteAddr), "reported as not allowed")
		return
	}
	got := filepath.Base(req.URL.Path)
	file := binary.Files[got]
	if _, err := w.Write(file); err != nil {
		s.log.V(0).Error(err, "error serving file")
	}
}
