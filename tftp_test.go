package ipxe

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/jacobweinstock/ipxe/binary"
	"github.com/pin/tftp"
	"inet.af/netaddr"
)

func TestListenAndServeTFTP(t *testing.T) {
	ht := &HandleTFTP{log: logr.Discard()}
	srv := tftp.NewServer(ht.ReadHandler, ht.WriteHandler)
	type args struct {
		ctx  context.Context
		addr netaddr.IPPort
		h    *tftp.Server
	}
	tests := []struct {
		name    string
		args    args
		wantErr interface{}
	}{
		{
			name: "fail",
			args: args{
				ctx:  context.Background(),
				addr: netaddr.IPPortFrom(netaddr.IPv4(127, 0, 0, 1), 80),
				h:    nil,
			},
			wantErr: &net.OpError{},
		},
		{
			name: "success",
			args: args{
				ctx:  func() context.Context { c, cn := context.WithCancel(context.Background()); defer cn(); return c }(),
				addr: netaddr.IPPortFrom(netaddr.IPv4(127, 0, 0, 1), 9999),
				h:    srv,
			},
			wantErr: interface{}(nil),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errChan := make(chan error, 1)
			go func() {
				errChan <- ListenAndServeTFTP(tt.args.ctx, tt.args.addr, tt.args.h)
			}()

			if tt.args.h != nil {
				tt.args.ctx.Done()
				time.Sleep(time.Second)
				tt.args.h.Shutdown()
			}
			err := <-errChan
			if !errors.As(err, &tt.wantErr) && err != nil {
				t.Fatalf("error mismatch, got: %T, want: %T", err, tt.wantErr)
			}
		})
	}
}

func TestHandlerTFTP_ReadHandler(t *testing.T) {
	ht := &HandleTFTP{log: logr.Discard()}
	rf := &fakeReaderFrom{
		addr:    net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9999},
		content: make([]byte, len(binary.Files["snp.efi"])),
	}
	err := ht.ReadHandler("snp.efi", rf)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(rf.content, binary.Files["snp.efi"]); diff != "" {
		t.Fatal(diff)
	}
}

func TestHandlerTFTP_WriteHandler(t *testing.T) {
	ht := &HandleTFTP{log: logr.Discard()}
	rf := &fakeReaderFrom{addr: net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9999}}
	err := ht.WriteHandler("snp.efi", rf)
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("error mismatch, got: %T, want: %T", err, os.ErrPermission)
	}
}

type fakeReaderFrom struct {
	addr    net.UDPAddr
	content []byte
}

func (f *fakeReaderFrom) ReadFrom(r io.Reader) (n int64, err error) {
	nInt, err := r.Read(f.content)
	return int64(nInt), err
}

func (f *fakeReaderFrom) SetSize(_ int64) {}

func (f *fakeReaderFrom) RemoteAddr() net.UDPAddr {
	return f.addr
}

func (f *fakeReaderFrom) WriteTo(_ io.Writer) (n int64, err error) {
	return 0, nil
}
