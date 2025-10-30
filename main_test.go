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
	t.Run("full", func(t *testing.T) {
		req := Request{
			MessageSize: 52362,
			Header: Header{
				Version:       3,
				CorrelationID: 2,
				ClientID:      String{Len: 5, Data: "hello"},
				ServerNames: Slice[String]{Len: 3, Data: []String{
					{Len: 5, Data: "hello"},
					{Len: 4, Data: "hell"},
					{Len: 3, Data: "hel"},
				}},
				ShitSize:     10,
				Shit:         []byte("1234567890"),
				Array:        [4]byte{12, 42, 1, 0},
				Ignored:      111,
				alsoIgnored:  222,
				alsoIgnored2: &Inner{Val: 20},
			},
			CustomInt: varint.Int32(777),
			Custom: Custom{
				Price:  124.5,
				Active: true,
			},
		}

		test(t, req, func(req *Request) {
			req.Header.Ignored = 0 // it will be ignored in decoded value
			req.Header.alsoIgnored = 0
			req.Header.alsoIgnored2 = nil
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
			ShitSize: 10,
			Shit:     []byte("1234567890"),
			Array:    [4]byte{12, 42, 1, 0},
			Ignored:  64,
		},
		Custom: Custom{
			Price:  124.5,
			Active: true,
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
	Header      Header
	CustomInt   varint.Int32
	Custom      Custom
}

type Header struct {
	Version       byte
	CorrelationID int32
	ClientID      String
	ServerNames   Slice[String]
	ShitSize      uint64 `sbin:"lenof:Shit"`
	Shit          []byte
	Array         [4]byte
	Ignored       int64 `sbin:"-"`
	alsoIgnored   int64
	alsoIgnored2  *Inner
	InnerSet      bool
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
	Price  float64
	Active bool
}

func (c Custom) MarshalBinary(w io.Writer, order binary.ByteOrder) (int, error) {
	if err := binary.Write(w, order, c.Price); err != nil {
		return 0, err
	}

	if err := binary.Write(w, order, c.Active); err != nil {
		return 0, err
	}

	return 9, nil
}

func (c *Custom) UnmarshalBinary(r io.Reader, order binary.ByteOrder) (int, error) {
	if err := binary.Read(r, order, &c.Price); err != nil {
		return 0, err
	}

	if err := binary.Read(r, order, &c.Active); err != nil {
		return 0, err
	}

	return 9, nil
}
