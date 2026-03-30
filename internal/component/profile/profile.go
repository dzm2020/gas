package profile

import (
	"context"

	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/cluster"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/component"
	"github.com/dzm2020/gas/pkg/lib/fileutil"
	"github.com/spf13/viper"
)

const Name = "profile"

var _ iface.IProfile = (*Profile)(nil)

// Profile 配置加载组件：持有一个 viper 实例
type Profile struct {
	component.BaseComponent[iface.INode]
	vp *viper.Viper
}

// New 创建配置组件，path 为配置文件路径。
func New(path string) *Profile {
	vp, err := fileutil.ViperLoadConfigWithInclude(path)
	if err != nil {
		panic(err)
	}
	return &Profile{vp: vp}
}

func (c *Profile) Name() string {
	return Name
}

func (c *Profile) Start(ctx context.Context, node iface.INode) error {
	return nil
}

func (c *Profile) Get(key string, cfg interface{}) error {
	return c.vp.UnmarshalKey(key, cfg)
}

func (c *Profile) Standalone() bool {
	return c.vp.GetBool("standalone")
}

func (c *Profile) GetCluster() *cluster.Config {
	conf := cluster.DefaultConfig()
	if err := c.Get("cluster", conf); err != nil {
		return conf
	}
	return conf
}

func (c *Profile) GetLogger() *glog.Config {
	conf := glog.DefaultConfig()
	if err := c.Get("logger", conf); err != nil {
		return conf
	}
	return conf
}
