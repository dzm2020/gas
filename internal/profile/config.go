package profile

import (
	"github.com/dzm2020/gas/pkg/glog"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	vp = viper.New()
)

func Init(path string) {
	var err error
	defer func() {
		if err != nil {
			glog.Fatal("read config", zap.Error(err))
		}
	}()

	vp.SetConfigFile(path)
	vp.SetConfigType("yaml")

	if err = vp.ReadInConfig(); err != nil {
		return
	}
}

func Get(key string, cfg interface{}) error {
	return vp.UnmarshalKey(key, cfg)
}
