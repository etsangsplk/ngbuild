package core

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mitchellh/mapstructure"
)

type config map[string]interface{}

func expandHome(dir string) string {
	return strings.Replace(dir, "~/", os.Getenv("HOME")+"/", 1)
}

var (
	configBaseDir   = ""
	configCacheLock sync.RWMutex
	configCache     = make(map[string]config)

	configDefaults = map[string]interface{}{
		"buildLocation":     os.TempDir(),
		"artifactsLocation": os.TempDir(),
		"cacheDirectory":    expandHome("~/.cache/ngbuild/"),
	}
)

func loadConfig(path string) (config, error) {
	configCacheLock.RLock()
	var err error
	if configBaseDir == "" {
		configBaseDir, err = getNGBuildDirectory()
		if err != nil {
			configBaseDir = ""
		}
	}

	if c, ok := configCache[path]; ok {
		configCacheLock.RUnlock()
		return c, nil
	}
	configCacheLock.RUnlock()

	raw, err := ioutil.ReadFile(filepath.Join(configBaseDir, path))
	if err != nil {
		return nil, err
	}

	var conf interface{}
	err = json.Unmarshal(raw, &conf)
	if err != nil {
		return nil, err
	}

	configCacheLock.Lock()
	defer configCacheLock.Unlock()
	configCache[path] = (config)(conf.(map[string]interface{}))

	return configCache[path], nil
}

func loadMasterConfig() (config, error) {
	return loadConfig("ngbuild.json")
}

func loadAppConfig(appname string) (config, error) {
	return loadConfig(filepath.Join("apps", appname, "config.json"))
}

// for the given config, apply it's data onto the given structure s
func applyConfig(appname string, s interface{}) error {
	if err := mapstructure.Decode(configDefaults, s); err != nil {
		return err
	}

	master, err := loadMasterConfig()
	if err != nil {
		return err
	}

	if err = mapstructure.Decode(master, s); err != nil {
		return err
	}

	if appname != "" {
		appconfig, err := loadAppConfig(appname)
		if err != nil {
			return err
		}

		return mapstructure.Decode(appconfig, s)
	}
	return nil
}

func getIntegrationConfig(conf config, integrationName string) config {
	if integrations, ok := conf["Integrations"]; ok == false {
		return nil
	} else if integrationsMap, ok := integrations.(map[string]interface{}); ok == false {
		return nil
	} else if integration, ok := integrationsMap[integrationName]; ok == false {
		return nil
	} else if integrationMap, ok := integration.(map[string]interface{}); ok == false {
		return nil
	} else {
		return (config)(integrationMap)
	}
}

// Like applyConfig, but will look for configs in /integrations/integrationName/
func applyIntegrationConfig(appname, integrationName string, s interface{}) error {
	master, err := loadMasterConfig()
	if err != nil {
		return err
	}

	if masterIntegration := getIntegrationConfig(master, integrationName); masterIntegration != nil {
		if err = mapstructure.Decode(masterIntegration, s); err != nil {
			return err
		}
	}

	if appname != "" {
		appconfig, err := loadAppConfig(appname)
		if err != nil {
			return err
		}

		if appIntegration := getIntegrationConfig(appconfig, integrationName); appIntegration != nil {
			if err = mapstructure.Decode(appIntegration, s); err != nil {
				return err
			}
		}
	}

	return nil
}
