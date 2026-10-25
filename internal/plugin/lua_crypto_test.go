package plugin

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestCryptoBase64RoundTrip(t *testing.T) {
	L := NewSandbox()
	defer L.Close()
	err := L.DoString(`
		enc = crypto.base64_encode("-----BEGIN PUBLIC KEY-----\n")
		dec = crypto.base64_decode(enc)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if L.GetGlobal("enc").String() != "LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0K" || L.GetGlobal("dec").String() != "-----BEGIN PUBLIC KEY-----\n" {
		t.Fatalf("enc %v dec %q", L.GetGlobal("enc"), L.GetGlobal("dec").String())
	}
}

func TestCryptoBase64DecodeAcceptsUnpaddedAndSpacedInput(t *testing.T) {
	L := NewSandbox()
	defer L.Close()
	err := L.DoString(`
		unpadded = crypto.base64_decode("aGk")
		spaced = crypto.base64_decode("  aGk=\n")
		bad, msg = crypto.base64_decode("!!")
	`)
	if err != nil {
		t.Fatal(err)
	}
	if got := L.GetGlobal("unpadded").String(); got != "hi" {
		t.Errorf("unpadded = %q", got)
	}
	if got := L.GetGlobal("spaced").String(); got != "hi" {
		t.Errorf("spaced = %q", got)
	}
	if L.GetGlobal("bad") != lua.LNil || L.GetGlobal("msg") == lua.LNil {
		t.Error("invalid base64 should return nil and an error")
	}
}
