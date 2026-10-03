package wake_on_wan_relay_config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var VERSION = "0.1.3"

func GetProjectRoot() string {
	if root := os.Getenv("PROJECT_ROOT"); root != "" {
		return root
	}
	if pwd, err := os.Getwd(); err == nil {
		if _, err := os.Stat(filepath.Join(pwd, "config.json")); err == nil {
			return pwd
		}
	}
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		return filepath.Dir(filepath.Dir(filename))
	}
	pwd, _ := os.Getwd()
	return pwd
}

type Context struct {
	MacAddress   string `json:"macAddress"`
	PrivateKey   string `json:"privateKey"`
	PublicKey    string `json:"publicKey"`
	ExternalAddr string `json:"externalAddress"`
	CertPath     string `json:"certPath"`
	KeyPath      string `json:"keyPath"`
	Version      string
}

func GetContext() *Context {
	context := new(Context)

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		if _, err := os.Stat("config.json"); err == nil {
			configPath = "config.json"
		} else {
			configPath = filepath.Join(GetProjectRoot(), "config.json")
		}
	}

	contextRaw, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("Failed to read config from %s: %v", configPath, err)
	}

	err = json.Unmarshal(contextRaw, context)
	if err != nil {
		log.Fatalf("Failed to unmarshal config. Check if it is valid json")
	}

	context.Version = strings.TrimPrefix(VERSION, "v")

	return context
}
