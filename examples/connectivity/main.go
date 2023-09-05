package main

import (
	outline "github.com/nthlink/nthlink-outline"
	"log"
	"os"
)

// Must run with proper arguments, for example:
//
// go run main.go 1.2.3.4:1234 chacha20-ietf-poly1305 Secret0 Prefix0 tcp 1.1.1.1:53 www.google.com
func main() {
	addr := os.Args[1]
	cipher := os.Args[2]
	secret := os.Args[3]
	prefix := os.Args[4]
	proto := os.Args[5]
	resolver := os.Args[6]
	resolveDomain := os.Args[7]
	log.Printf("address %v", addr)
	log.Printf("cipher %v", cipher)
	log.Printf("secret %v", secret)
	log.Printf("prefix %v", prefix)
	log.Printf("proto %v", proto)
	log.Printf("resolver %v", resolver)
	log.Printf("resolveDomain %v", resolveDomain)
	err := outline.TestConnectivity(addr, cipher, secret, prefix, proto, resolver, resolveDomain)
	if err != nil {
		log.Printf("Test connectivity failed: %v", err)
	}
}
