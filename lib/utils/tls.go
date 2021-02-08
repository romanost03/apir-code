package utils

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log"

	"google.golang.org/grpc/credentials"
)

// WARNING: DO NOT USE THESE KEYS IN A REAL DEPLOYMENT!

var ServerPublicKeys = [...]string{
	`-----BEGIN CERTIFICATE-----
MIIBKDCB26ADAgECAhEA2rLIr85wuhR2LiFxsoEXxTAFBgMrZXAwEjEQMA4GA1UE
ChMHQWNtZSBDbzAeFw0yMTAyMDEwODU2MzVaFw0yMjAyMDEwODU2MzVaMBIxEDAO
BgNVBAoTB0FjbWUgQ28wKjAFBgMrZXADIQCS1XsHHHVbLbrBZX1wUZrMSDS8RVmT
xOHl5Op4oBu5l6NGMEQwDgYDVR0PAQH/BAQDAgeAMBMGA1UdJQQMMAoGCCsGAQUF
BwMBMAwGA1UdEwEB/wQCMAAwDwYDVR0RBAgwBocEfwAAATAFBgMrZXADQQCyOVP+
otDbpiB+f8ETaojdqhSv2+wBP7fFYA8q4F5fgQrJyTJgRZAcgqfhKy7jE+tWC9ba
rzPH39t58Gq5I4AI
-----END CERTIFICATE-----`,

	`-----BEGIN CERTIFICATE-----
MIIBKDCB26ADAgECAhEA0SLvmCOobA+Vtxcw//FD+jAFBgMrZXAwEjEQMA4GA1UE
ChMHQWNtZSBDbzAeFw0yMTAyMDEwODU5MjhaFw0yMjAyMDEwODU5MjhaMBIxEDAO
BgNVBAoTB0FjbWUgQ28wKjAFBgMrZXADIQAtDvVlqbeUjm6IOjrqpAHmzx05fjnn
3BWZT2t6NGtkeaNGMEQwDgYDVR0PAQH/BAQDAgeAMBMGA1UdJQQMMAoGCCsGAQUF
BwMBMAwGA1UdEwEB/wQCMAAwDwYDVR0RBAgwBocEfwAAATAFBgMrZXADQQBFTN6h
e0wZn6QAcAbs6aDGsWsBjT9lMETgFcP0qGSNIqc36Zn+lTY6FUAdP+/hv7bd1+Vo
cgTGK0dBHeAHmjsL
-----END CERTIFICATE-----`,

	`-----BEGIN CERTIFICATE-----
MIIBKDCB26ADAgECAhEAnqqQT5AodfGTnIEvLHKu/TAFBgMrZXAwEjEQMA4GA1UE
ChMHQWNtZSBDbzAeFw0yMTAyMDEwOTAxMDNaFw0yMjAyMDEwOTAxMDNaMBIxEDAO
BgNVBAoTB0FjbWUgQ28wKjAFBgMrZXADIQDH5vAABH/UOaNyXLMWZPT8m0VyxhLR
I1C0rzLcm63viKNGMEQwDgYDVR0PAQH/BAQDAgeAMBMGA1UdJQQMMAoGCCsGAQUF
BwMBMAwGA1UdEwEB/wQCMAAwDwYDVR0RBAgwBocEfwAAATAFBgMrZXADQQBFVB8Y
W9KXfYp6Snm+TTR0Lgwtnr2Yg7KEM4oaxSJ1LtSyFGuxINpSfoeafjQyCQ477WZF
2kMQ2ktNQ9U8SYsP
-----END CERTIFICATE-----`}

var serverSecretKeys = [...]string{
	`-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIJJUrGrPnC3oXsm+C9wdyuSgmDsUxF1BqBOxsARYQzzb
-----END PRIVATE KEY-----`,

	`-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIOWEOiyfclF0OXWtJmnTKbMJTXUglziqwl3+d2spXXyP
-----END PRIVATE KEY-----`,

	`-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIMaliVpywAKJHIoJaQiyJ3vnGp/66lKzFCX4AMUB8D2G
-----END PRIVATE KEY-----`}

// ServerCertificates holds the certificates for the servers
var ServerCertificates []tls.Certificate

func init() {
	numServers := 3

	ServerCertificates = make([]tls.Certificate, numServers)

	var err error
	for i := range ServerCertificates {
		ServerCertificates[i], err = tls.X509KeyPair(
			[]byte(ServerPublicKeys[i]),
			[]byte(serverSecretKeys[i]))
		if err != nil {
			log.Fatalf("could not load certficate #%v %v", i, err)
		}
	}
}

func LoadServersCertificates() (credentials.TransportCredentials, error) {
	cp := x509.NewCertPool()
	for _, cert := range ServerPublicKeys {
		if !cp.AppendCertsFromPEM([]byte(cert)) {
			return nil, errors.New("credentials: failed to append certificates")
		}
	}
	creds := credentials.NewClientTLSFromCert(cp, "127.0.0.1")

	return creds, nil
}
