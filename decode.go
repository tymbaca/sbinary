package sbinary

import (
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"strings"
)

const _tag = "bin"

type Decoder struct {
	r io.Reader
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

func (d *Decoder) Decode(obj any, order binary.ByteOrder) error {
	val := reflect.ValueOf(obj)
	if val.Kind() != reflect.Pointer {
		return fmt.Errorf("obj must be a pointer, got: %v", val.Kind())
	}

	if val.IsNil() {
		return fmt.Errorf("obj must be a pointer to initialized variable (not nil), got: %v", val.Kind())
	}

	// dereference
	val = val.Elem()
	return decode(val, d.r, order, nil)
}

func decode(val reflect.Value, from io.Reader, order binary.ByteOrder, size *int) error {
	ptr := reflect.New(val.Type()) // Create a pointer to the struct
	ptr.Elem().Set(val)            // Set the value of the new pointer to the current struct

	if e, ok := ptr.Interface().(Unmarshaler); ok {
		_, err := e.UnmarshalBinary(from, order)
		if err != nil {
			return err
		}

		val.Set(ptr.Elem()) // Update the original value with the unmarshaled data
		return nil
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
		if val.Type().Elem().Kind() != reflect.Uint8 {
			return fmt.Errorf("non-bytes slices are not supported")
		}

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

		val.SetBytes(buf)
		return nil

	case reflect.Array:
		if val.Type().Elem().Kind() != reflect.Uint8 {
			return fmt.Errorf("non-bytes arrays are not supported")
		}

		arrSize := val.Type().Len()
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

			if !fieldInfo.IsExported() {
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
