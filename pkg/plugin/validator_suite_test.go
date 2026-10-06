package plugin_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/hydrolix/plugin/pkg/plugin"
	"github.com/hydrolix/plugin/pkg/plugin/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// ValidatorTestSuite runs QueryValidator through real sqlds and ClickHouse.
type ValidatorTestSuite struct {
	suite.Suite
	DsTestSuite
	validator *plugin.QueryValidator
}

func TestValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(ValidatorTestSuite))
}

func (s *ValidatorTestSuite) SetupSuite() {
	s.DsTestSuite.SetupSuite()
	settings := s.validatorSettings()

	db, err := plugin.NewHydrolix().Connect(s.Ctx, settings, json.RawMessage{})
	s.Require().NoError(err)
	_, err = db.ExecContext(s.Ctx, "CREATE TABLE IF NOT EXISTS default.validate_t (ts DateTime, v Int32) ENGINE = MergeTree ORDER BY ts")
	s.Require().NoError(err)
	s.Require().NoError(db.Close())

	instance, err := plugin.NewDatasource(s.Ctx, settings)
	s.Require().NoError(err)
	s.validator = instance.(*plugin.HdxSqlDatasource).Validator
}

func (s *ValidatorTestSuite) validatorSettings() backend.DataSourceInstanceSettings {
	jsonData, err := json.Marshal(models.PluginSettings{
		Host:            s.ChContainer.Hostname,
		UserName:        s.ChContainer.Username,
		Protocol:        "native",
		Port:            s.ChContainer.NativePort,
		DefaultDatabase: "default",
	})
	s.Require().NoError(err)
	return backend.DataSourceInstanceSettings{
		UID:                     "validator-suite",
		JSONData:                jsonData,
		DecryptedSecureJSONData: map[string]string{"password": s.ChContainer.Password},
	}
}

func (s *ValidatorTestSuite) validate(sql string, settings ...models.QuerySetting) models.ValidationResult {
	res, err := s.validator.Validate(context.Background(), models.HdxQuery{
		RawSQL:        sql,
		QuerySettings: settings,
		TimeRange:     backend.TimeRange{From: time.Unix(1744243200, 0).UTC(), To: time.Unix(1744329599, 0).UTC()},
		Interval:      time.Minute,
	})
	s.Require().NoError(err)
	return res
}

func (s *ValidatorTestSuite) TestValidQueryWithTimeFilter() {
	assert.Equal(s.T(), models.ValidationResult{}, s.validate("SELECT count() FROM validate_t WHERE $__timeFilter()"))
}

// Without the skip, sqlds would expand the literal's macro text and fail.
func (s *ValidatorTestSuite) TestMacroTextInLiteralSurvivesTheDryRun() {
	assert.Equal(s.T(), models.ValidationResult{}, s.validate("SELECT '$__timeFilter' AS a FROM validate_t WHERE $__timeFilter()"))
}

func (s *ValidatorTestSuite) TestUnknownColumnIsAnError() {
	res := s.validate("SELECT nope FROM validate_t WHERE $__timeFilter()")
	require.NotEmpty(s.T(), res.Error)
	assert.Contains(s.T(), res.Error, "nope")
}

func (s *ValidatorTestSuite) TestUnfilteredPrimaryKeyWarns() {
	assert.Equal(s.T(),
		models.ValidationResult{Warning: "Primary key `ts` of `validate_t` not filtered in WHERE; add $__timeFilter() to limit the scan."},
		s.validate("SELECT count() FROM validate_t"))
}

func (s *ValidatorTestSuite) TestCTEFilteredByItsReaderIsValid() {
	assert.Equal(s.T(), models.ValidationResult{}, s.validate("WITH x AS (SELECT * FROM validate_t) SELECT count() FROM x WHERE $__timeFilter(ts)"))
}

func (s *ValidatorTestSuite) TestQuerySettingsReachTheDriver() {
	res := s.validate("SELECT count() FROM validate_t WHERE $__timeFilter()", models.QuerySetting{Setting: "no_such_setting_hdx11409", Value: "1"})
	assert.Contains(s.T(), res.Error, "no_such_setting_hdx11409")
}
