package image

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/util/wait"
	"math/big"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/registry"
	_ "github.com/docker/distribution/registry/storage/driver/filesystem" // Driver for persisting docker image data to the filesystem.
	_ "github.com/docker/distribution/registry/storage/driver/inmemory"   // Driver for keeping docker image data in memory.
	"github.com/phayes/freeport"
)

// RunDockerRegistry runs a docker registry on an available port and returns its host string if successful, otherwise it returns an error.
// If the rootDir argument isn't empty, the registry is configured to use this as the root directory for persisting image data to the filesystem.
// If the rootDir argument is empty, the registry is configured to keep image data in memory.
func RunDockerRegistry(ctx context.Context, rootDir string) (string, string, error) {
	dockerPort, err := freeport.GetFreePort()
	if err != nil {
		return "", "", err
	}
	host := fmt.Sprintf("localhost:%d", dockerPort)
	certPool := x509.NewCertPool()

	cafile, err := ioutil.TempFile("", "ca")
	if err != nil {
		return "", "", err
	}
	certfile, err := ioutil.TempFile("", "cert")
	if err != nil {
		return "", "", err
	}
	keyfile, err := ioutil.TempFile("", "key")
	if err != nil {
		return "", "", err
	}
	if err := GenerateCerts(cafile, certfile, keyfile, certPool); err != nil {
		return "", "", err
	}
	if err := cafile.Close(); err != nil {
		return "", "", err
	}
	if err := certfile.Close(); err != nil {
		return "", "", err
	}
	if err := keyfile.Close(); err != nil {
		return "", "", err
	}

	config := &configuration.Configuration{}
	config.HTTP.Addr = host
	config.HTTP.TLS.Certificate = certfile.Name()
	config.HTTP.TLS.Key = keyfile.Name()
	config.Log.Level = "debug"

	if rootDir != "" {
		config.Storage = map[string]configuration.Parameters{"filesystem": map[string]interface{}{
			"rootdirectory": rootDir,
		}}
	} else {
		config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	}
	config.HTTP.DrainTimeout = 2 * time.Second

	dockerRegistry, err := registry.NewRegistry(ctx, config)
	if err != nil {
		return "", "", err
	}

	go func() {
		defer func() {
			os.Remove(cafile.Name())
			os.Remove(certfile.Name())
			os.Remove(keyfile.Name())
		}()
		if err := dockerRegistry.ListenAndServe(); err != nil {
			panic(fmt.Errorf("docker registry stopped listening: %v", err))
		}
	}()

	err = wait.Poll(100*time.Millisecond, 10*time.Second, func() (done bool, err error) {
		tr := &http.Transport{TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			RootCAs:            certPool,
		}}
		client := &http.Client{Transport: tr}
		r, err := client.Get("https://"+host+"/v2/")
		if err != nil {
			return false, nil
		}
		if r.StatusCode == http.StatusOK {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return "", "", err
	}

	// Return the registry host string
	return host, cafile.Name(), nil
}

func certToPem(der []byte) ([]byte, error) {
	out := &bytes.Buffer{}
	if err := pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func keyToPem(key *ecdsa.PrivateKey) ([]byte, error) {
	b, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf( "unable to marshal private key: %v", err)
	}
	out := &bytes.Buffer{}
	if err := pem.Encode(out, &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func GenerateCerts(caWriter, certWriter, keyWriter io.Writer, pool *x509.CertPool ) error {
	priv, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		return err
	}
	ca := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"test ca"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA: true,
	}
	cert := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"test cert"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses: []net.IP{
			net.ParseIP("127.0.0.1"),
			net.ParseIP("::1"),
		},
		DNSNames: []string{"localhost"},
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, &ca, &ca, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	caFile, err := certToPem(caBytes)
	if err != nil {
		return err
	}
	if _, err := caWriter.Write(caFile); err != nil {
		return err
	}
	pool.AppendCertsFromPEM(caFile)

	certBytes, err := x509.CreateCertificate(rand.Reader, &cert, &ca, &priv.PublicKey, priv)
	if err != nil {
		return err
	}
	certFile, err := certToPem(certBytes)
	if err != nil {
		return err
	}
	if _, err := certWriter.Write(certFile); err != nil {
		return err
	}

	keyFile, err := keyToPem(priv)
	if err != nil {
		return err
	}
	if _, err := keyWriter.Write(keyFile); err != nil {
		return err
	}
	return nil
}
