package profile

import (
	"context"

	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/cluster"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/component"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const Name = "profile"

var _ iface.IProfile = (*Profile)(nil)

// Profile 配置加载组件：持有一个 viper 实例
type Profile struct {
	component.BaseComponent[iface.INode]
	path string
	vp   *viper.Viper
}

// New 创建配置组件，path 为配置文件路径。
func New(path string) *Profile {
	return &Profile{path: path}
}

func (c *Profile) Name() string {
	return Name
}

func (c *Profile) Start(ctx context.Context, node iface.INode) error {
	c.vp = viper.New()
	c.vp.SetConfigFile(c.path)
	if err := c.vp.ReadInConfig(); err != nil {
		return err
	}
	return c.vp.UnmarshalKey("node", node.Info())
}

func (c *Profile) Get(key string, cfg interface{}) error {
	return c.vp.UnmarshalKey(key, cfg)
}

func (c *Profile) IsSingleNodeMode() bool {
	return c.vp.GetBool("single-node")
}

func (c *Profile) GetCluster() *cluster.Config {
	conf := cluster.DefaultConfig()
	if err := c.Get("cluster", conf); err != nil {
		glog.Fatal("get cluster config", zap.Error(err))
	}
	return conf
}

func (c *Profile) GetLogger() *glog.Config {
	conf := glog.DefaultConfig()
	if err := c.Get("logger", conf); err != nil {
		glog.Fatal("get logger config", zap.Error(err))
	}
	return conf
}
