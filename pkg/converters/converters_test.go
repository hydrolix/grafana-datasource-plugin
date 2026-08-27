package converters_test

import (
	"encoding/json"
	"errors"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"net"
	"testing"
	"time"

	"github.com/hydrolix/plugin/pkg/converters"
	"github.com/stretchr/testify/assert"
)

func getConverter(columnType string) sqlutil.Converter {
	for _, c := range converters.Converters {
		if c.Name == columnType || (c.InputTypeRegex != nil && c.InputTypeRegex.MatchString(columnType)) {
			return c
		}
	}
	panic("Converter not found: " + columnType)
}

func TestDate(t *testing.T) {
	str := "2014-11-12T11:45:26.371Z"
	d, _ := time.Parse(time.RFC3339Nano, str)
	sut := getConverter("Date")
	v, err := sut.FrameConverter.ConverterFunc(&d)
	assert.Nil(t, err)
	actual := v.(time.Time)
	assert.Equal(t, d, actual)
}

func TestNullableDate(t *testing.T) {
	str := "2014-11-12T11:45:26.371Z"
	d, _ := time.Parse(time.RFC3339Nano, str)
	val := &d
	sut := getConverter("Nullable(Date)")
	v, err := sut.FrameConverter.ConverterFunc(&val)
	assert.Nil(t, err)
	actual := v.(*time.Time)
	assert.Equal(t, val, actual)
}

func TestNullableDateShouldBeNil(t *testing.T) {
	sut := getConverter("Nullable(Date)")
	var d *time.Time
	v, err := sut.FrameConverter.ConverterFunc(&d)
	assert.Nil(t, err)
	actual := v.(*time.Time)
	assert.Equal(t, (*time.Time)(nil), actual)
}

func TestNullableString(t *testing.T) {
	var value *string
	sut := getConverter("Nullable(String)")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(*string)
	assert.Equal(t, value, actual)
}

func TestBool(t *testing.T) {
	value := true
	sut := getConverter("Bool")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(bool)
	assert.True(t, actual)
}

func TestNullableBool(t *testing.T) {
	var value *bool
	sut := getConverter("Nullable(Bool)")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(*bool)
	assert.Equal(t, value, actual)
}

func TestFloat64(t *testing.T) {
	value := 1.1
	sut := getConverter("Float64")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(float64)
	assert.Equal(t, value, actual)
}

func TestNullableFloat64(t *testing.T) {
	var value *float64
	sut := getConverter("Nullable(Float64)")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(*float64)
	assert.Equal(t, value, actual)
}

func TestInt64(t *testing.T) {
	value := int64(1)
	sut := getConverter("Int64")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(int64)
	assert.Equal(t, value, actual)
}

func TestNullableInt64(t *testing.T) {
	var value *int64
	sut := getConverter("Nullable(Int64)")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(*int64)
	assert.Equal(t, value, actual)
}

func TestInt32(t *testing.T) {
	value := int32(1)
	sut := getConverter("Int32")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(int32)
	assert.Equal(t, value, actual)
}

func TestNullableInt32(t *testing.T) {
	var value *int32
	sut := getConverter("Nullable(Int32)")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(*int32)
	assert.Equal(t, value, actual)
}

func TestInt8(t *testing.T) {
	value := int8(1)
	sut := getConverter("Int8")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(int8)
	assert.Equal(t, value, actual)
}

func TestNullableInt8(t *testing.T) {
	var value *int8
	sut := getConverter("Nullable(Int8)")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(*int8)
	assert.Equal(t, value, actual)
}

func TestInt16(t *testing.T) {
	value := int16(1)
	sut := getConverter("Int16")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(int16)
	assert.Equal(t, value, actual)
}

func TestNullableInt16(t *testing.T) {
	var value *int16
	sut := getConverter("Nullable(Int16)")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(*int16)
	assert.Equal(t, value, actual)
}

func TestUInt8(t *testing.T) {
	value := uint8(1)
	sut := getConverter("UInt8")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(uint8)
	assert.Equal(t, value, actual)
}

func TestNullableUInt8(t *testing.T) {
	value := uint8(100)
	val := &value
	sut := getConverter("Nullable(UInt8)")
	v, err := sut.FrameConverter.ConverterFunc(&val)
	assert.Nil(t, err)
	actual := v.(*uint8)
	assert.Equal(t, value, *actual)
}

