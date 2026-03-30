package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dzm2020/gas/pkg/lib/serializer"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func LoadJsonFile(path string, value interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return serializer.Json.Unmarshal(data, value)
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

// ViperLoadConfigWithInclude 加载配置，支持 @include 递归引用
func ViperLoadConfigWithInclude(configPath string) (*viper.Viper, error) {
	// 1. 初始化 Viper 实例
	v := viper.New()
	v.SetConfigFile(configPath)
	// 2. 读取当前配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %s failed: %w", configPath, err)
	}

	// 3. 提取 @include 字段（支持单个字符串或数组）
	var includes []string
	rawInclude := v.Get("include")
	if rawInclude != nil {
		switch val := rawInclude.(type) {
		case string:
			includes = []string{val}
		case []interface{}:
			for _, item := range val {
				if str, ok := item.(string); ok {
					includes = append(includes, str)
				}
			}
		}
	}

	// 4. 递归加载并合并所有 include 的配置
	baseDir := filepath.Dir(configPath)
	for _, incPath := range includes {
		// 路径处理：相对路径基于主配置文件所在目录
		fullIncPath := incPath
		if !filepath.IsAbs(incPath) {
			fullIncPath = filepath.Join(baseDir, incPath)
		}

		// 递归加载被引用的配置
		incV, err := ViperLoadConfigWithInclude(fullIncPath)
		if err != nil {
			return nil, fmt.Errorf("load include %s failed: %w", incPath, err)
		}

		// 合并到主 Viper（后加载覆盖先加载）
		if err := v.MergeConfigMap(incV.AllSettings()); err != nil {
			return nil, fmt.Errorf("merge include %s failed: %w", incPath, err)
		}
	}

	// 5. 可选：移除 @include 字段（避免污染最终配置）
	v.Set("include", nil)

	return v, nil
}
