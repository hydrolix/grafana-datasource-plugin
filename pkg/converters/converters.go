package converters

import (
	"encoding/json"
	"reflect"
	"regexp"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
)

type Converter struct {
	convert    func(in interface{}) (interface{}, error)
	fieldType  data.FieldType
	matchRegex *regexp.Regexp
	scanType   reflect.Type
}

var matchRegexes = map[string]*regexp.Regexp{
	// for complex Arrays e.g. Array(Tuple)
	"Array()":                   regexp.MustCompile(`^Array\(.*\)`),
	"Date":                      regexp.MustCompile(`^Date\(?`),
	"Map()":                     regexp.MustCompile(`^Map\(.*\)`),
	"Nullable(Date)":            regexp.MustCompile(`^Nullable\(Date\(?`),
	"Nullable(String)":          regexp.MustCompile(`^Nullable\(String`),
}

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
		fieldType:  data.FieldTypeTime,
		matchRegex: matchRegexes["Date"],
		scanType:   reflect.PointerTo(reflect.TypeOf(time.Time{})),
	},
	"Nullable(Date)": {
		fieldType:  data.FieldTypeNullableTime,
		matchRegex: matchRegexes["Nullable(Date)"],
		scanType:   reflect.PointerTo(reflect.PointerTo(reflect.TypeOf(time.Time{}))),
	},
	"Nullable(String)": {
		fieldType:  data.FieldTypeNullableString,
		matchRegex: matchRegexes["Nullable(String)"],
		scanType:   reflect.PointerTo(reflect.PointerTo(reflect.TypeOf(""))),
	},
	"Array()": {
		convert:    jsonConverter,
		fieldType:  data.FieldTypeNullableJSON,
		matchRegex: matchRegexes["Array()"],
		scanType:   reflect.TypeOf((*interface{})(nil)).Elem(),
	},
	"Map()": {
		convert:    jsonConverter,
		fieldType:  data.FieldTypeNullableJSON,
		matchRegex: matchRegexes["Map()"],
		scanType:   reflect.TypeOf((*interface{})(nil)).Elem(),
	},

}

func createSqlSdkConverters() []sqlutil.Converter {
	var list []sqlutil.Converter
	for name, converter := range convertersMap {
		list = append(list, createConverter(name, converter))
	}
	return list
}

func GetConverter(columnType string) sqlutil.Converter {
	if converter, ok := convertersMap[columnType]; ok {
		return createConverter(columnType, converter)
	}
	return findConverterWithRegex(columnType)
}

func findConverterWithRegex(columnType string) sqlutil.Converter {
	for name, converter := range convertersMap {
		if converter.matchRegex != nil && converter.matchRegex.MatchString(columnType) {
			return createConverter(name, converter)
		}
	}

	return sqlutil.Converter{}
}

func createConverter(name string, converter Converter) sqlutil.Converter {
	convert := defaultConvert
	if converter.convert != nil {
		convert = converter.convert
	}
	return sqlutil.Converter{
		Name:           name,
		InputScanType:  converter.scanType,
		InputTypeRegex: converter.matchRegex,
		InputTypeName:  name,
		FrameConverter: sqlutil.FrameConverter{
			FieldType:     converter.fieldType,
			ConverterFunc: convert,
		},
	}
}


func defaultConvert(in interface{}) (interface{}, error) {
	if in == nil {
		return reflect.Zero(reflect.TypeOf(in)).Interface(), nil
	}

	val := reflect.ValueOf(in)
	switch val.Kind(){
	case reflect.String:
		return in, nil
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

var Converters = createSqlSdkConverters()
