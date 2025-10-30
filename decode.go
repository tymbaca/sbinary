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
// Currently only numeric types, slices, strings, arrays and structures are supported.
// For other types and any custom logic you can implement [CustomEncoder] and [CustomDecoder].
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
	case reflect.Int8:
		i, err := readInt[int8](from, order)
		if err != nil {
			return err
		}

		val.SetInt(int64(i))
		return nil

	case reflect.Int16:
		i, err := readInt[int16](from, order)
		if err != nil {
			return err
		}

		val.SetInt(int64(i))
		return nil

	case reflect.Int32:
		i, err := readInt[int32](from, order)
		if err != nil {
			return err
		}

		val.SetInt(int64(i))
		return nil

	case reflect.Int64:
		i, err := readInt[int64](from, order)
		if err != nil {
			return err
		}

		val.SetInt(int64(i))
		return nil

	case reflect.Uint8:
		i, err := readUint[uint8](from, order)
		if err != nil {
			return err
		}

		val.SetUint(uint64(i))
		return nil

	case reflect.Uint16:
		i, err := readUint[uint16](from, order)
		if err != nil {
			return err
		}

		val.SetUint(uint64(i))
		return nil

	case reflect.Uint32:
		i, err := readUint[uint32](from, order)
		if err != nil {
			return err
		}

		val.SetUint(uint64(i))
		return nil

	case reflect.Uint64:
		i, err := readUint[uint64](from, order)
		if err != nil {
			return err
		}

		val.SetUint(uint64(i))
		return nil

		// todo float

	case reflect.String:
		// todo avoid code duplication
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

	default:
		// ignore
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
	default:
		return "", 0, false
	}

	targetField, ok := strings.CutPrefix(tag, "lenof:")
	if !ok {
		return "", 0, false
	}

	return targetField, size, true
}

func readInt[I int8 | int16 | int32 | int64](r io.Reader, order binary.ByteOrder) (I, error) {
	var i I
	if err := binary.Read(r, order, &i); err != nil {
		return i, fmt.Errorf("can't read int: %w", err)
	}

	return i, nil
}

func readUint[U uint8 | uint16 | uint32 | uint64](r io.Reader, order binary.ByteOrder) (U, error) {
	var u U
	if err := binary.Read(r, order, &u); err != nil {
		return u, fmt.Errorf("can't read uint: %w", err)
	}

	return u, nil
}
