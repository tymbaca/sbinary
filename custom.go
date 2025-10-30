package sbinary

import (
	"encoding/binary"
	"io"
)

type CustomEncoder interface {
	Encode(w io.Writer, order binary.ByteOrder) error
}

type CustomDecoder interface {
	Decode(r io.Reader, order binary.ByteOrder) error
}