func TestNullableUInt8ShouldBeNil(t *testing.T) {
	var value *uint8
	val := &value
	sut := getConverter("Nullable(UInt8)")
	v, err := sut.FrameConverter.ConverterFunc(val)
	assert.Nil(t, err)
	actual := v.(*uint8)
	assert.Equal(t, value, actual)
}

func TestUInt16(t *testing.T) {
	value := uint16(100)
	val := &value
	sut := getConverter("UInt16")
	v, err := sut.FrameConverter.ConverterFunc(&val)
	assert.Nil(t, err)
	actual := v.(*uint16)
	assert.Equal(t, value, *actual)
}

func TestNullableUInt16(t *testing.T) {
	value := uint16(100)
	val := &value
	sut := getConverter("Nullable(UInt16)")
	v, err := sut.FrameConverter.ConverterFunc(&val)
	assert.Nil(t, err)
	actual := v.(*uint16)
	assert.Equal(t, value, *actual)
}

func TestNullableUInt16ShouldBeNil(t *testing.T) {
	var value *uint16
	val := &value
	sut := getConverter("Nullable(UInt16)")
	v, err := sut.FrameConverter.ConverterFunc(val)
	assert.Nil(t, err)
	actual := v.(*uint16)
	assert.Equal(t, value, actual)
}

func TestUInt32(t *testing.T) {
	value := uint32(100)
	val := &value
	sut := getConverter("UInt32")
	v, err := sut.FrameConverter.ConverterFunc(&val)
	assert.Nil(t, err)
	actual := v.(*uint32)
	assert.Equal(t, value, *actual)
}

func TestNullableUInt32(t *testing.T) {
	value := uint32(100)
	val := &value
	sut := getConverter("Nullable(UInt32)")
	v, err := sut.FrameConverter.ConverterFunc(&val)
	assert.Nil(t, err)
	actual := v.(*uint32)
	assert.Equal(t, value, *actual)
}

func TestNullableUInt32ShouldBeNil(t *testing.T) {
	var value *uint32
	val := &value
	sut := getConverter("Nullable(UInt32)")
	v, err := sut.FrameConverter.ConverterFunc(val)
	assert.Nil(t, err)
	actual := v.(*uint32)
	assert.Equal(t, value, actual)
}

func TestUInt64(t *testing.T) {
	value := uint64(100)
	val := &value
	sut := getConverter("UInt64")
	v, err := sut.FrameConverter.ConverterFunc(&val)
	assert.Nil(t, err)
	actual := v.(*uint64)
	assert.Equal(t, value, *actual)
}

func TestNullableUInt64(t *testing.T) {
	value := uint64(100)
	val := &value
	sut := getConverter("Nullable(UInt64)")
	v, err := sut.FrameConverter.ConverterFunc(&val)
	assert.Nil(t, err)
	actual := v.(*uint64)
	assert.Equal(t, value, *actual)
}

func TestNullableUInt64ShouldBeNil(t *testing.T) {
	var value *uint64
	val := &value
	sut := getConverter("Nullable(UInt64)")
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
	sut := getConverter("Map(String, Uint16)")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	msg, err := toJson(value)
	assert.Nil(t, err)
	assert.Equal(t, msg, *v.(*json.RawMessage))
}

func TestArray(t *testing.T) {
	value := []string{"1", "2", "3"}
	ipConverter := getConverter("Array(String)")
	v, err := ipConverter.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	msg, err := toJson(value)
	assert.Nil(t, err)
	assert.Equal(t, msg, *v.(*json.RawMessage))
}

func TestUUID(t *testing.T) {
	value := "61f0c404-5cb3-11e7-907b-a6006ad3dba0"
	sut := getConverter("UUID")
	v, err := sut.FrameConverter.ConverterFunc(&value)
	assert.Nil(t, err)
	actual := v.(string)
	assert.Equal(t, value, actual)
}

func TestNullableUUID(t *testing.T) {
	value := "61f0c404-5cb3-11e7-907b-a6006ad3dba0"
	val := &value
	sut := getConverter("Nullable(UUID)")
	v, err := sut.FrameConverter.ConverterFunc(&val)
	assert.Nil(t, err)
	actual := v.(*string)
	assert.Equal(t, value, *actual)
}

