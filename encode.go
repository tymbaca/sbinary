package sbinary

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"unsafe"
)

// Marshal marshals obj into bytes using provided byte order.
// See [Encoder.Encode] for details.
func Marshal(obj any, order binary.ByteOrder) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	buf.Grow(int(unsafe.Sizeof(obj)))

	if err := NewEncoder(buf).Encode(obj, order); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Encoder encodes incoming bytes into Go objects.
type Encoder struct {
	w io.Writer
}

// NewEncoder created an [Encoder] that will use w for the output.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

// Encode encodes obj. For numeric fields it uses provided byte order.
// It can be called multiple times.
//
// See [Decoder.Decode] comment for more info.
// When encoding, nil slice and zero-length slice are encoded
// in the same way, no bytes will be written.
func (e *Encoder) Encode(data any, order binary.ByteOrder) error {
	val := reflect.ValueOf(data)

	return encode(val, e.w, order)
}

var customEncoderType = reflect.TypeFor[CustomEncoder]()

func encode(val reflect.Value, into io.Writer, order binary.ByteOrder) error {
	valType := val.Type()
	if reflect.PointerTo(valType).Implements(customEncoderType) {
		// Handle custom unmarshaler with reflection to ensure pointer
		ptr := reflect.New(valType) // Create a pointer to the struct
		ptr.Elem().Set(val)         // Set the value of the new pointer to the current struct
		return ptr.Interface().(CustomEncoder).Encode(into, order)
	}

	switch val.Kind() {
	case reflect.Bool:
		return writeNumeric[bool](into, order, val)
	case reflect.Int:
		if val.Type().Size() == 4 {
			return writeNumeric[int32](into, order, reflect.ValueOf(int32(val.Int())))
		} else {
			return writeNumeric[int64](into, order, reflect.ValueOf(int64(val.Int())))
		}
	case reflect.Uint:
		if val.Type().Size() == 4 {
			return writeNumeric[uint32](into, order, reflect.ValueOf(uint32(val.Uint())))
		} else {
			return writeNumeric[uint64](into, order, reflect.ValueOf(uint64(val.Uint())))
		}
	case reflect.Int8:
		return writeNumeric[int8](into, order, val)
	case reflect.Int16:
		return writeNumeric[int16](into, order, val)
	case reflect.Int32:
		return writeNumeric[int32](into, order, val)
	case reflect.Int64:
		return writeNumeric[int64](into, order, val)
	case reflect.Uint8:
		return writeNumeric[uint8](into, order, val)
	case reflect.Uint16:
		return writeNumeric[uint16](into, order, val)
	case reflect.Uint32:
		return writeNumeric[uint32](into, order, val)
	case reflect.Uint64:
		return writeNumeric[uint64](into, order, val)
	case reflect.Float32:
		return writeNumeric[float32](into, order, val)
	case reflect.Float64:
		return writeNumeric[float64](into, order, val)

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

	case reflect.Pointer:
		if val.IsNil() {
			val = reflect.New(val.Type().Elem())
		}

		return encode(val.Elem(), into, order)

	case reflect.Interface:
		// ignored
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

func writeNumeric[T fixedNumeric](into io.Writer, order binary.ByteOrder, val reflect.Value) error {
	return binary.Write(into, order, val.Interface())
}

func arrayToSlice(arr reflect.Value) reflect.Value {
	ptr := reflect.New(arr.Type()).Elem()
	ptr.Set(arr)

	return ptr.Slice(0, ptr.Len())
}
