package encoder

import (
	"bytes"
	"fmt"
	"strconv"
	"unsafe"

	"github.com/goccy/go-json/internal/errors"
	"github.com/goccy/go-json/internal/runtime"
)

var (
	isWhiteSpace = [256]bool{
		' ':  true,
		'\n': true,
		'\t': true,
		'\r': true,
	}
	isHTMLEscapeChar = [256]bool{
		'<': true,
		'>': true,
		'&': true,
	}
	nul = byte('\000')
)

func Compact(buf *bytes.Buffer, src []byte, escape bool) error {
	if len(src) == 0 {
		return errors.ErrUnexpectedEndOfJSON("", 0)
	}
	dst := make([]byte, 0, len(src))
	dst, err := compact(dst, src, escape)
	if err != nil {
		return err
	}
	if _, err := buf.Write(dst); err != nil {
		return err
	}
	return nil
}

func compact(dst, src []byte, escape bool) ([]byte, error) {
	src = append(src, nul)
	buf, _, err := compactValue(dst, src, 0, escape)
	if err != nil {
		return nil, err
	}
	(*runtime.SliceHeader)(unsafe.Pointer(&src)).Len--
	return buf, nil
}

func skipWhiteSpace(buf []byte, cursor int64) int64 {
LOOP:
	if isWhiteSpace[buf[cursor]] {
		cursor++
		goto LOOP
	}
	return cursor
}

func compactValue(dst, src []byte, cursor int64, escape bool) ([]byte, int64, error) {
	for {
		switch src[cursor] {
		case ' ', '\t', '\n', '\r':
			cursor++
			continue
		case '{':
			return compactObject(dst, src, cursor, escape)
		case '}':
			return nil, 0, errors.ErrSyntax("unexpected character '}'", cursor)
		case '[':
			return compactArray(dst, src, cursor, escape)
		case ']':
			return nil, 0, errors.ErrSyntax("unexpected character ']'", cursor)
		case '"':
			return compactString(dst, src, cursor, escape)
		case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return compactNumber(dst, src, cursor)
		case 't':
			return compactTrue(dst, src, cursor)
		case 'f':
			return compactFalse(dst, src, cursor)
		case 'n':
			return compactNull(dst, src, cursor)
		default:
			return nil, 0, errors.ErrSyntax(fmt.Sprintf("unexpected character '%c'", src[cursor]), cursor)
		}
	}
}

func compactObject(dst, src []byte, cursor int64, escape bool) ([]byte, int64, error) {
	switch src[cursor] {
	case 'n':
		return compactNull(dst, src, cursor)
	case '{':
		dst = append(dst, '{')
	default:
		return nil, 0, errors.ErrExpected("expected { character for object value", cursor)
	}
	cursor = skipWhiteSpace(src, cursor+1)
	if src[cursor] == '}' {
		dst = append(dst, '}')
		return dst, cursor + 1, nil
	}
	var err error
	for {
		cursor = skipWhiteSpace(src, cursor)
		dst, cursor, err = compactString(dst, src, cursor, escape)
		if err != nil {
			return nil, 0, err
		}
		cursor = skipWhiteSpace(src, cursor)
		if src[cursor] != ':' {
			return nil, 0, errors.ErrExpected("colon after object key", cursor)
		}
		dst = append(dst, ':')
		dst, cursor, err = compactValue(dst, src, cursor+1, escape)
		if err != nil {
			return nil, 0, err
		}
		cursor = skipWhiteSpace(src, cursor)
		switch src[cursor] {
		case '}':
			dst = append(dst, '}')
			cursor++
			return dst, cursor, nil
		case ',':
			dst = append(dst, ',')
		default:
			return nil, 0, errors.ErrExpected("comma after object value", cursor)
		}
		cursor++
	}
}

