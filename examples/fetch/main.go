package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/Jigsaw-Code/outline-sdk/transport"
	"github.com/Jigsaw-Code/outline-sdk/transport/shadowsocks"
)

func parseStringPrefix(utf8Str string) ([]byte, error) {
	runes := []rune(utf8Str)
	rawBytes := make([]byte, len(runes))
	for i, r := range runes {
		if (r & 0xFF) != r {
			return nil, fmt.Errorf("character out of range: %d", r)
		}
		rawBytes[i] = byte(r)
	}
	return rawBytes, nil
}

func fetchViaProxy(addr, cipher, secret, prefix, url string) error {
	cryptoKey, err := shadowsocks.NewEncryptionKey(cipher, secret)
	if err != nil {
		return err
	}
	dialer, err := shadowsocks.NewStreamDialer(&transport.TCPEndpoint{Address: addr}, cryptoKey)
	if err != nil {
		return err
	}
	if len(prefix) > 0 {
		prefix, err := parseStringPrefix(prefix)
		if err != nil {
			return err
		}
		dialer.SaltGenerator = shadowsocks.NewPrefixSaltGenerator(prefix)
	}
	dialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
		if !strings.HasPrefix(network, "tcp") {
			return nil, fmt.Errorf("protocol not supported: %v", network)
		}
		return dialer.Dial(ctx, addr)
	}
	httpClient := &http.Client{Transport: &http.Transport{DialContext: dialContext}}
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(os.Stdout, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

// Must run with proper arguments, for example:
//
// go run main.go 1.2.3.4:1234 chacha20-ietf-poly1305 Secret0 Prefix0 https://www.google.com
func main() {
	addr := os.Args[1]
	cipher := os.Args[2]
	secret := os.Args[3]
	prefix := os.Args[4]
	url := os.Args[5]
	log.Printf("address %v", addr)
	log.Printf("cipher %v", cipher)
	log.Printf("secret %v", secret)
	log.Printf("prefix %v", prefix)
	log.Printf("url: %v", url)
	fetchViaProxy(addr, cipher, secret, prefix, url)
}
