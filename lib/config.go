package wake_on_wan_relay_config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

const VERSION = "0.1.1"

func GetProjectRoot() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatalf("Something went so wrong that it wasn't possible to fetch this filetype")
	}

	return filepath.Dir(filepath.Dir(filename))

}

type Context struct {
	MacAddress   string `json:"macAddress"`
	PrivateKey   string `json:"privateKey"`
	PublicKey    string `json:"publicKey"`
	ExternalAddr string `json:"externalAddress"`
	Version      string
}

func GetContext() *Context {
	context := new(Context)

	contextRaw, err := os.ReadFile(GetProjectRoot() + "/config.json")
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
