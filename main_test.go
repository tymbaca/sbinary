package sbinary

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tymbaca/varint"
)

func TestEncodeDecode(t *testing.T) {
	t.Run("slice", func(t *testing.T) {
		test(t, Slice[String]{Len: 3, Data: []String{
			{Len: 5, Data: "hello"},
			{Len: 4, Data: "hell"},
			{Len: 3, Data: "hel"},
		}}, nil)
		test(t, Slice[String]{Len: 0, Data: []String{}}, nil)
	})
	t.Run("array", func(t *testing.T) {
		test(t, [3]String{
			{Len: 5, Data: "hello"},
			{Len: 4, Data: "hell"},
			{Len: 3, Data: "hel"},
		}, nil)
		test(t, [0]String{}, nil)
	})
	t.Run("custom", func(t *testing.T) {
		test(t, Custom{Optional: ptr(777.0)}, nil)
		test(t, Custom{Optional: nil}, nil)
	})
	t.Run("pointer", func(t *testing.T) {
		test(t, PtrWrapper{
			PtrStruct:   &Inner{Val: 10},
			PtrSliceLen: ptr[int64](3),
			PtrSlice:    &[]*int64{ptr[int64](1), ptr[int64](2), ptr[int64](3)},
		}, nil)
		test(t, PtrWrapper{
			PtrStruct:   nil,
			PtrSliceLen: nil,
			PtrSlice:    nil,
		}, func(v *PtrWrapper) {
			v.PtrStruct = new(Inner)
			v.PtrSliceLen = new(int64)
			v.PtrSlice = &[]*int64{}
		})
	})
	t.Run("numeric", func(t *testing.T) {
		test(t, Numerics{
			Int:   -1,
			Int8:  -2,
			Int16: -3,
			Int32: -4,
			Int64: -5,
		}, nil)
		test(t, Numerics{
			Int:   1,
			Int8:  2,
			Int16: 3,
			Int32: 4,
			Int64: 5,
		}, nil)
		test(t, Numerics{
			Int:   0,
			Int8:  0,
			Int16: 0,
			Int32: 0,
			Int64: 0,
		}, nil)
		test(t, Numerics{
			Uint:   1,
			Uint8:  2,
			Uint16: 3,
			Uint32: 4,
			Uint64: 5,
		}, nil)
		test(t, Numerics{
			Uint:   0,
			Uint8:  0,
			Uint16: 0,
			Uint32: 0,
			Uint64: 0,
		}, nil)
		test(t, Numerics{
			Float32: 11.1111,
			Float64: 12.1234,
		}, nil)
		test(t, Numerics{
			Float32: -11.1111,
			Float64: -12.1234,
		}, nil)
		test(t, Numerics{
			Float32: 0,
			Float64: 0,
		}, nil)
	})

	t.Run("full", func(t *testing.T) {
		req := Request{
			MessageSize: 52362,
			Header: Header{
				Version:       3,
				CorrelationID: 2,
				ClientID:      String{Len: 5, Data: "hello"},
				Numerics: Numerics{
					Int:     -1,
					Int8:    -2,
					Int16:   -3,
					Int32:   -4,
					Int64:   -5,
					Uint:    6,
					Uint8:   7,
					Uint16:  8,
					Uint32:  9,
					Uint64:  10,
					Float32: 11.11,
					Float64: 12.12,
				},
				Bool: true,
				ServerNames: Slice[String]{Len: 3, Data: []String{
					{Len: 5, Data: "hello"},
					{Len: 4, Data: "hell"},
					{Len: 3, Data: "hel"},
				}},
				ShitSize:        10,
				Shit:            []byte("1234567890"),
				Array:           [4]byte{12, 42, 1, 0},
				Pointer:         &Inner{Val: 20},
				TagIgnored:      111,
				InterfaceIgnore: &Inner{Val: 30},
				privateIgnored:  222,
			},
			CustomInt: varint.Int32(777),
			Custom: Custom{
				Optional: ptr(124.5),
			},
		}

		test(t, req, func(req *Request) {
			req.Header.TagIgnored = 0        // it will be ignored in decoded value
			req.Header.InterfaceIgnore = nil // it will be ignored in decoded value
			req.Header.privateIgnored = 0
		})
	})
}

func test[T any](t *testing.T, input T, middle func(v *T)) {
	buf := bytes.NewBuffer(nil)
	require.Nil(t, NewEncoder(buf).Encode(input, binary.BigEndian))

	var inputDecoded T
	require.NotNil(t, NewDecoder(buf).Decode(inputDecoded, binary.BigEndian)) // not pointer
	require.Nil(t, NewDecoder(buf).Decode(&inputDecoded, binary.BigEndian))

	if middle != nil {
		middle(&input)
	}

	require.Equal(t, input, inputDecoded)
}

func BenchmarkEncodeDecode(b *testing.B) {
	req := Request{
		MessageSize: 1,
		Header: Header{
			Version:       3,
			CorrelationID: 2,
			ClientID:      String{Len: 5, Data: "hello"},
			ServerNames: Slice[String]{Len: 3, Data: []String{
				{Len: 5, Data: "hello"},
				{Len: 4, Data: "hell"},
				{Len: 3, Data: "hel"},
			}},
			ShitSize:   10,
			Shit:       []byte("1234567890"),
			Array:      [4]byte{12, 42, 1, 0},
			TagIgnored: 64,
		},
		Custom: Custom{
			Optional: ptr(124.5),
		},
	}

	buf := bytes.NewBuffer(nil)
	for range b.N {
		NewEncoder(buf).Encode(req, binary.BigEndian)

		var reqDecoded Request
		NewDecoder(buf).Decode(&reqDecoded, binary.BigEndian)
	}
}

type Request struct {
	MessageSize uint32
	Custom      Custom
	Header      Header
	CustomInt   varint.Int32
}

type Header struct {
	Version         byte
	CorrelationID   int32
	ClientID        String
	Numerics        Numerics
	Bool            bool
	ServerNames     Slice[String]
	ShitSize        uint64 `sbin:"lenof:Shit"`
	Shit            []byte
	Array           [4]byte
	Pointer         *Inner
	TagIgnored      int64 `sbin:"-"`
	InterfaceIgnore any
	privateIgnored  int64
}

type Numerics struct {
	Int     int
	Int8    int8
	Int16   int16
	Int32   int32
	Int64   int64
	Uint    uint
	Uint8   uint8
	Uint16  uint16
	Uint32  uint32
	Uint64  uint64
	Float32 float32
	Float64 float64
}

type PtrWrapper struct {
	PtrStruct   *Inner
	PtrSliceLen *int64 `sbin:"lenof:PtrSlice"`
	PtrSlice    *[]*int64
}

type Inner struct {
	Val int
}

type String struct {
	Len  uint32 `sbin:"lenof:Data"`
	Data string
}

type Slice[T any] struct {
	Len  uint32 `sbin:"lenof:Data"`
	Data []T
}

type Custom struct {
	Optional *float64
}

func (c Custom) Encode(w io.Writer, order binary.ByteOrder) error {
	if c.Optional != nil {
		if err := binary.Write(w, order, true); err != nil {
			return err
		}

		return binary.Write(w, order, *c.Optional)
	}

	return binary.Write(w, order, false)
}

func (c *Custom) Decode(r io.Reader, order binary.ByteOrder) error {
	var set bool
	if err := binary.Read(r, order, &set); err != nil {
		return err
	}

	if set {
		var v float64
		if err := binary.Read(r, order, &v); err != nil {
			return err
		}
		c.Optional = &v
	}

	return nil
}

func ptr[T any](v T) *T {
	return &v
}
