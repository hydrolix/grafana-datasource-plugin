// Package converters provides Hydrolix plugin SQL converters.
package converters

import (
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"reflect"
	"regexp"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
)

// Converter structure to describe and generate sqlutil.Converter
type Converter struct {
	convert    func(in interface{}) (interface{}, error)
	fieldType  data.FieldType
	matchRegex *regexp.Regexp
	scanType   reflect.Type
}

// toSqlConverter turns this Converter into a sqlutil.Converter
func (c *Converter) toSqlConverter(name string) sqlutil.Converter {
	convert := defaultConvert
	if c.convert != nil {
		convert = c.convert
	}
	return sqlutil.Converter{
		Name:           name,
		InputScanType:  c.scanType,
		InputTypeRegex: c.matchRegex,
		InputTypeName:  name,
		FrameConverter: sqlutil.FrameConverter{
			FieldType:     c.fieldType,
			ConverterFunc: convert,
		},
	}
}

// Default converter transforms nullables to their type and empty nullables as string nullables.
func defaultConvert(in interface{}) (interface{}, error) {
	if in == nil {
		return reflect.Zero(reflect.TypeOf(in)).Interface(), nil
	}

	val := reflect.ValueOf(in)
	switch val.Kind() {
	case reflect.Pointer:
		if val.IsNil() {
			// we can't dereference nil pointer.
			return (*string)(nil), nil
		}
		return val.Elem().Interface(), nil
	default:
		return in, nil
	}

}

// Json converter  transforms value to json
func jsonConverter(in interface{}) (interface{}, error) {
	if in == nil {
		return (*string)(nil), nil
	}
	bjson, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}

	msg := json.RawMessage(bjson)
	return &msg, nil
}

// IP address rendering is driven by the *column type*, not by the value, which
// is what ClickHouse itself does: an IPv4 column always renders dotted-quad, and
// an IPv6 column always renders in IPv6 notation — including the "::ffff:"
// prefix for an IPv4-mapped address. Go's net.IP.String() is value-driven
// instead and collapses a mapped 16-byte address to dotted-quad, which is why
// the IPv6 path cannot simply call it (see design decision D5).
//
// The registry pairs each type name with the matching renderer, so the mapping
// holds regardless of how many bytes the driver happens to hand over.

// ipv4Text renders a value from an IPv4 column in dotted-quad form. The driver
// supplies 4 bytes, but net.ParseIP and friends yield the 16-byte IPv4-mapped
// encoding, so normalise through To4 rather than assuming a length.
func ipv4Text(ip net.IP) (string, error) {
	v4 := ip.To4()
	if v4 == nil {
		return "", fmt.Errorf("expected an IPv4 address, got %d bytes (%v)", len(ip), ip)
	}
	return v4.String(), nil
}

// ipv6Text renders a value from an IPv6 column in IPv6 notation, matching
// ClickHouse's own formatter. netip.Addr is used instead of net.IP because
// AddrFrom16 never unmaps: an IPv4-mapped address stays "::ffff:1.2.3.4" where
// net.IP.String() would print "1.2.3.4".
//
// A 4-byte input (not something the driver produces for an IPv6 column, but
// cheap to handle) is widened by To16 into its mapped form, which is exactly
// how ClickHouse would store and render it in an IPv6 column.
func ipv6Text(ip net.IP) (string, error) {
	b := ip.To16()
	if b == nil {
		return "", fmt.Errorf("expected an IPv6 address, got %d bytes (%v)", len(ip), ip)
	}
	return netip.AddrFrom16([16]byte(b)).String(), nil
}

// ipConverter builds the ConverterFunc for a non-nullable IP column. The
// clickhouse driver hands IP columns to database/sql as net.IP, which is not a
// type data.Frame can hold, so the frame field is a plain string.
func ipConverter(text func(net.IP) (string, error)) func(in interface{}) (interface{}, error) {
	return func(in interface{}) (interface{}, error) {
		ip, ok := in.(*net.IP)
		if !ok {
			return nil, fmt.Errorf("ip converter: expected *net.IP, got %T", in)
		}
		s, err := text(*ip)
		if err != nil {
			return nil, err
		}
		return s, nil
	}
}

