package tftp

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
	"time"

	"github.com/go-logr/logr"
	"github.com/jacobweinstock/ipxe/backend"
	"github.com/jacobweinstock/ipxe/binary"
	"github.com/pin/tftp"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"inet.af/netaddr"
)

type tftpHandler struct {
	log     logr.Logger
	backend backend.Reader
}

// Serve listens on the given address and serves TFTP requests.
func Serve(ctx context.Context, l logr.Logger, b backend.Reader, addr netaddr.IPPort, timeout time.Duration) error {
	errChan := make(chan error)
	t := &tftpHandler{log: l, backend: b}
	s := tftp.NewServer(t.readHandler, t.writeHandler)
	s.SetTimeout(timeout)
	go func() {
		errChan <- s.ListenAndServe(addr.String())
	}()
	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		s.Shutdown()
		return ctx.Err()
	}
}

func (t tftpHandler) readHandler(filename string, rf io.ReaderFrom) error {
	var ip net.IP
	if rpi, ok := rf.(tftp.OutgoingTransfer); ok {
		ip = rpi.RemoteAddr().IP
	} else {
		ip = net.IP{}
	}

	full := filename
	filename = path.Base(filename)
	l := t.log.WithValues("client", ip.String(), "event", "open", "filename", filename, "random", time.Now().UnixNano())

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
	ctx, span := tracer.Start(ctx, "TFTP get",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(attribute.String("filename", filename)),
		trace.WithAttributes(attribute.String("requested-filename", longfile)),
		trace.WithAttributes(attribute.String("IP", ip.String())),
	)

	span.AddEvent("job.CreateFromIP")

	// parse mac from the full filename
	mac, err := net.ParseMAC(path.Dir(full))
	if err != nil {
		l.Error(err, "couldnt get mac from request path")
	}
	l = l.WithValues("mac", mac.String())
	_, err = t.backend.Mac(ctx, ip, mac)
	if err != nil {
		l.Error(err, "retrieved job is empty")
		span.SetStatus(codes.Error, "no existing job: "+err.Error())
		span.End()

		return fmt.Errorf("mac(%q) not found in backend: %w", mac, err)
	}

	// This gates serving PXE file by
	// 1. the existence of a hardware record in tink server
	// AND
	// 2. the network.interfaces[].netboot.allow_pxe value, in the tink server hardware record, equal to true
	// This allows serving custom ipxe scripts, starting up into OSIE or other installation environments
	// without a tink workflow present.
	allowed, err := t.backend.Allowed(ctx, ip, mac)
	if err != nil {
		l.Error(err, "failed to determine if client is allowed to boot")
		span.SetStatus(codes.Error, "failed to determine if client is allowed to boot: "+err.Error())
		span.End()
		return err
	}
	if !allowed {
		l.Info("the hardware data for this machine, or lack there of, does not allow it to pxe; allow_pxe: false")
		span.SetStatus(codes.Error, "allow_pxe is false")
		span.End()

		return fmt.Errorf("allow_pxe is false")
	}

	span.SetStatus(codes.Ok, filename)
	span.End()

	content, ok := binary.Files[filepath.Base(filename)]
	if !ok {
		err := errors.Wrap(os.ErrNotExist, "unknown file")
		l.Error(err, "unknown file")
		return err
	}
	ct := bytes.NewReader(content)

	rf.(tftp.OutgoingTransfer).SetSize(int64(ct.Len()))
	b, err := rf.ReadFrom(ct)
	if err != nil {
		l.Error(err, "failed to send file", "EOF?", errors.Is(err, io.EOF), "b", b, "content size", len(content))
		return err
	}
	l.Info("served", "bytes sent", b, "content size", len(content))
	return nil
}

func (t tftpHandler) writeHandler(filename string, wt io.WriterTo) error {
	err := errors.Wrap(os.ErrPermission, "access_violation")
	var ip net.IP
	if rpi, ok := wt.(tftp.OutgoingTransfer); ok {
		ip = rpi.RemoteAddr().IP
	} else {
		ip = net.IP{}
	}
	t.log.Error(err, "client", ip, "event", "create", "filename", filename)

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
