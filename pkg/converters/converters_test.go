package converters_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/hydrolix/plugin/pkg/converters"
	"github.com/stretchr/testify/assert"
)

func TestDate(t *testing.T) {
	str := "2014-11-12T11:45:26.371Z"
	d, _ := time.Parse(time.RFC3339Nano, str)
	sut := converters.GetConverter("Date")
	v, err := sut.FrameConverter.ConverterFunc(&d)
	assert.Nil(t, err)
	actual := v.(time.Time)
	assert.Equal(t, d, actual)
}

func TestNullableDate(t *testing.T) {
	str := "2014-11-12T11:45:26.371Z"
	d, _ := time.Parse(time.RFC3339Nano, str)
	val := &d
	sut := converters.GetConverter("Nullable(Date)")
	v, err := sut.FrameConverter.ConverterFunc(&val)
	assert.Nil(t, err)
	actual := v.(*time.Time)
	assert.Equal(t, val, actual)
}

func TestNullableDateShouldBeNil(t *testing.T) {
	sut := converters.GetConverter("Nullable(Date)")
	var d *time.Time
	v, err := sut.FrameConverter.ConverterFunc(&d)
	assert.Nil(t, err)
	actual := v.(*time.Time)
	assert.Equal(t, (*time.Time)(nil), actual)
}

func TestNullableString(t *testing.T) {
	var value *string
	sut := converters.GetConverter("Nullable(String)")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(*string)
	assert.Equal(t, value, actual)
}

func TestBool(t *testing.T) {
	value := true
	sut := converters.GetConverter("Bool")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(bool)
	assert.True(t, actual)
}

func TestNullableBool(t *testing.T) {
	var value *bool
	sut := converters.GetConverter("Nullable(Bool)")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(*bool)
	assert.Equal(t, value, actual)
}

func TestFloat64(t *testing.T) {
	value := 1.1
	sut := converters.GetConverter("Float64")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(float64)
	assert.Equal(t, value, actual)
}

func TestNullableFloat64(t *testing.T) {
	var value *float64
	sut := converters.GetConverter("Nullable(Float64)")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(*float64)
	assert.Equal(t, value, actual)
}

func TestInt64(t *testing.T) {
	value := int64(1)
	sut := converters.GetConverter("Int64")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(int64)
	assert.Equal(t, value, actual)
}

func TestNullableInt64(t *testing.T) {
	var value *int64
	sut := converters.GetConverter("Nullable(Int64)")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(*int64)
	assert.Equal(t, value, actual)
}

func TestInt32(t *testing.T) {
	value := int32(1)
	sut := converters.GetConverter("Int32")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(int32)
	assert.Equal(t, value, actual)
}

func TestNullableInt32(t *testing.T) {
	var value *int32
	sut := converters.GetConverter("Nullable(Int32)")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(*int32)
	assert.Equal(t, value, actual)
}

func TestInt8(t *testing.T) {
	value := int8(1)
	sut := converters.GetConverter("Int8")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(int8)
	assert.Equal(t, value, actual)
}

func TestNullableInt8(t *testing.T) {
	var value *int8
	sut := converters.GetConverter("Nullable(Int8)")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(*int8)
	assert.Equal(t, value, actual)
}

func TestInt16(t *testing.T) {
	value := int16(1)
	sut := converters.GetConverter("Int16")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(int16)
	assert.Equal(t, value, actual)
}

func TestNullableInt16(t *testing.T) {
	var value *int16
	sut := converters.GetConverter("Nullable(Int16)")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(*int16)
	assert.Equal(t, value, actual)
}

func TestUInt8(t *testing.T) {
	value := uint8(1)
	sut := converters.GetConverter("UInt8")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(uint8)
	assert.Equal(t, value, actual)
}

func TestNullableUInt8(t *testing.T) {
	value := uint8(100)
	val := &value
	sut := converters.GetConverter("Nullable(UInt8)")
	v, err := sut.FrameConverter.ConverterFunc(&val)
	assert.Nil(t, err)
	actual := v.(*uint8)
	assert.Equal(t, value, *actual)
}