// nullableIPConverter is ipConverter for Nullable(IPv4)/Nullable(IPv6). NULL
// arrives as a nil *net.IP and is passed on as a nil *string. The returned
// pointer is freshly allocated so it never aliases the reused scan buffer.
func nullableIPConverter(text func(net.IP) (string, error)) func(in interface{}) (interface{}, error) {
	return func(in interface{}) (interface{}, error) {
		ip, ok := in.(**net.IP)
		if !ok {
			return nil, fmt.Errorf("nullable ip converter: expected **net.IP, got %T", in)
		}
		if *ip == nil {
			return (*string)(nil), nil
		}
		s, err := text(**ip)
		if err != nil {
			return nil, err
		}
		return &s, nil
	}
}

// Map of plugin converters
var convertersMap = map[string]Converter{
	"String": {
		fieldType: data.FieldTypeString,
		scanType:  reflect.PointerTo(reflect.TypeOf("")),
	},
	"Bool": {
		fieldType: data.FieldTypeBool,
		scanType:  reflect.PointerTo(reflect.TypeOf(true)),
	},
	"Nullable(Bool)": {
		fieldType: data.FieldTypeNullableBool,
		scanType:  reflect.PointerTo(reflect.PointerTo(reflect.TypeOf(true))),
	},
	"Float64": {
		fieldType: data.FieldTypeFloat64,
		scanType:  reflect.PointerTo(reflect.TypeOf(float64(0))),
	},
	"Nullable(Float64)": {
		fieldType: data.FieldTypeNullableFloat64,
		scanType:  reflect.PointerTo(reflect.PointerTo(reflect.TypeOf(float64(0)))),
	},
	"Int64": {
		fieldType: data.FieldTypeInt64,
		scanType:  reflect.PointerTo(reflect.TypeOf(int64(0))),
	},
	"Int32": {
		fieldType: data.FieldTypeInt32,
		scanType:  reflect.PointerTo(reflect.TypeOf(int32(0))),
	},
	"Int16": {
		fieldType: data.FieldTypeInt16,
		scanType:  reflect.PointerTo(reflect.TypeOf(int16(0))),
	},
	"Int8": {
		fieldType: data.FieldTypeInt8,
		scanType:  reflect.PointerTo(reflect.TypeOf(int8(0))),
	},
	"UInt64": {
		fieldType: data.FieldTypeUint64,
		scanType:  reflect.PointerTo(reflect.TypeOf(uint64(0))),
	},
	"UInt32": {
		fieldType: data.FieldTypeUint32,
		scanType:  reflect.PointerTo(reflect.TypeOf(uint32(0))),
	},
	"UInt16": {
		fieldType: data.FieldTypeUint16,
		scanType:  reflect.PointerTo(reflect.TypeOf(uint16(0))),
	},
	"UInt8": {
		fieldType: data.FieldTypeUint8,
		scanType:  reflect.PointerTo(reflect.TypeOf(uint8(0))),
	},
	"Nullable(UInt64)": {
		fieldType: data.FieldTypeNullableUint64,
		scanType:  reflect.PointerTo(reflect.PointerTo(reflect.TypeOf(uint64(0)))),
	},
	"Nullable(UInt32)": {
		fieldType: data.FieldTypeNullableUint32,
		scanType:  reflect.PointerTo(reflect.PointerTo(reflect.TypeOf(uint32(0)))),
	},
	"Nullable(UInt16)": {
		fieldType: data.FieldTypeNullableUint16,
		scanType:  reflect.PointerTo(reflect.PointerTo(reflect.TypeOf(uint16(0)))),
	},
	"Nullable(UInt8)": {
		fieldType: data.FieldTypeNullableUint8,
		scanType:  reflect.PointerTo(reflect.PointerTo(reflect.TypeOf(uint8(0)))),
	},
	"Nullable(Int64)": {
		fieldType: data.FieldTypeNullableInt64,
		scanType:  reflect.PointerTo(reflect.PointerTo(reflect.TypeOf(int64(0)))),
	},
	"Nullable(Int32)": {
		fieldType: data.FieldTypeNullableInt32,
		scanType:  reflect.PointerTo(reflect.PointerTo(reflect.TypeOf(int32(0)))),
	},
	"Nullable(Int16)": {
		fieldType: data.FieldTypeNullableInt16,
		scanType:  reflect.PointerTo(reflect.PointerTo(reflect.TypeOf(int16(0)))),
	},
	"Nullable(Int8)": {
		fieldType: data.FieldTypeNullableInt8,
		scanType:  reflect.PointerTo(reflect.PointerTo(reflect.TypeOf(int8(0)))),
	},
	// covers DateTime with tz, DateTime64 - see regexes, Date32
	"Date": {
		matchRegex: regexp.MustCompile(`^Date\(?`),
		fieldType:  data.FieldTypeTime,
		scanType:   reflect.PointerTo(reflect.TypeOf(time.Time{})),
	},
	"Nullable(Date)": {
		matchRegex: regexp.MustCompile(`^Nullable\(Date\(?`),
		fieldType:  data.FieldTypeNullableTime,
		scanType:   reflect.PointerTo(reflect.PointerTo(reflect.TypeOf(time.Time{}))),
	},
	"Nullable(String)": {
		matchRegex: regexp.MustCompile(`^Nullable\(String`),
		fieldType:  data.FieldTypeNullableString,
		scanType:   reflect.PointerTo(reflect.PointerTo(reflect.TypeOf(""))),
	},
	// uuid.UUID is a driver.Valuer that yields its textual form, so a UUID
	// column already reaches database/sql as a string.
	"UUID": {
		fieldType: data.FieldTypeString,
		scanType:  reflect.PointerTo(reflect.TypeOf("")),
	},
	"Nullable(UUID)": {
		fieldType: data.FieldTypeNullableString,
		scanType:  reflect.PointerTo(reflect.PointerTo(reflect.TypeOf(""))),
	},
	"IPv4": {
		fieldType: data.FieldTypeString,
		scanType:  reflect.PointerTo(reflect.TypeOf(net.IP{})),
		convert:   ipConverter(ipv4Text),
	},
	"Nullable(IPv4)": {
		fieldType: data.FieldTypeNullableString,
		scanType:  reflect.PointerTo(reflect.PointerTo(reflect.TypeOf(net.IP{}))),
		convert:   nullableIPConverter(ipv4Text),
	},
	"IPv6": {
		fieldType: data.FieldTypeString,
		scanType:  reflect.PointerTo(reflect.TypeOf(net.IP{})),
		convert:   ipConverter(ipv6Text),
	},
	"Nullable(IPv6)": {
		fieldType: data.FieldTypeNullableString,
		scanType:  reflect.PointerTo(reflect.PointerTo(reflect.TypeOf(net.IP{}))),
		convert:   nullableIPConverter(ipv6Text),
	},
	"Array()": {
		matchRegex: regexp.MustCompile(`^Array\(.*\)`),
		fieldType:  data.FieldTypeNullableJSON,
		scanType:   reflect.TypeOf((*interface{})(nil)).Elem(),
		convert:    jsonConverter,
	},
	"Map()": {
		matchRegex: regexp.MustCompile(`^Map\(.*\)`),
		fieldType:  data.FieldTypeNullableJSON,
		scanType:   reflect.TypeOf((*interface{})(nil)).Elem(),
		convert:    jsonConverter,
	},
}

// Converters List of adapters for Grafana data.Frame
var Converters = func() []sqlutil.Converter {
	var list = make([]sqlutil.Converter, 0, len(convertersMap))
	for name, converter := range convertersMap {
		list = append(list, converter.toSqlConverter(name))
	}
	return list
}()
