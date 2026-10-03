package wake_on_wan_relay_config

import (
	"encoding/json"
	"log"
	"os"
)

const VERSION = "0.0.1"

type Context struct {
	MacAddress string `json:"macAddress"`
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
	Version    string
}

func GetContext() *Context {
	context := new(Context)

	contextRaw, err := os.ReadFile("../config.json")
	if err != nil {
		log.Fatalf("Failed to read config")
	}

	err = json.Unmarshal(contextRaw, context)
	if err != nil {
		log.Fatalf("Failed to unmarshal config. Check if it is valid json")
	}

	context.Version = VERSION

	return context
}
