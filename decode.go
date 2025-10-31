package sbinary

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"strings"
)

const _tag = "sbin"

// Unmarshal unmarshals data using provided byte order and stores result into pointer obj.
// See [Decoder.Decode] for details.
func Unmarshal(data []byte, obj any, order binary.ByteOrder) error {
	if err := NewDecoder(bytes.NewReader(data)).Decode(obj, order); err != nil {
		return err
	}

	return nil
}

// Decoder decodes incoming bytes into Go objects.
type Decoder struct {
	r io.Reader
}

// NewDecoder created a [Decoder] that will use r for the input.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

// Decode decodes obj. For numberic fields it uses provided byte order.
// It can be called multiple times.
//
// Currently only slices, arrays, strings, numeric types (including bools) and structures are supported.
// For other types and any custom logic you can implement [CustomEncoder] and [CustomDecoder].
//
// Pointers treated as just values. Nil pointer will be encoded as if it was
// valid pointer to zero-value (e.g. *int64 will be encoded as just int64(0)).
// When decoding, a new value will be allocated on the heap and filled with incoming data.
//
// Use of int and uint types are not recommended, because the sending and receiving machines can
// have different architecture (32 or 64 bit). Use fixed-siz types like uin32, int64, etc.
//
// When decoding slices or strings, there must be another integer field (any signed or
// unsigned type) before slice field with tag `sbin:"lenof:<TargetField>"`, otherwise
// error will be returned, e.g.:
//
//	type String struct {
//		Len uint32 `sbin:"lenof:Data"`
//		Data string
//	}
//
//	type StringSlice struct {
//		Len  uint32 `sbin:"lenof:Data"`
//		Data []String
//	}
//
//	type Slice[T any] struct {
//		Len  uint32 `sbin:"lenof:Data"`
//		Data []T
//	}
//
// Filling the length field on the encoder side is the called responsibility. Both length and data fields will be
// encoded as-is, e.g. when encoding `String{Len: 3, Data: "4444"}` will be encoded as `encode(3) + encode("4444")`.
//
// For slices, if length field is zero, then the data field will be set to zero-length slice (not nil).
func (d *Decoder) Decode(obj any, order binary.ByteOrder) error {
	val := reflect.ValueOf(obj)
	if val.Kind() != reflect.Pointer {
		return fmt.Errorf("obj must be a pointer, got: %v", val.Kind())
	}

	if val.IsNil() {
		return fmt.Errorf("obj must be a valid pointer, got nil")
	}

	// dereference
	val = val.Elem()
	return decode(val, d.r, order, nil)
}

