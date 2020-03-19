package serfer_test

import (
	"encoding/base64"
	"testing"

	"github.com/johandry/serfer"
)

func TestKeyRing(t *testing.T) {
	// echo -n "1234567890123456" | base64 # => "MTIzNDU2Nzg5MDEyMzQ1Ng=="
	pk := "1234567890123456"
	pkEncoded := base64.StdEncoding.EncodeToString([]byte(pk))
	s, err := serfer.NewSecure([]string{pkEncoded}, nil)
	if err != nil {
		t.Errorf("Expected no error creating the Serfer, but got %s", err)
	}
	if string(s.Conf.MemberlistConfig.SecretKey) != pk {
		t.Errorf("Expected SecretKey to be %q, but got %q", pk, s.Conf.MemberlistConfig.SecretKey)
	}
	// echo -n "abcdefghijklmnop" | base64 # => "YWJjZGVmZ2hpamtsbW5vcA=="
	// echo -n "qrstuvwxyzabcdef" | base64 # => "cXJzdHV2d3h5emFiY2RlZg=="
	k1 := "abcdefghijklmnop"
	k1Encoded := base64.StdEncoding.EncodeToString([]byte(k1))
	k2 := "qrstuvwxyzabcdef"
	k2Encoded := base64.StdEncoding.EncodeToString([]byte(k2))
	err = s.Keyring(k1Encoded, k2Encoded)
	if err != nil {
		t.Errorf("Expected no error creating the Keyring, but got %s", err)
	}

	actualPK := s.Conf.MemberlistConfig.Keyring.GetPrimaryKey()
	if string(actualPK) != pk {
		t.Errorf("Expected Primary Key to be %q, but got %q", pk, actualPK)
	}

	actualKeys := s.Conf.MemberlistConfig.Keyring.GetKeys()
	if string(actualKeys[0]) != pk {
		t.Errorf("Expected first key to be %q, but got %q", pk, actualKeys[0])
	}
	if !(string(actualKeys[1]) == k1 || string(actualKeys[1]) == k2) {
		t.Errorf("Expected second key to be %q or %q, but got %q", k1, k2, actualKeys[1])
	}
}
