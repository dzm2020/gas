package lib

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func LoadJsonFile(path string, value interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return Json.Unmarshal(data, value)
}

func LoadYamlFile(path string, value interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, value)
}

// LoadConfigFile 根据文件扩展名自动检测格式并加载配置文件
// 支持 .json, .yaml, .yml 格式
func LoadConfigFile(path string, value interface{}) error {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return LoadYamlFile(path, value)
	case ".json":
		return LoadJsonFile(path, value)
	default:
		// 默认尝试JSON格式
		return LoadJsonFile(path, value)
	}
}
