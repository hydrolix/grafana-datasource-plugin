// Package converters provides Hydrolix plugin SQL converters.
package converters

import (
	"encoding/json"
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
