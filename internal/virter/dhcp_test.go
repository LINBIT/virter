package virter

import (
	"net"
	"testing"
)

func TestIPToID(t *testing.T) {
	_, ipnet, err := net.ParseCIDR("192.168.0.0/24")
	id, err := ipToID(*ipnet, net.ParseIP("192.168.0.13"))
	if err != nil {
		t.Fatal(err)
	}

	if id != 13 {
		t.Fatalf("Expected ID 13, actual: %d", id)
	}
}

func TestIPToIDWrongNet(t *testing.T) {
	_, ipnet, err := net.ParseCIDR("10.0.0.0/24")
	id, err := ipToID(*ipnet, net.ParseIP("192.168.0.13"))
	if err == nil {
		t.Fatal("ip not in networ mask")
	}

	if id != 0 {
		t.Fatal("id should be zero + err")
	}
}

func TestCidr(t *testing.T) {
	if c := cidr(net.ParseIP("255.255.255.0")); c != 24 {
		t.Fatalf("cidr expected 24 got: %d", c)
	}

	if c := cidr(net.ParseIP("255.255.0.0")); c != 16 {
		t.Fatalf("cidr expected 24 got: %d", c)
	}
}
