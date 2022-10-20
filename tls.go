package readline

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	tls "github.com/refraction-networking/utls"
	"io/ioutil"
	"math/big"
	"net"
)

func wrapTLSClientConn(c net.Conn, tlsConfig *tls.Config) (out net.Conn) {
	if tlsConfig == nil {
		tlsConfig, _ = newClientTLSConfig("", "", "", "")
	}
	out = tls.Client(c, tlsConfig)
	return
}

func wrapTLSServerConn(c net.Conn, tlsConfig *tls.Config) (out net.Conn) {
	if tlsConfig == nil {
		tlsConfig, _ = newServerTLSConfig("", "", "")
	}

	out = tls.Server(c, tlsConfig)
	return
}

func newCustomTLSKeyPair(certfile, keyfile string) (*tls.Certificate, error) {
	tlsCert, err := tls.LoadX509KeyPair(certfile, keyfile)
	if err != nil {
		return nil, err
	}
	return &tlsCert, nil
}

func newRandomTLSKeyPair() *tls.Certificate {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		&key.PublicKey,
		key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tlsCert
}

// Only support one ca file to add
func newCertPool(caPath string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()

	caCrt, err := ioutil.ReadFile(caPath)
	if err != nil {
		return nil, err
	}

	pool.AppendCertsFromPEM(caCrt)

	return pool, nil
}

func newServerTLSConfig(certPath, keyPath, caPath string) (*tls.Config, error) {
	var base = &tls.Config{}

	if certPath == "" || keyPath == "" {
		// server will generate tls conf by itself
		cert := newRandomTLSKeyPair()
		base.Certificates = []tls.Certificate{*cert}
	} else {
		cert, err := newCustomTLSKeyPair(certPath, keyPath)
		if err != nil {
			return nil, err
		}

		base.Certificates = []tls.Certificate{*cert}
	}

	if caPath != "" {
		pool, err := newCertPool(caPath)
		if err != nil {
			return nil, err
		}

		base.ClientAuth = tls.RequireAndVerifyClientCert
		base.ClientCAs = pool
	}

	return base, nil
}

func newClientTLSConfig(certPath, keyPath, caPath, serverName string) (*tls.Config, error) {
	var base = &tls.Config{}
	//certPath = "cert.pem"
	//keyPath = "cert.key"
	if certPath == "" || keyPath == "" {
		// client will not generate tls conf by itself
	} else {
		cert, err := newCustomTLSKeyPair(certPath, keyPath)
		if err != nil {
			return nil, err
		}
		base.Certificates = []tls.Certificate{*cert}
	}

	if caPath != "" {
		pool, err := newCertPool(caPath)
		if err != nil {
			return nil, err
		}

		base.RootCAs = pool
		base.ServerName = serverName
		base.InsecureSkipVerify = true
	} else {
		base.ServerName = serverName
		base.InsecureSkipVerify = true
	}
	//base

	base.CipherSuites = []uint16{4865, 4866, 4867, 49195, 49199, 49196, 49200, 52393, 52392, 49171, 49172, 156, 157, 47, 53}
	base.CurvePreferences = []tls.CurveID{tls.X25519, tls.CurveP256, tls.CurveP384}
	return base, nil
}