func TestNullableUUIDShouldBeNil(t *testing.T) {
	var value *string
	val := &value
	sut := getConverter("Nullable(UUID)")
	v, err := sut.FrameConverter.ConverterFunc(val)
	assert.Nil(t, err)
	actual := v.(*string)
	assert.Equal(t, value, actual)
}

func TestIPv4(t *testing.T) {
	ip := net.ParseIP("1.2.3.4")
	sut := getConverter("IPv4")
	v, err := sut.FrameConverter.ConverterFunc(&ip)
	assert.Nil(t, err)
	actual := v.(string)
	assert.Equal(t, "1.2.3.4", actual)
}

func TestIPv6(t *testing.T) {
	ip := net.ParseIP("2001:db8::1")
	sut := getConverter("IPv6")
	v, err := sut.FrameConverter.ConverterFunc(&ip)
	assert.Nil(t, err)
	actual := v.(string)
	assert.Equal(t, "2001:db8::1", actual)
}

// TestIPv6IPv4MappedAddress pins the rendering rule: it follows the *column type*, so an
// IPv4-mapped address in an IPv6 column keeps ClickHouse's "::ffff:" prefix
// rather than collapsing to the dotted-quad form net.IP.String() would give.
// This is the dominant case on Hydrolix, which stores IPv4 addresses mapped.
func TestIPv6IPv4MappedAddress(t *testing.T) {
	ip := net.IP{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 1, 2, 3, 4}
	assert.Equal(t, 16, len(ip))
	// The behaviour being overridden: value-driven formatting loses the prefix.
	assert.Equal(t, "1.2.3.4", ip.String())

	sut := getConverter("IPv6")
	v, err := sut.FrameConverter.ConverterFunc(&ip)
	assert.Nil(t, err)
	actual := v.(string)
	assert.Equal(t, "::ffff:1.2.3.4", actual)
}

// TestIPv4ColumnStaysDottedQuad is the other half of that rule: the *same* mapped bytes
// in an IPv4 column render dotted-quad, because that is what ClickHouse shows
// for an IPv4 column. The two converters must not be interchangeable.
func TestIPv4ColumnStaysDottedQuad(t *testing.T) {
	ip := net.IP{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 1, 2, 3, 4}
	sut := getConverter("IPv4")
	v, err := sut.FrameConverter.ConverterFunc(&ip)
	assert.Nil(t, err)
	assert.Equal(t, "1.2.3.4", v.(string))
}

// TestIPv6RenderingMatchesClickhouse pins the full rendering table against
// ClickHouse's toString() output, verified on clickhouse-server 24.8 and 25.x.
// The IPv4-compatible form (::1.2.3.4, no ffff) is the single divergence:
// ClickHouse prints "::1.2.3.4", Go prints "::102:304". That address form was
// deprecated by RFC 4291 and ClickHouse cannot store it distinctly from any
// other 16-byte value, so it is left alone rather than special-cased.
func TestIPv6RenderingMatchesClickhouse(t *testing.T) {
	sut := getConverter("IPv6")
	cases := []struct {
		ip       net.IP
		expected string
	}{
		{net.ParseIP("2001:db8::1"), "2001:db8::1"},
		{net.IP{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 1, 2, 3, 4}, "::ffff:1.2.3.4"},
		{net.IP{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 0, 0, 0, 0}, "::ffff:0.0.0.0"},
		{net.IPv6zero, "::"},
		{net.IPv6loopback, "::1"},
	}
	for _, c := range cases {
		v, err := sut.FrameConverter.ConverterFunc(&c.ip)
		assert.Nil(t, err)
		assert.Equal(t, c.expected, v.(string))
	}
}

// TestNullableIPv6IPv4MappedAddress: the nullable path must render identically
// to the non-nullable one. Both variants are wired from the same renderer, and
// this keeps them from drifting apart.
func TestNullableIPv6IPv4MappedAddress(t *testing.T) {
	ip := net.IP{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 1, 2, 3, 4}
	val := &ip
	sut := getConverter("Nullable(IPv6)")
	v, err := sut.FrameConverter.ConverterFunc(&val)
	assert.Nil(t, err)
	assert.Equal(t, "::ffff:1.2.3.4", *v.(*string))
}

