package backend

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/tinkerbell/tink/protos/hardware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

const (
	schemeFile  = "file"
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

type Tinkerbell struct {
	Client hardware.HardwareServiceClient
	Log    logr.Logger
}

func (t Tinkerbell) Mac(ctx context.Context, ip net.IP, mac net.HardwareAddr) (net.HardwareAddr, error) {
	hw, err := t.Client.ByIP(ctx, &hardware.GetRequest{Ip: ip.String()})
	if err != nil {
		return nil, fmt.Errorf("failed to get hardware info: %w", err)
	}
	for _, elem := range hw.GetNetwork().GetInterfaces() {
		if net.ParseIP(elem.GetDhcp().GetIp().GetAddress()).Equal(ip) {
			mac, err := net.ParseMAC(elem.GetDhcp().GetMac())
			if err != nil {
				return nil, err
			}

			return mac, nil
		}
	}

	return nil, fmt.Errorf("not found")
}

func (t Tinkerbell) Allowed(ctx context.Context, ip net.IP, mac net.HardwareAddr) (bool, error) {
	hw, err := t.Client.ByIP(ctx, &hardware.GetRequest{Ip: ip.String()})
	if err != nil {
		fmt.Println("==========")
		fmt.Printf("%T\n", err)
		errStatus, _ := status.FromError(err)
		fmt.Println(errStatus.Message())
		fmt.Println(errStatus.Code())
		fmt.Println("==========")
		err = errors.Wrap(err, errStatus.Code().String())
		t.Log.V(0).Error(err, "failed to get hardware info")
		return false, err
	}
	for _, elem := range hw.GetNetwork().GetInterfaces() {
		if net.ParseIP(elem.GetDhcp().GetIp().GetAddress()).Equal(ip) {
			return elem.GetNetboot().GetAllowPxe(), nil
		}
	}

	return false, nil
}

// setupClient is a small control loop to create a tink server client.
// it keeps trying so that if the problem is temporary or can be resolved and the
// this doesn't stop and need to be restarted by an outside process or person.
func SetupClient(ctx context.Context, log logr.Logger, tlsVal string, tink string) (*grpc.ClientConn, error) {
	if tink == "" {
		return nil, errors.New("tink server address is required")
	}
	// setup tink server grpc client
	dialOpt, err := grpcTLS(tlsVal)
	if err != nil {
		log.V(0).Error(err, "unable to create gRPC client TLS dial option")
		return nil, err
	}

	grpcClient, err := grpc.DialContext(ctx, tink, dialOpt)
	if err != nil {
		log.V(0).Error(err, "error connecting to tink server")
		return nil, err
	}

	return grpcClient, nil
}

// toCreds takes a byte string, assumed to be a tls cert, and creates a transport credential.
func toCreds(pemCerts []byte) credentials.TransportCredentials {
	cp := x509.NewCertPool()
	ok := cp.AppendCertsFromPEM(pemCerts)
	if !ok {
		return nil
	}
	return credentials.NewClientTLSFromCert(cp, "")
}

// loadTLSSecureOpts handles taking a string that is assumed to be a boolean
// and creating a grpc.DialOption for TLS.
// If the value is true, the server has a cert from a well known CA.
// If the value is false, the server is not using TLS

// loadTLSFromFile handles reading in a cert file and forming a TLS grpc.DialOption

// loadTLSFromHTTP handles reading a cert from an HTTP endpoint and forming a TLS grpc.DialOption

// grpcTLS is the logic for how/from where TLS should be loaded.
func grpcTLS(tlsVal string) (grpc.DialOption, error) {
	u, err := url.Parse(tlsVal)
	if err != nil {
		return nil, errors.Wrap(err, "must be file://, http://, or string boolean")
	}
	switch u.Scheme {
	case "":
		secure, err := strconv.ParseBool(tlsVal)
		if err != nil {
			return nil, errors.WithMessagef(err, "expected boolean, got: %v", tlsVal)
		}
		var dialOpt grpc.DialOption
		if secure {
			// 1. the server has a cert from a well known CA - grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))
			dialOpt = grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12}))
		} else {
			// 2. the server is not using TLS - grpc.WithInsecure()
			dialOpt = grpc.WithInsecure()
		}
		return dialOpt, nil
	case schemeFile:
		// 3. the server has a self-signed cert and the cert have be provided via file/env/flag -
		data, err := os.ReadFile(filepath.Join(u.Host, u.Path))
		if err != nil {
			return nil, err
		}
		return grpc.WithTransportCredentials(toCreds(data)), nil
	case schemeHTTP, schemeHTTPS:
		// 4. the server has a self-signed cert and the cert needs to be grabbed from a URL -
		resp, err := http.NewRequestWithContext(context.Background(), "GET", tlsVal, http.NoBody)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		cert, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		return grpc.WithTransportCredentials(toCreds(cert)), nil
	}
	return nil, fmt.Errorf("not an expected value: %v", tlsVal)
}
