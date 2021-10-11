package http

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/jacobweinstock/ipxe-bin/backend"
	"github.com/jacobweinstock/ipxe-bin/bin"
)

type server struct {
}

func ListenAndServe(ctx context.Context, l logr.Logger, b backend.Reader, addr string) error {
	router := http.NewServeMux()
	router.HandleFunc("/undionly.kpxe", server{}.serveFile)
	router.HandleFunc("/ipxe.efi", server{}.serveFile)
	router.HandleFunc("/snp.efi", server{}.serveFile)
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
		fmt.Println("1")
	case e := <-errChan:
		fmt.Println("2")
		err = e
	}
	return err
}

func (s server) serveFile(w http.ResponseWriter, req *http.Request) {
	got := req.URL.Path
	fmt.Println("url path:", got)
	got = filepath.Base(got)
	file, found := bin.Files[got]
	if !found {
		http.Error(w, "we dont serve that file", http.StatusNotFound)
		return
	}
	w.Write(file)
}
