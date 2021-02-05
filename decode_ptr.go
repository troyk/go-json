package json

import (
	"unsafe"
)

type ptrDecoder struct {
	dec        decoder
	typ        *rtype
	structName string
	fieldName  string
}

func newPtrDecoder(dec decoder, typ *rtype, structName, fieldName string) *ptrDecoder {
	return &ptrDecoder{
		dec:        dec,
		typ:        typ,
		structName: structName,
		fieldName:  fieldName,
	}
}

func (d *ptrDecoder) contentDecoder() decoder {
	dec, ok := d.dec.(*ptrDecoder)
	if !ok {
		return d.dec
	}
	return dec.contentDecoder()
}

//nolint:golint
//go:linkname unsafe_New reflect.unsafe_New
func unsafe_New(*rtype) unsafe.Pointer

func (d *ptrDecoder) decodeStream(s *stream, p unsafe.Pointer) error {
	s.skipWhiteSpace()
	if s.char() == nul {
		s.read()
	}
	if s.char() == 'n' {
		if err := nullBytes(s); err != nil {
			return err
		}
		*(*unsafe.Pointer)(p) = nil
		return nil
	}
	newptr := unsafe_New(d.typ)
	*(*unsafe.Pointer)(p) = newptr
	if err := d.dec.decodeStream(s, newptr); err != nil {
		return err
	}
	return nil
}

func (d *ptrDecoder) decode(buf *sliceHeader, cursor int64, p unsafe.Pointer) (int64, error) {
	cursor = skipWhiteSpace(buf, cursor)
	if char(buf.data, cursor) == 'n' {
		buflen := int64(buf.len)
		if cursor+3 >= buflen {
			return 0, errUnexpectedEndOfJSON("null", cursor)
		}
		if char(buf.data, cursor+1) != 'u' {
			return 0, errInvalidCharacter(char(buf.data, cursor+1), "null", cursor)
		}
		if char(buf.data, cursor+2) != 'l' {
			return 0, errInvalidCharacter(char(buf.data, cursor+2), "null", cursor)
		}
		if char(buf.data, cursor+3) != 'l' {
			return 0, errInvalidCharacter(char(buf.data, cursor+3), "null", cursor)
		}
		*(*unsafe.Pointer)(p) = nil
		cursor += 4
		return cursor, nil
	}
	newptr := unsafe_New(d.typ)
	*(*unsafe.Pointer)(p) = newptr
	c, err := d.dec.decode(buf, cursor, newptr)
	if err != nil {
		return 0, err
	}
	cursor = c
	return cursor, nil
}
