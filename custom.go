package sbinary

import (
	"encoding/binary"
	"io"
)

type Marshaler interface {
	MarshalBinary(w io.Writer, order binary.ByteOrder) error
}

type Unmarshaler interface {
	UnmarshalBinary(r io.Reader, order binary.ByteOrder) error
}
