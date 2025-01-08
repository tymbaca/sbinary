package sbinary

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeDecode(t *testing.T) {
	req := Request{
		MessageSize: 52362,
		Header: Header{
			Version:       3,
			CorrelationID: 2,
			ClientID:      String{Len: 5, Data: "hello"},
			ShitSize:      10,
			Shit:          []byte("1234567890"),
		},
		Custom: Custom{
			Price:  124.5,
			Active: true,
		},
	}

	buf := bytes.NewBuffer(nil)
	require.Nil(t, NewEncoder(buf).Encode(req, binary.BigEndian))

	fmt.Println(buf.Bytes())

	var reqDecoded Request
	require.NotNil(t, NewDecoder(buf).Decode(reqDecoded, binary.BigEndian)) // not pointer
	require.Nil(t, NewDecoder(buf).Decode(&reqDecoded, binary.BigEndian))

	require.Equal(t, req, reqDecoded)
}

func BenchmarkEncodeDecode(b *testing.B) {
	req := Request{
		MessageSize: 1,
		Header: Header{
			Version:       3,
			CorrelationID: 2,
			ClientID:      String{Len: 5, Data: "hello"},
			ShitSize:      10,
			Shit:          []byte("1234567890"),
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
	Custom      Custom
}

type Header struct {
	Version       byte
	CorrelationID int32
	ClientID      String
	ShitSize      uint64 `bin:"lenof:Shit"`
	Shit          []byte
}

type String struct {
	Len  int32 `bin:"lenof:Data"`
	Data string
}

type Custom struct {
	Price  float64
	Active bool
}

func (c *Custom) MarshalBinary(w io.Writer, order binary.ByteOrder) error {
	if err := binary.Write(w, order, c.Price); err != nil {
		return err
	}

	if err := binary.Write(w, order, c.Active); err != nil {
		return err
	}

	return nil
}

func (c *Custom) UnmarshalBinary(r io.Reader, order binary.ByteOrder) error {
	if err := binary.Read(r, order, &c.Price); err != nil {
		return err
	}

	if err := binary.Read(r, order, &c.Active); err != nil {
		return err
	}

	return nil
}
