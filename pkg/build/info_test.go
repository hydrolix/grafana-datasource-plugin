package build

import (
	"errors"
	"github.com/grafana/grafana-plugin-sdk-go/build/buildinfo"
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
		mBuildInfo := buildinfo.Info{Time: 5, PluginID: "testid", Version: "testversion"}
		bi := BuildInfo{
			buildInfoProvider: func() (buildinfo.Info, error) {
				return mBuildInfo, nil
			},
		}
		info := bi.GetBuildInfo()
		assert.Equal(t, mBuildInfo, info)
	})
	t.Run("error grafana build info", func(t *testing.T) {
		bi := BuildInfo{
			buildInfoProvider: func() (buildinfo.Info, error) {
				return buildinfo.Info{}, errors.New("error")
			},
		}
		info := bi.GetBuildInfo()
		assert.Equal(t, DefaultBuilInfo, info)
	})
}
