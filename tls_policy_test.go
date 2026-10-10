// Copyright 2026 - MinIO, Inc. All rights reserved.
// Use of this source code is governed by the AGPLv3
// license that can be found in the LICENSE file.

package kes

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"strings"
	"testing"

	"github.com/minio/kms-go/kes"
)

// handshake dials the server at endpoint and completes a TLS handshake
// plus one request, since TLS 1.3 reports client certificate errors
// only after the client's first read.
func handshake(endpoint string, conf *tls.Config) error {
	conn, err := tls.Dial("tcp", strings.TrimPrefix(endpoint, "https://"), conf)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err = conn.Write([]byte("GET /version HTTP/1.1\r\nHost: kes\r\n\r\n")); err != nil {
		return err
	}
	_, err = conn.Read(make([]byte, 1))
	return err
}

func TestTLSPolicy(t *testing.T) {
	ctx := testContext(t)
	srv, endpoint := startServer(ctx, nil)
	defer srv.Close()

	rootCAs := x509.NewCertPool()
	rootCAs.AddCert(defaultServerCertificate().Leaf)
	adminKey, err := kes.ParseAPIKey(defaultAPIKey)
	if err != nil {
		t.Fatal(err)
	}
	clientCert, err := kes.GenerateCertificate(adminKey)
	if err != nil {
		t.Fatal(err)
	}
	conf := func(version uint16, cert *tls.Certificate) *tls.Config {
		return &tls.Config{
			ServerName: "localhost",
			MinVersion: version,
			MaxVersion: version,
			RootCAs:    rootCAs,
			GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
				if cert == nil {
					return &tls.Certificate{}, nil
				}
				return cert, nil
			},
		}
	}

	if err := handshake(endpoint, conf(tls.VersionTLS13, &clientCert)); err != nil {
		t.Fatalf("TLS 1.3 client with a certificate was rejected: %v", err)
	}
	if err := handshake(endpoint, conf(tls.VersionTLS12, &clientCert)); err == nil {
		t.Fatal("TLS 1.2 client was accepted")
	}
	if err := handshake(endpoint, conf(tls.VersionTLS13, nil)); err == nil {
		t.Fatal("Client without a certificate was accepted")
	}
}

func TestStartRejectsOptionalClientCert(t *testing.T) {
	for _, auth := range []tls.ClientAuthType{tls.NoClientCert, tls.RequestClientCert, tls.VerifyClientCertIfGiven} {
		conf := &Config{
			Admin: defaultIdentity,
			TLS: &tls.Config{
				ClientAuth:   auth,
				Certificates: []tls.Certificate{defaultServerCertificate()},
			},
			Keys: &MemKeyStore{},
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		srv := &Server{}
		if err := srv.Start(testContext(t), ln, conf); err == nil {
			t.Errorf("Server started with client auth %v", auth)
		}
		ln.Close()
	}
}
