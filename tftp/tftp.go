package tftp

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"regexp"
	"time"

	"github.com/go-logr/logr"
	"github.com/jacobweinstock/ipxe-bin/backend"
	"github.com/jacobweinstock/ipxe-bin/bin"
	"github.com/pkg/errors"
	tftpgo "github.com/tinkerbell/tftp-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type tftpHandler struct {
	log     logr.Logger
	backend backend.Reader
}

// ServeTFTP listens on the given address and serves TFTP requests.
func ServeTFTP(ctx context.Context, l logr.Logger, b backend.Reader, addr string) error {
	errChan := make(chan error)
	go func() {
		errChan <- tftpgo.ListenAndServe(addr, tftpHandler{l, b})
	}()
	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Read implements the tftpgo.ReadCloser interface.
func (t tftpHandler) ReadFile(c tftpgo.Conn, filename string) (tftpgo.ReadCloser, error) {
	/*
		labels := prometheus.Labels{"from": "tftp", "op": "read"}
		metrics.JobsTotal.With(labels).Inc()
		metrics.JobsInProgress.With(labels).Inc()
		timer := prometheus.NewTimer(metrics.JobDuration.With(labels))
		defer timer.ObserveDuration()
		defer metrics.JobsInProgress.With(labels).Dec()
	*/

	ip, _ := tftpClientIP(c.RemoteAddr())
	filename = path.Base(filename)
	l := t.log.WithValues("client", ip.String(), "event", "open", "filename", filename)

	// clients can send traceparent over TFTP by appending the traceparent string
	// to the end of the filename they really want
	longfile := filename // hang onto this to report in traces
	ctx, shortfile, err := extractTraceparentFromFilename(context.Background(), filename)
	if err != nil {
		l.V(0).Error(err, "")
	}
	if shortfile != filename {
		l = l.WithValues("filename", shortfile) // flip to the short filename in logs
		l.V(0).Info("client requested filename '", filename, "' with a traceparent attached and has been shortened to '", shortfile, "'")
		filename = shortfile
	}
	tracer := otel.Tracer("TFTP")
	ctx, span := tracer.Start(ctx, "TFTP get",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(attribute.String("filename", filename)),
		trace.WithAttributes(attribute.String("requested-filename", longfile)),
		trace.WithAttributes(attribute.String("IP", ip.String())),
	)

	span.AddEvent("job.CreateFromIP")

	j, err := t.backend.Mac(ctx, ip)
	if err != nil {
		l.V(0).Error(err, "retrieved job is empty")
		span.SetStatus(codes.Error, "no existing job: "+err.Error())
		span.End()

		return serveFakeReader(l, filename)
	}

	// This gates serving PXE file by
	// 1. the existence of a hardware record in tink server
	// AND
	// 2. the network.interfaces[].netboot.allow_pxe value, in the tink server hardware record, equal to true
	// This allows serving custom ipxe scripts, starting up into OSIE or other installation environments
	// without a tink workflow present.
	allowed, err := t.backend.Allowed(ctx, ip)
	if err != nil {
		l.V(0).Error(err, "failed to determine if client is allowed to boot")
		span.SetStatus(codes.Error, "failed to determine if client is allowed to boot: "+err.Error())
		span.End()
		return serveFakeReader(l, filename)
	}
	if !allowed {
		l.Info("the hardware data for this machine, or lack there of, does not allow it to pxe; allow_pxe: false")
		span.SetStatus(codes.Error, "allow_pxe is false")
		span.End()

		return serveFakeReader(l, filename)
	}

	span.SetStatus(codes.Ok, filename)
	span.End()

	return Open(ctx, l, j, filename, ip.String())
}

// extractTraceparentFromFilename takes a context and filename and checks the filename for
// a traceparent tacked onto the end of it. If there is a match, the traceparent is extracted
// and a new SpanContext is contstructed and added to the context.Context that is returned.
// The filename is shortened to just the original filename so the rest of boots tftp can
// carry on as usual.
func extractTraceparentFromFilename(ctx context.Context, filename string) (context.Context, string, error) {
	// traceparentRe captures 4 items, the original filename, the trace id, span id, and trace flags
	traceparentRe := regexp.MustCompile("^(.*)-[[:xdigit:]]{2}-([[:xdigit:]]{32})-([[:xdigit:]]{16})-([[:xdigit:]]{2})")
	parts := traceparentRe.FindStringSubmatch(filename)
	if len(parts) == 5 {
		traceID, err := trace.TraceIDFromHex(parts[2])
		if err != nil {
			return ctx, filename, fmt.Errorf("parsing OpenTelemetry trace id %q failed: %w", parts[2], err)
		}

		spanID, err := trace.SpanIDFromHex(parts[3])
		if err != nil {
			return ctx, filename, fmt.Errorf("parsing OpenTelemetry span id %q failed: %w", parts[3], err)
		}

		// create a span context with the parent trace id & span id
		spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    traceID,
			SpanID:     spanID,
			Remote:     true,
			TraceFlags: trace.FlagsSampled, // TODO: use the parts[4] value instead
		})

		// inject it into the context.Context and return it along with the original filename
		return trace.ContextWithSpanContext(ctx, spanCtx), parts[1], nil
	}
	// no traceparent found, return everything as it was
	return ctx, filename, nil
}

