package fileutil

import "testing"

func TestViperLoadConfigWithInclude(t *testing.T) {
	vp, err := ViperLoadConfigWithInclude("./config.yaml")
	if err != nil {
		t.Error(err)
	}

	if vp.GetInt("db.port") != 3306 {
		t.Error("db port should be 3306")
	}
}
