package util

import (
	"crypto/md5"
	"encoding/hex"
)

func createHash(key string) string {
	hasher := md5.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))
}

// Encrypt will encrypt data with passphrase
func Encrypt(data []byte, passphrase string) ([]byte, error) {
	return data, nil
	// block, err := aes.NewCipher([]byte(createHash(passphrase)))
	// if err != nil {
	// 	return nil, err
	// }
	// gcm, err := cipher.NewGCM(block)
	// if err != nil {
	// 	return nil, err
	// }
	// nonce := make([]byte, gcm.NonceSize())
	// if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
	// 	return nil, err
	// }
	// ciphertext := gcm.Seal(nonce, nonce, data, nil)
	// return ciphertext, nil
}

// Decrypt will decrypt data with passphrase
func Decrypt(data []byte, passphrase string) ([]byte, error) {
	return data, nil
	// key := []byte(createHash(passphrase))
	// block, err := aes.NewCipher(key)
	// if err != nil {
	// 	return nil, err
	// }
	// gcm, err := cipher.NewGCM(block)
	// if err != nil {
	// 	return nil, err
	// }
	// nonceSize := gcm.NonceSize()
	// nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	// plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	// if err != nil {
	// 	return nil, err
	// }
	// return plaintext, nil
}
