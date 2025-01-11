package sbinary

import (
	"encoding/binary"
	"io"
)

type Marshaler interface {
	MarshalBinary(w io.Writer, order binary.ByteOrder) (int, error)
}

type Unmarshaler interface {
	UnmarshalBinary(r io.Reader, order binary.ByteOrder) (int, error)
}