func serveFakeReader(l logr.Logger, filename string) (tftpgo.ReadCloser, error) {
	switch filename {
	case "test.1mb":
		l.V(0).Info("test.1mb", "tftp_fake_read", true)

		return &fakeReader{1 * 1024 * 1024}, nil
	case "test.8mb":
		l.V(0).Info("test.8mb", "tftp_fake_read", true)

		return &fakeReader{8 * 1024 * 1024}, nil
	}
	l.V(0).Error(errors.Wrap(os.ErrPermission, "access_violation"), "add why")

	return nil, os.ErrPermission
}

func (t tftpHandler) WriteFile(c tftpgo.Conn, filename string) (tftpgo.WriteCloser, error) {
	ip, _ := tftpClientIP(c.RemoteAddr())
	err := errors.Wrap(os.ErrPermission, "access_violation")
	t.log.V(0).Error(err, "client", ip, "event", "create", "filename", filename)

	return nil, err
}

func tftpClientIP(addr net.Addr) (net.IP, error) {
	switch a := addr.(type) {
	case *net.IPAddr:
		return a.IP, nil
	case *net.UDPAddr:
		return a.IP, nil
	case *net.TCPAddr:
		return a.IP, nil
	}

	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		err = errors.Wrap(err, "parse host:port")

		return nil, err
	}

	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			ip = v4
		}

		return ip, nil
	}

	return nil, fmt.Errorf("unable to get IP")
}

var zeros = make([]byte, 1456)

type fakeReader struct {
	N int
}

func (r *fakeReader) Close() error {
	return nil
}

func (r *fakeReader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return
	}
	if len(p) > r.N {
		p = p[:r.N]
	}

	for len(p) > 0 {
		n = copy(p, zeros)
		r.N -= n
		p = p[n:]
	}

	if r.N == 0 {
		err = io.EOF
	}

	return
}

type Transfer struct {
	log    logr.Logger
	unread []byte
	start  time.Time
}

// Open sets up a tftp transfer object that implements tftpgo.ReadCloser.
func Open(_ context.Context, l logr.Logger, mac net.HardwareAddr, filename, client string) (*Transfer, error) {
	logger := l.WithValues("mac", mac, "client", client, "filename", filename)

	content, ok := bin.Files[filename]
	if !ok {
		err := errors.Wrap(os.ErrNotExist, "unknown file")
		l.V(0).Error(err, "event", "open")
		return nil, err
	}

	t := &Transfer{
		log:    logger,
		unread: content,
		start:  time.Now(),
	}

	t.log.V(1).Info("debugging", "event", "open")
	return t, nil
}

func (t *Transfer) Close() error {
	d := time.Since(t.start)
	n := len(t.unread)

	t.log.V(0).Info("event", "event", "close", "duration", d, "unread", n)

	t.unread = nil
	return nil
}

func (t *Transfer) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		t.log.V(0).Info("event", "read", 0, "unread", len(t.unread))
		return
	}

	n = copy(p, t.unread)
	t.unread = t.unread[n:]

	if len(t.unread) == 0 {
		err = io.EOF
	}

	t.log.V(1).Info("event", "read", n, "unread", len(t.unread))
	return
}

func (t *Transfer) Size() int {
	return len(t.unread)
}