func decode(val reflect.Value, from io.Reader, order binary.ByteOrder, size *int) error {
	// TODO: check for unexpected EOF

	switch v := val.Addr().Interface().(type) {
	case CustomDecoder:
		return v.Decode(from, order)
	}

	switch val.Kind() {
	case reflect.Bool:
		return decodeNumeric[bool](val, from, order)
	case reflect.Int:
		if val.Type().Size() == 4 {
			return decodeNumeric[int32](val, from, order)
		} else {
			return decodeNumeric[int64](val, from, order)
		}
	case reflect.Uint:
		if val.Type().Size() == 4 {
			return decodeNumeric[uint32](val, from, order)
		} else {
			return decodeNumeric[uint64](val, from, order)
		}
	case reflect.Int8:
		return decodeNumeric[int8](val, from, order)
	case reflect.Int16:
		return decodeNumeric[int16](val, from, order)
	case reflect.Int32:
		return decodeNumeric[int32](val, from, order)
	case reflect.Int64:
		return decodeNumeric[int64](val, from, order)
	case reflect.Uint8:
		return decodeNumeric[uint8](val, from, order)
	case reflect.Uint16:
		return decodeNumeric[uint16](val, from, order)
	case reflect.Uint32:
		return decodeNumeric[uint32](val, from, order)
	case reflect.Uint64:
		return decodeNumeric[uint64](val, from, order)
	case reflect.Float32:
		return decodeNumeric[float32](val, from, order)
	case reflect.Float64:
		return decodeNumeric[float64](val, from, order)

	case reflect.String:
		if size == nil {
			return fmt.Errorf("size of slice not specified")
		}
		if *size <= 0 {
			return nil // maybe set to 0 len slice if len is 0?
		}

		buf := make([]byte, *size)
		_, err := io.ReadFull(from, buf)
		if err != nil {
			return fmt.Errorf("can't decode a slice: %w", err)
		}

		val.SetString(string(buf))
		return nil

	case reflect.Slice:
		if size == nil {
			return fmt.Errorf("size of slice not specified")
		}
		if *size <= 0 {
			val.Set(reflect.MakeSlice(val.Type(), 0, 0))
			return nil // maybe set to 0 len slice if len is 0?
		}

		if val.Type().Elem().Kind() != reflect.Uint8 {
			return decodeSliceOrArray(val, from, order, *size)
		}

		buf := make([]byte, *size)
		_, err := io.ReadFull(from, buf)
		if err != nil {
			return fmt.Errorf("can't decode a slice: %w", err)
		}

		val.SetBytes(buf)
		return nil

	case reflect.Array:
		arrSize := val.Type().Len()
		if val.Type().Elem().Kind() != reflect.Uint8 {
			return decodeSliceOrArray(val, from, order, arrSize)
		}

		buf := make([]byte, arrSize)
		_, err := io.ReadFull(from, buf)
		if err != nil {
			return fmt.Errorf("can't decode an array: %w", err)
		}

		reflect.Copy(val, reflect.ValueOf(buf))
		return nil

	case reflect.Struct:
		// lengths of arbitary-sized fields, specified by tags
		lens := make(map[string]int)

		for i := range val.NumField() {
			fieldVal := val.Field(i)
			fieldInfo := val.Type().Field(i)
			fieldTag := fieldInfo.Tag.Get(_tag)

			if fieldTag == "-" || !fieldInfo.IsExported() {
				continue
			}

			var err error
			if size, ok := lens[fieldInfo.Name]; ok {
				err = decode(fieldVal, from, order, &size)
			} else {
				err = decode(fieldVal, from, order, nil)
			}
			if err != nil {
				return fmt.Errorf("can't decode field %v (%v): %w", fieldInfo.Name, fieldVal.Type().Name(), err)
			}

			// if current field specifies the length of another field - save it into the map
			if anotherField, size, ok := sizeOfAnotherField(fieldVal, fieldTag); ok {
				lens[anotherField] = size
			}
		}

	case reflect.Pointer:
		val.Set(reflect.New(val.Type().Elem()))

		return decode(val.Elem(), from, order, size)

	case reflect.Interface:
		// ignored
	}

	return nil
}

func decodeSliceOrArray(val reflect.Value, from io.Reader, order binary.ByteOrder, size int) error {
	if val.Type().Kind() == reflect.Slice {
		val.Grow(size)
		val.SetLen(size)
	}

	for i := range size {
		item := val.Index(i)
		if err := decode(item, from, order, nil); err != nil {
			return err
		}
	}
	return nil
}

func sizeOfAnotherField(val reflect.Value, tag string) (string, int, bool) {
	var size int
	switch {
	case val.CanInt():
		size = int(val.Int())
	case val.CanUint():
		size = int(val.Uint())
	case val.Kind() == reflect.Pointer: // so sizes could be specified as pointers, e.g. *int64 or event *****int64
		return sizeOfAnotherField(val.Elem(), tag)
	default:
		return "", 0, false
	}

	targetField, ok := strings.CutPrefix(tag, "lenof:")
	if !ok {
		return "", 0, false
	}

	return targetField, size, true
}

type fixedNumeric interface {
	int8 | int16 | int32 | int64 |
		uint8 | uint16 | uint32 | uint64 |
		float32 | float64 |
		bool
}

func decodeNumeric[T fixedNumeric](val reflect.Value, r io.Reader, order binary.ByteOrder) error {
	i, err := readNumeric[T](r, order)
	if err != nil {
		return err
	}

	val.Set(reflect.ValueOf(i).Convert(val.Type()))
	return nil
}

func readNumeric[T fixedNumeric](r io.Reader, order binary.ByteOrder) (T, error) {
	var v T
	if err := binary.Read(r, order, &v); err != nil {
		return v, fmt.Errorf("can't read %T: %w", v, err)
	}

	return v, nil
}
