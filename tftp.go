package ipxe

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"

	"github.com/go-logr/logr"
	"github.com/jacobweinstock/ipxe/binary"
	"github.com/pin/tftp"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"inet.af/netaddr"
)

// HandleTFTP is the struct that implements the TFTP read and write function handlers.
type HandleTFTP struct {
	Log logr.Logger
}

// ListenAndServeTFTP sets up the listener on the given address and serves TFTP requests.
func ListenAndServeTFTP(ctx context.Context, addr netaddr.IPPort, s *tftp.Server) error {
	a, err := net.ResolveUDPAddr("udp", addr.String())
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", a)
	if err != nil {
		return err
	}
	return ServeTFTP(ctx, conn, s)
}

// ServeTFTP serves TFTP requests using the given conn and server.
func ServeTFTP(_ context.Context, conn net.PacketConn, s *tftp.Server) error {
	return s.Serve(conn)
}

// ReadHandler handlers TFTP GET requests.
func (t HandleTFTP) ReadHandler(filename string, rf io.ReaderFrom) error {
	client := net.UDPAddr{}
	if rpi, ok := rf.(tftp.OutgoingTransfer); ok {
		client = rpi.RemoteAddr()
	}

	full := filename
	filename = path.Base(filename)
	l := t.Log.WithValues("event", "get", "filename", filename, "uri", full, "client", client)

	// clients can send traceparent over TFTP by appending the traceparent string
	// to the end of the filename they really want
	longfile := filename // hang onto this to report in traces
	ctx, shortfile, err := extractTraceparentFromFilename(context.Background(), filename)
	if err != nil {
		l.Error(err, "")
	}
	if shortfile != filename {
		l = l.WithValues("filename", shortfile) // flip to the short filename in logs
		l.Info("client requested filename '", filename, "' with a traceparent attached and has been shortened to '", shortfile, "'")
		filename = shortfile
	}
	tracer := otel.Tracer("TFTP")
	_, span := tracer.Start(ctx, "TFTP get",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(attribute.String("filename", filename)),
		trace.WithAttributes(attribute.String("requested-filename", longfile)),
		trace.WithAttributes(attribute.String("IP", client.IP.String())),
	)

	// parse mac from the full filename
	mac, _ := net.ParseMAC(path.Dir(full))
	l = l.WithValues("mac", mac.String())

	span.SetStatus(codes.Ok, filename)
	span.End()

	content, ok := binary.Files[filepath.Base(filename)]
	if !ok {
		err := errors.Wrap(os.ErrNotExist, "file unknown")
		l.Error(err, "file unknown")
		return err
	}
	ct := bytes.NewReader(content)

	b, err := rf.ReadFrom(ct)
	if err != nil {
		l.Error(err, "file serve failed", "EOF", errors.Is(err, io.EOF), "b", b, "content size", len(content))
		return err
	}
	l.Info("file served", "bytes sent", b, "content size", len(content))
	return nil
}

// WriteHandler handles TFTP PUT requests. It will always return an error. This library does not support PUT.
func (t HandleTFTP) WriteHandler(filename string, wt io.WriterTo) error {
	err := errors.Wrap(os.ErrPermission, "access_violation")
	client := net.UDPAddr{}
	if rpi, ok := wt.(tftp.OutgoingTransfer); ok {
		client = rpi.RemoteAddr()
	}
	t.Log.Error(err, "client", client, "event", "put", "filename", filename)

	return err
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
