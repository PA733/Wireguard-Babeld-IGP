package service

import (
	"crypto/rand"
	"encoding/base64"

	"golang.org/x/crypto/curve25519"
)

type KeyService struct{}

func NewKeyService() *KeyService {
	return &KeyService{}
}

// 生成公私钥对
func (k *KeyService) GenerateKeyPair() (privateKey, publicKey string, err error) {
	privateKeyBytes := make([]byte, 32)
	_, err = rand.Read(privateKeyBytes)
	if err != nil {
		return "", "", err
	}
	privateKeyBytes[0] &= 248
	privateKeyBytes[31] &= 127
	privateKeyBytes[31] |= 64

	publicKeyBytes, err := curve25519.X25519(privateKeyBytes, curve25519.Basepoint)
	if err != nil {
		return "", "", err
	}

	return base64.StdEncoding.EncodeToString(privateKeyBytes), base64.StdEncoding.EncodeToString(publicKeyBytes), nil
}
