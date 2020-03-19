package serfer

import (
	"encoding/base64"
	"fmt"

	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/serf"
)

// KeyResponse is an alias for serf.KeyResponse
type KeyResponse = serf.KeyResponse

// Keyring set a list of keys to be used to decode communication between nodes
func (s *Serfer) Keyring(keys ...string) error {
	if len(keys) == 0 {
		return fmt.Errorf("Keyring should contain at least one key")
	}
	kr, err := keyring(keys...)
	if err != nil {
		return err
	}

	// But if there is a SecretKey, add it to the Keyring (if isn't there already)
	if len(s.Conf.MemberlistConfig.SecretKey) != 0 {
		// AddKey don't add an existing key. They won't be repeated
		kr.AddKey(s.Conf.MemberlistConfig.SecretKey)
		kr.UseKey(s.Conf.MemberlistConfig.SecretKey)
	} else {
		s.Conf.MemberlistConfig.SecretKey = kr.GetPrimaryKey()
	}
	s.Conf.MemberlistConfig.Keyring = kr
	return nil
}

func keyring(keys ...string) (*memberlist.Keyring, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("keyring should contain at least one key")
	}
	decodedKeys := make([][]byte, len(keys))
	for i, k := range keys {
		decodedKey, err := base64.StdEncoding.DecodeString(k)
		if err != nil {
			return nil, fmt.Errorf("can't decode key number %d. %s", i, err)
		}
		if l := len(decodedKey); l != 16 && l != 24 && l != 32 {
			return nil, fmt.Errorf("incorrect size of key number %d, the size is %d and should be 16, 24 or 32 bytes", i, l)
		}
		decodedKeys[i] = decodedKey
	}
	return memberlist.NewKeyring(decodedKeys, decodedKeys[0])
}

// GetKeys return all the keys used by the cluster.
func (s *Serfer) GetKeys() [][]byte {
	// if s.Conf.MemberlistConfig.EncryptionEnabled()
	if len(s.Conf.MemberlistConfig.SecretKey) != 0 {
		keys := make([][]byte, 1)
		keys[0] = s.Conf.MemberlistConfig.SecretKey
		return keys
	}
	return s.Conf.MemberlistConfig.Keyring.GetKeys()
}

func emptyKeyResponse() *KeyResponse {
	return &KeyResponse{
		NumNodes: 1,
		NumResp:  1,
		NumErr:   0,
	}
}

// InstallKey send a query to install a new key on all the members. If the
// cluster does not exists (not join to a cluster yet), will add the key
func (s *Serfer) InstallKey(key string) (*KeyResponse, error) {
	if s.cluster != nil {
		manager := s.cluster.KeyManager()
		return manager.InstallKey(key)
	}

	return emptyKeyResponse(), s.Conf.MemberlistConfig.Keyring.AddKey([]byte(key))
}

// UseKey sends a query to all members to switch to the given primary key. If
// the cluster does not exists will switch to the given primary key
func (s *Serfer) UseKey(key string) (*KeyResponse, error) {
	if s.cluster != nil {
		manager := s.cluster.KeyManager()
		return manager.UseKey(key)
	}

	return emptyKeyResponse(), s.Conf.MemberlistConfig.Keyring.UseKey([]byte(key))
}

// RemoveKey sends a query to all members to remove a key from the keyring
func (s *Serfer) RemoveKey(key string) (*KeyResponse, error) {
	if s.cluster != nil {
		manager := s.cluster.KeyManager()
		return manager.RemoveKey(key)
	}

	return emptyKeyResponse(), s.Conf.MemberlistConfig.Keyring.RemoveKey([]byte(key))
}

// ListKeys sends a query to all members to return a list of their keys
func (s *Serfer) ListKeys() (*KeyResponse, error) {
	if s.cluster != nil {
		manager := s.cluster.KeyManager()
		return manager.ListKeys()
	}

	resp := emptyKeyResponse()
	for _, k := range s.Conf.MemberlistConfig.Keyring.GetKeys() {
		resp.Keys[string(k)] = 1
	}

	return resp, nil
}
