package sbinary

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
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

func encode(val reflect.Value, into io.Writer, order binary.ByteOrder) error {
	// Handle custom unmarshaler with reflection to ensure pointer
	ptr := reflect.New(val.Type()) // Create a pointer to the struct
	ptr.Elem().Set(val)            // Set the value of the new pointer to the current struct

	if e, ok := ptr.Interface().(Marshaler); ok {
		_, err := e.MarshalBinary(into, order)
		return err
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

	case reflect.Slice:
		elemKind := val.Type().Elem().Kind()
		if elemKind == reflect.Uint8 {
			_, err := into.Write(val.Bytes())
			return err
		}

	case reflect.String:
		_, err := into.Write([]byte(val.String()))
		return err

	case reflect.Struct:
		for i := range val.NumField() {
			fieldVal := val.Field(i)
			// for debug purposes
			fieldInfo := val.Type().Field(i)
			fieldTag := fieldInfo.Tag.Get(_tag)
			_ = fieldTag

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

func writeInt[I int8 | int16 | int32 | int64](into io.Writer, order binary.ByteOrder, val reflect.Value) error {
	i := I(val.Int())
	return binary.Write(into, order, i)
}

func writeUint[U uint8 | uint16 | uint32 | uint64](into io.Writer, order binary.ByteOrder, val reflect.Value) error {
	u := U(val.Uint())
	return binary.Write(into, order, u)
}
