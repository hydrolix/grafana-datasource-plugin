package build

import (
	"errors"
	"github.com/grafana/grafana-plugin-sdk-go/build"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetBuildInfo(t *testing.T) {
	t.Run("not set grafana build info", func(t *testing.T) {
		bi := BuildInfo{}
		info := bi.GetBuildInfo()
		assert.Equal(t, DefaultBuilInfo, info)
	})
	t.Run("set grafana build info", func(t *testing.T) {
		mBuildInfo := build.Info{Time: 5, PluginID: "testid", Version: "testversion"}
		bi := BuildInfo{
			buildInfoProvider: func() (build.Info, error) {
				return mBuildInfo, nil
			},
		}
		info := bi.GetBuildInfo()
		assert.Equal(t, mBuildInfo, info)
	})
	t.Run("error grafana build info", func(t *testing.T) {
		bi := BuildInfo{
			buildInfoProvider: func() (build.Info, error) {
				return build.Info{}, errors.New("error")
			},
		}
		info := bi.GetBuildInfo()
		assert.Equal(t, DefaultBuilInfo, info)
	})
}
