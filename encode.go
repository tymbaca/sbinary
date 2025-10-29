package sbinary

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unsafe"
)

func Marshal(data any, order binary.ByteOrder) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	buf.Grow(int(unsafe.Sizeof(data)))

	if err := NewEncoder(buf).Encode(data, order); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type Encoder struct {
	w io.Writer
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

func (e *Encoder) Encode(data any, order binary.ByteOrder) error {
	val := reflect.ValueOf(data)

	return encode(val, e.w, order)
}

var marshalerType = reflect.TypeFor[Marshaler]()

func encode(val reflect.Value, into io.Writer, order binary.ByteOrder) error {
	valType := val.Type()
	if reflect.PointerTo(valType).Implements(marshalerType) {
		// Handle custom unmarshaler with reflection to ensure pointer
		ptr := reflect.New(valType) // Create a pointer to the struct
		ptr.Elem().Set(val)         // Set the value of the new pointer to the current struct
		_, err := ptr.Interface().(Marshaler).MarshalBinary(into, order)
		return err
	}

	if strings.Contains(val.Type().Name(), "Custom") {
		panic("custom didn't match iface")
	}

	switch val.Kind() {
	case reflect.Int8:
		return writeInt[int8](into, order, val)

	case reflect.Int16:
		return writeInt[int16](into, order, val)

	case reflect.Int32:
		return writeInt[int32](into, order, val)

	case reflect.Int64:
		return writeInt[int64](into, order, val)

	case reflect.Uint8:
		return writeUint[uint8](into, order, val)

	case reflect.Uint16:
		return writeUint[uint16](into, order, val)

	case reflect.Uint32:
		return writeUint[uint32](into, order, val)

	case reflect.Uint64:
		return writeUint[uint64](into, order, val)

		// todo float

	case reflect.String:
		_, err := into.Write([]byte(val.String()))
		return err

	case reflect.Slice:
		elemKind := val.Type().Elem().Kind()
		if elemKind == reflect.Uint8 {
			_, err := into.Write(val.Bytes())
			return err
		}

		return encodeSliceOrArray(val, into, order)

	case reflect.Array:
		elemKind := val.Type().Elem().Kind()
		if elemKind == reflect.Uint8 {
			sliceVal := arrayToSlice(val)
			_, err := into.Write(sliceVal.Bytes())
			return err
		}

		return encodeSliceOrArray(val, into, order)

	case reflect.Struct:
		for i := range val.NumField() {
			fieldVal := val.Field(i)
			// for debug purposes
			fieldInfo := val.Type().Field(i)
			fieldTag := fieldInfo.Tag.Get(_tag)
			_ = fieldTag

			if fieldTag == "-" || !fieldInfo.IsExported() {
				continue
			}

			err := encode(fieldVal, into, order)
			if err != nil {
				return fmt.Errorf("can't encode field %v (%v): %w", fieldInfo.Name, fieldVal.Type().Name(), err)
			}
		}

		return nil

	default:
		fmt.Println("ignoring field:", val) // TODO: remove
	}

	return nil
}

func encodeSliceOrArray(val reflect.Value, into io.Writer, order binary.ByteOrder) error {
	for i := 0; i < val.Len(); i++ {
		item := val.Index(i)
		if err := encode(item, into, order); err != nil {
			return err
		}
	}

	return nil
}

func writeInt[I int8 | int16 | int32 | int64](into io.Writer, order binary.ByteOrder, val reflect.Value) error {
	i := I(val.Int())
	return binary.Write(into, order, i)
}

func writeUint[U uint8 | uint16 | uint32 | uint64](into io.Writer, order binary.ByteOrder, val reflect.Value) error {
	u := U(val.Uint())
	return binary.Write(into, order, u)
}

func arrayToSlice(arr reflect.Value) reflect.Value {
	ptr := reflect.New(arr.Type()).Elem()
	ptr.Set(arr)

	return ptr.Slice(0, ptr.Len())
}