func TestNullableUInt8ShouldBeNil(t *testing.T) {
	var value *uint8
	val := &value
	sut := converters.GetConverter("Nullable(UInt8)")
	v, err := sut.FrameConverter.ConverterFunc(val)
	assert.Nil(t, err)
	actual := v.(*uint8)
	assert.Equal(t, value, actual)
}

func TestUInt16(t *testing.T) {
	value := uint16(100)
	val := &value
	sut := converters.GetConverter("UInt16")
	v, err := sut.FrameConverter.ConverterFunc(&val)
	assert.Nil(t, err)
	actual := v.(*uint16)
	assert.Equal(t, value, *actual)
}

func TestNullableUInt16(t *testing.T) {
	value := uint16(100)
	val := &value
	sut := converters.GetConverter("Nullable(UInt16)")
	v, err := sut.FrameConverter.ConverterFunc(&val)
	assert.Nil(t, err)
	actual := v.(*uint16)
	assert.Equal(t, value, *actual)
}

func TestNullableUInt16ShouldBeNil(t *testing.T) {
	var value *uint16
	val := &value
	sut := converters.GetConverter("Nullable(UInt16)")
	v, err := sut.FrameConverter.ConverterFunc(val)
	assert.Nil(t, err)
	actual := v.(*uint16)
	assert.Equal(t, value, actual)
}

func TestUInt32(t *testing.T) {
	value := uint32(100)
	val := &value
	sut := converters.GetConverter("UInt32")
	v, err := sut.FrameConverter.ConverterFunc(&val)
	assert.Nil(t, err)
	actual := v.(*uint32)
	assert.Equal(t, value, *actual)
}

func TestNullableUInt32(t *testing.T) {
	value := uint32(100)
	val := &value
	sut := converters.GetConverter("Nullable(UInt32)")
	v, err := sut.FrameConverter.ConverterFunc(&val)
	assert.Nil(t, err)
	actual := v.(*uint32)
	assert.Equal(t, value, *actual)
}

func TestNullableUInt32ShouldBeNil(t *testing.T) {
	var value *uint32
	val := &value
	sut := converters.GetConverter("Nullable(UInt32)")
	v, err := sut.FrameConverter.ConverterFunc(val)
	assert.Nil(t, err)
	actual := v.(*uint32)
	assert.Equal(t, value, actual)
}

func TestUInt64(t *testing.T) {
	value := uint64(100)
	val := &value
	sut := converters.GetConverter("UInt64")
	v, err := sut.FrameConverter.ConverterFunc(&val)
	assert.Nil(t, err)
	actual := v.(*uint64)
	assert.Equal(t, value, *actual)
}

func TestNullableUInt64(t *testing.T) {
	value := uint64(100)
	val := &value
	sut := converters.GetConverter("Nullable(UInt64)")
	v, err := sut.FrameConverter.ConverterFunc(&val)
	assert.Nil(t, err)
	actual := v.(*uint64)
	assert.Equal(t, value, *actual)
}

func TestNullableUInt64ShouldBeNil(t *testing.T) {
	var value *uint64
	val := &value
	sut := converters.GetConverter("Nullable(UInt64)")
	v, err := sut.FrameConverter.ConverterFunc(val)
	assert.Nil(t, err)
	actual := v.(*uint64)
	assert.Equal(t, value, actual)
}

func toJson(obj interface{}) (json.RawMessage, error) {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return nil, errors.New("unable to marshal")
	}
	var rawJSON json.RawMessage
	err = json.Unmarshal(bytes, &rawJSON)
	if err != nil {
		return nil, errors.New("unable to unmarshal")
	}
	return rawJSON, nil
}

func TestMap(t *testing.T) {
	value := map[string]interface{}{
		"1": uint16(1),
		"2": uint16(2),
		"3": uint16(3),
		"4": uint16(4),
	}
	sut := converters.GetConverter("Map(String, Uint16)")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	msg, err := toJson(value)
	assert.Nil(t, err)
	assert.Equal(t, msg, *v.(*json.RawMessage))
}

func TestArray(t *testing.T) {
	value := []string{"1", "2", "3"}
	ipConverter := converters.GetConverter("Array(String)")
	v, err := ipConverter.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	msg, err := toJson(value)
	assert.Nil(t, err)
	assert.Equal(t, msg, *v.(*json.RawMessage))
}