// TestIPConverterRejectsWrongLength: a value that is neither 4 nor 16 bytes
// cannot be rendered, and must error rather than yield "?" or an empty cell.
func TestIPConverterRejectsWrongLength(t *testing.T) {
	ip := net.IP{1, 2, 3}
	for _, name := range []string{"IPv4", "IPv6"} {
		sut := getConverter(name)
		v, err := sut.FrameConverter.ConverterFunc(&ip)
		assert.NotNil(t, err, name)
		assert.Nil(t, v, name)
	}
}

func TestNullableIPv4(t *testing.T) {
	ip := net.ParseIP("5.6.7.8")
	val := &ip
	sut := getConverter("Nullable(IPv4)")
	v, err := sut.FrameConverter.ConverterFunc(&val)
	assert.Nil(t, err)
	actual := v.(*string)
	assert.Equal(t, "5.6.7.8", *actual)
}

func TestNullableIPv4ShouldBeNil(t *testing.T) {
	var ip *net.IP
	val := &ip
	sut := getConverter("Nullable(IPv4)")
	v, err := sut.FrameConverter.ConverterFunc(val)
	assert.Nil(t, err)
	actual := v.(*string)
	assert.Equal(t, (*string)(nil), actual)
}

func TestNullableIPv6(t *testing.T) {
	ip := net.ParseIP("2001:db8::2")
	val := &ip
	sut := getConverter("Nullable(IPv6)")
	v, err := sut.FrameConverter.ConverterFunc(&val)
	assert.Nil(t, err)
	actual := v.(*string)
	assert.Equal(t, "2001:db8::2", *actual)
}

func TestNullableIPv6ShouldBeNil(t *testing.T) {
	var ip *net.IP
	val := &ip
	sut := getConverter("Nullable(IPv6)")
	v, err := sut.FrameConverter.ConverterFunc(val)
	assert.Nil(t, err)
	actual := v.(*string)
	assert.Equal(t, (*string)(nil), actual)
}

func TestIPConverterMismatchedType(t *testing.T) {
	sut := getConverter("IPv4")
	_, err := sut.FrameConverter.ConverterFunc("not-an-ip-pointer")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "*net.IP")
	assert.Contains(t, err.Error(), "string")
}

func TestNullableIPConverterMismatchedType(t *testing.T) {
	sut := getConverter("Nullable(IPv4)")
	_, err := sut.FrameConverter.ConverterFunc("not-a-double-pointer")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "**net.IP")
	assert.Contains(t, err.Error(), "string")
}

// TestNullableIPv4NoAliasing exercises the sqlutil.FrameConverter aliasing
// contract: the scan buffer is reused across rows, so the returned *string
// for an earlier row must survive later rows mutating that buffer.
func TestNullableIPv4NoAliasing(t *testing.T) {
	sut := getConverter("Nullable(IPv4)")
	var ptr *net.IP
	scan := &ptr

	ip1 := net.ParseIP("1.1.1.1")
	ptr = &ip1
	v1, err := sut.FrameConverter.ConverterFunc(scan)
	assert.Nil(t, err)
	s1 := v1.(*string)

	ip2 := net.ParseIP("2.2.2.2")
	ptr = &ip2
	v2, err := sut.FrameConverter.ConverterFunc(scan)
	assert.Nil(t, err)
	s2 := v2.(*string)

	assert.Equal(t, "1.1.1.1", *s1)
	assert.Equal(t, "2.2.2.2", *s2)
}

// TestUUIDAndIPTypesResolveExactlyOnce pins exact-name matching: no
// regex reachability, so registry map iteration order can't produce a
// nondeterministic match for any of these six type names.
func TestUUIDAndIPTypesResolveExactlyOnce(t *testing.T) {
	names := []string{"UUID", "Nullable(UUID)", "IPv4", "Nullable(IPv4)", "IPv6", "Nullable(IPv6)"}
	for _, name := range names {
		count := 0
		for _, c := range converters.Converters {
			if c.Name == name || (c.InputTypeRegex != nil && c.InputTypeRegex.MatchString(name)) {
				count++
			}
		}
		assert.Equal(t, 1, count, "expected exactly one converter to match %s", name)
	}
}