func compactArray(dst, src []byte, cursor int64, escape bool) ([]byte, int64, error) {
	switch src[cursor] {
	case 'n':
		return compactNull(dst, src, cursor)
	case '[':
		dst = append(dst, '[')
	default:
		return nil, 0, errors.ErrExpected("expected [ character for array value", cursor)
	}
	cursor = skipWhiteSpace(src, cursor+1)
	if src[cursor] == ']' {
		dst = append(dst, ']')
		return dst, cursor + 1, nil
	}
	var err error
	for {
		dst, cursor, err = compactValue(dst, src, cursor, escape)
		if err != nil {
			return nil, 0, err
		}
		cursor = skipWhiteSpace(src, cursor)
		switch src[cursor] {
		case ']':
			dst = append(dst, ']')
			cursor++
			return dst, cursor, nil
		case ',':
			dst = append(dst, ',')
		default:
			return nil, 0, errors.ErrExpected("comma after array value", cursor)
		}
		cursor++
	}
}

func compactString(dst, src []byte, cursor int64, escape bool) ([]byte, int64, error) {
	if src[cursor] != '"' {
		return nil, 0, errors.ErrInvalidCharacter(src[cursor], "string", cursor)
	}
	start := cursor
	for {
		cursor++
		c := src[cursor]
		if escape {
			if isHTMLEscapeChar[c] {
				dst = append(dst, src[start:cursor]...)
				dst = append(dst, `\u00`...)
				dst = append(dst, hex[c>>4], hex[c&0xF])
				start = cursor + 1
			} else if c == 0xE2 && cursor+2 < int64(len(src)) && src[cursor+1] == 0x80 && src[cursor+2]&^1 == 0xA8 {
				dst = append(dst, src[start:cursor]...)
				dst = append(dst, `\u202`...)
				dst = append(dst, hex[src[cursor+2]&0xF])
				cursor += 2
				start = cursor + 3
			}
		}
		switch c {
		case '\\':
			cursor++
			if src[cursor] == nul {
				return nil, 0, errors.ErrUnexpectedEndOfJSON("string", int64(len(src)))
			}
		case '"':
			cursor++
			return append(dst, src[start:cursor]...), cursor, nil
		case nul:
			return nil, 0, errors.ErrUnexpectedEndOfJSON("string", int64(len(src)))
		}
	}
}

func compactNumber(dst, src []byte, cursor int64) ([]byte, int64, error) {
	start := cursor
	for {
		cursor++
		if floatTable[src[cursor]] {
			continue
		}
		break
	}
	num := src[start:cursor]
	if _, err := strconv.ParseFloat(*(*string)(unsafe.Pointer(&num)), 64); err != nil {
		return nil, 0, err
	}
	dst = append(dst, num...)
	return dst, cursor, nil
}

func compactTrue(dst, src []byte, cursor int64) ([]byte, int64, error) {
	if cursor+3 >= int64(len(src)) {
		return nil, 0, errors.ErrUnexpectedEndOfJSON("true", cursor)
	}
	if !bytes.Equal(src[cursor:cursor+4], []byte(`true`)) {
		return nil, 0, errors.ErrInvalidCharacter(src[cursor], "true", cursor)
	}
	dst = append(dst, "true"...)
	cursor += 4
	return dst, cursor, nil
}

func compactFalse(dst, src []byte, cursor int64) ([]byte, int64, error) {
	if cursor+4 >= int64(len(src)) {
		return nil, 0, errors.ErrUnexpectedEndOfJSON("false", cursor)
	}
	if !bytes.Equal(src[cursor:cursor+5], []byte(`false`)) {
		return nil, 0, errors.ErrInvalidCharacter(src[cursor], "false", cursor)
	}
	dst = append(dst, "false"...)
	cursor += 5
	return dst, cursor, nil
}

func compactNull(dst, src []byte, cursor int64) ([]byte, int64, error) {
	if cursor+3 >= int64(len(src)) {
		return nil, 0, errors.ErrUnexpectedEndOfJSON("null", cursor)
	}
	if !bytes.Equal(src[cursor:cursor+4], []byte(`null`)) {
		return nil, 0, errors.ErrInvalidCharacter(src[cursor], "null", cursor)
	}
	dst = append(dst, "null"...)
	cursor += 4
	return dst, cursor, nil
}
