package keyring

import "github.com/zalando/go-keyring"

const service = "velane"

// SaveAPIKey stores the API key in the system keychain.
func SaveAPIKey(key string) error {
	return keyring.Set(service, "api_key", key)
}

// LoadAPIKey retrieves the API key from the system keychain.
func LoadAPIKey() (string, error) {
	return keyring.Get(service, "api_key")
}
