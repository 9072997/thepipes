//go:build windows

package acme

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/mholt/acmez/v3"
	"github.com/mholt/acmez/v3/acme"
)

const leStagingURL = "https://acme-staging-v02.api.letsencrypt.org/directory"

// testACME attempts to obtain a certificate from the Let's Encrypt staging
// environment for the given name (IP address or domain). The certificate is
// discarded on success; the test only verifies reachability.
func testACME(ctx context.Context, name string) error {
	accountKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate account key: %w", err)
	}
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate cert key: %w", err)
	}

	client := acmez.Client{
		Client: &acme.Client{
			Directory: leStagingURL,
		},
		ChallengeSolvers: map[string]acmez.Solver{
			acme.ChallengeTypeHTTP01:    &httpSolver{},
			acme.ChallengeTypeTLSALPN01: &tlsALPNSolver{},
		},
	}

	account := acme.Account{
		TermsOfServiceAgreed: true,
		PrivateKey:           accountKey,
	}
	account, err = client.NewAccount(ctx, account)
	if err != nil {
		return fmt.Errorf("register account: %w", err)
	}

	csr, err := acmez.NewCSR(certKey, []string{name})
	if err != nil {
		return fmt.Errorf("create CSR: %w", err)
	}
	params, err := acmez.OrderParametersFromCSR(account, csr)
	if err != nil {
		return fmt.Errorf("create order params: %w", err)
	}
	params.Profile = "shortlived"

	_, err = client.ObtainCertificate(ctx, params)
	if err != nil {
		return fmt.Errorf("obtain certificate: %w", err)
	}
	return nil
}

// httpSolver serves HTTP-01 challenges on port 80.
type httpSolver struct {
	mu     sync.Mutex
	server *http.Server
	mux    *http.ServeMux
}

func (s *httpSolver) Present(_ context.Context, chal acme.Challenge) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.mux == nil {
		s.mux = http.NewServeMux()
		s.server = &http.Server{
			Addr:    ":80",
			Handler: s.mux,
		}
		ln, err := net.Listen("tcp", ":80")
		if err != nil {
			return fmt.Errorf("listen :80: %w", err)
		}
		go s.server.Serve(ln)
	}

	s.mux.HandleFunc(chal.HTTP01ResourcePath(), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte(chal.KeyAuthorization))
	})
	return nil
}

func (s *httpSolver) CleanUp(ctx context.Context, _ acme.Challenge) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server != nil {
		err := s.server.Shutdown(ctx)
		s.server = nil
		s.mux = nil
		return err
	}
	return nil
}

// tlsALPNSolver serves TLS-ALPN-01 challenges on port 443.
type tlsALPNSolver struct {
	mu       sync.Mutex
	listener net.Listener
}

func (s *tlsALPNSolver) Present(_ context.Context, chal acme.Challenge) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cert, err := acmez.TLSALPN01ChallengeCert(chal)
	if err != nil {
		return fmt.Errorf("challenge cert: %w", err)
	}

	tlsCfg := &tls.Config{
		NextProtos: []string{acmez.ACMETLS1Protocol},
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return cert, nil
		},
	}

	ln, err := tls.Listen("tcp", ":443", tlsCfg)
	if err != nil {
		return fmt.Errorf("listen :443: %w", err)
	}
	s.listener = ln

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				// Complete the TLS handshake then close.
				_ = conn.(*tls.Conn).Handshake()
				conn.Close()
			}()
		}
	}()

	return nil
}

func (s *tlsALPNSolver) CleanUp(_ context.Context, _ acme.Challenge) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil {
		err := s.listener.Close()
		s.listener = nil
		return err
	}
	return nil
}
