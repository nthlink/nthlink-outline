package outline

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/Jigsaw-Code/outline-sdk/network"
	"github.com/Jigsaw-Code/outline-sdk/network/lwip2transport"
	"github.com/Jigsaw-Code/outline-sdk/transport"
	"github.com/Jigsaw-Code/outline-sdk/transport/shadowsocks"
	"github.com/Jigsaw-Code/outline-sdk/x/connectivity"
)

type PacketWriter interface {
	WritePacket(data []byte)
}

type SocketProtector interface {
	Protect(fd int) bool
}

var ipDevice network.IPDevice

func WritePacket(data []byte) {
	_, err := ipDevice.Write(data)
	if err != nil {
		log.Printf("write {} bytes IP packet to device failed: %v", len(data), err)
	}
}

func SetNonblock(fd int, nonblocking bool) bool {
	err := syscall.SetNonblock(fd, nonblocking)
	if err != nil {
		return false
	}
	return true
}

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

func Start(packetWriter PacketWriter, socketProtector SocketProtector, addr, cipher, secret, prefix string) error {
	cryptoKey, err := shadowsocks.NewEncryptionKey(cipher, secret)
	if err != nil {
		return err
	}
	var dialer net.Dialer
	if socketProtector != nil {
		dialer = net.Dialer{
			Control: func(network, address string, c syscall.RawConn) error {
				return c.Control(func(fd uintptr) {
					socketProtector.Protect(int(fd))
				})
			},
		}
	} else {
		dialer = net.Dialer{}
	}
	streamDialer, err := shadowsocks.NewStreamDialer(&transport.TCPEndpoint{Dialer: dialer, Address: addr}, cryptoKey)
	if err != nil {
		return err
	}
	// More about prefix: https://www.reddit.com/r/outlinevpn/wiki/index/prefixing/
	if len(prefix) > 0 {
		prefix, err := parseStringPrefix(prefix)
		if err != nil {
			return err
		}
		streamDialer.SaltGenerator = shadowsocks.NewPrefixSaltGenerator(prefix)
	}
	packetListener, err := shadowsocks.NewPacketListener(transport.UDPEndpoint{Dialer: dialer, Address: addr}, cryptoKey)
	if err != nil {
		return err
	}
	// TODO Support dnstruncate packet proxy in case the server doesn't support UDP,
	// server connectivity can be tested by `TestConnectivity`.
	packetProxy, err := network.NewPacketProxyFromPacketListener(packetListener)
	if err != nil {
		return err
	}
	ipDevice, err = lwip2transport.ConfigureDevice(streamDialer, packetProxy)
	if err != nil {
		return err
	}
	go func() {
		buf := make([]byte, ipDevice.MTU())
		for {
			n, err := ipDevice.Read(buf)
			if err != nil {
				log.Printf("read packet from IP device failed: %v", err)
				break
			}
			packetWriter.WritePacket(buf[:n])
		}
	}()
	return nil
}

func Stop() error {
	if ipDevice != nil {
		return ipDevice.Close()
	}
	return nil
}

// Tests connectivity of the given Shadowsocks server via TCP/UDP DNS query.
//
// `proto`: tcp or udp
// `resolver`: the DNS server used in test, for example: 1.1.1.1:53
// `resolveDomain`: the query domain name, for example: www.google.com
func TestConnectivity(addr, cipher, secret, prefix, proto, resolver, resolveDomain string) error {
	cryptoKey, err := shadowsocks.NewEncryptionKey(cipher, secret)
	if err != nil {
		return err
	}
	proto = strings.TrimSpace(proto)
	proto = strings.ToLower(proto)
	var duration time.Duration
	var testErr error
	switch proto {
	case "tcp":
		streamDialer, err := shadowsocks.NewStreamDialer(&transport.TCPEndpoint{Address: addr}, cryptoKey)
		if err != nil {
			return err
		}
		if len(prefix) > 0 {
			prefix, err := parseStringPrefix(prefix)
			if err != nil {
				return err
			}
			streamDialer.SaltGenerator = shadowsocks.NewPrefixSaltGenerator(prefix)
		}
		streamEndpoint := &transport.StreamDialerEndpoint{Dialer: streamDialer, Address: resolver}
		duration, testErr = connectivity.TestResolverStreamConnectivity(context.Background(), streamEndpoint, resolveDomain)
	case "udp":
		packetListener, err := shadowsocks.NewPacketListener(transport.UDPEndpoint{Dialer: net.Dialer{}, Address: addr}, cryptoKey)
		if err != nil {
			return err
		}
		packetDialer := transport.PacketListenerDialer{Listener: packetListener}
		packetEndpoint := &transport.PacketDialerEndpoint{Dialer: packetDialer, Address: resolver}
		duration, testErr = connectivity.TestResolverPacketConnectivity(context.Background(), packetEndpoint, resolveDomain)
	default:
		return errors.New("unknown protocol")
	}
	if testErr == nil {
		log.Printf("connectivity test ok in %v", duration.String())
		return nil
	}
	return testErr
}
