package mt

import (
	"bytes"
	"fmt"
	"io"
)

var (
	// the last field in a message is followed by an "End of Text" (CrLf-)
	eot = []byte{'\r', '\n', '-'}
)

type Decoder struct {
	data   []byte
	offset int
	pos    int

	step    func(*Decoder, []byte) ([]byte, error)
	stack   int
	isField bool
}

func NewDecoder(data []byte) *Decoder {
	return &Decoder{
		data: data,
		step: stateBeginBlock,
	}
}

/*
func (d *Decoder) Decode(v interface{}) error {
	rv := reflect.ValueOf(v)
	switch {
	case rv.Kind() != reflect.Ptr:
		return fmt.Errorf("non-pointer %v", reflect.TypeOf(v))
	case rv.IsNil():
		return fmt.Errorf("nil pointer")
	default:
		return d.decode(rv.Elem())
	}
}

func (d *Decoder) decode(v reflect.Value) error {
	tok, err := d.NextToken()
	if err != nil {
		return err
	}
	switch tok[0] {
	case '{':
		switch v.Kind() {
		case reflect.Map:
			m, err := d.decodeMap()
			if err != nil {
				return err
			}
			v.Set(reflect.ValueOf(m))
		}
	}
	return nil
}

func (d *Decoder) decodeMap() (map[string]interface{}, error) {
	m := make(map[string]interface{})
	for {
		tok, err := d.NextToken()
		if err != nil {
			return nil, err
		}
		if tok[0] == '}' {
			return m, nil
		}

		key := string(tok)
		val, err := d.decodeValue()
		if err != nil {
			return nil, err
		}
		m[key] = val
	}
}

func (d *Decoder) decodeValue() (interface{}, error) {
	tok, err := d.NextToken()
	if err != nil {
		return nil, err
	}
	switch tok[0] {
	case '{':
		return d.decodeMap()
	case ':':
		return d.decodeMap()
	default:
		return string(tok), nil
	}
}
*/

func (d *Decoder) NextToken() ([]byte, error) {
	d.move(d.pos)
	if d.eof() {
		if d.stack != 0 {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, io.EOF
	}
	tok := d.readToken()
	if len(tok) < 1 {
		return nil, io.ErrUnexpectedEOF
	}
	return d.step(d, tok)
}

func (d *Decoder) remaining() []byte {
	if d.offset >= len(d.data) {
		return nil
	}
	return d.data[d.offset:]
}

func (d *Decoder) move(n int) {
	d.offset += n
}

func (d *Decoder) eof() bool {
	return len(d.data[d.offset:]) == 0
}

func (d *Decoder) readToken() []byte {
	data := d.remaining()
	for pos, c := range data {
		// ignore past whitespaces
		if isWhitespace(c) {
			continue
		}

		switch c {
		case '{', '}', ':':
			d.pos = pos + 1
			return data[pos:d.pos]
		}

		d.pos = d.scanIdent()
		return data[:d.pos]
	}
	return nil
}

func (d *Decoder) scanIdent() int {
	pos := 0
	for _, c := range d.remaining() {
		if !isIdent(c) {
			break
		}
		pos++
	}
	return pos
}

func (d *Decoder) pushStack() {
	d.stack++
}

func (d *Decoder) popStack() int {
	d.stack--
	return d.stack
}

func isIdent(c byte) bool {
	switch c {
	case ':', '{', '}':
		return false
	}
	return true
}

func isWhitespace(c byte) bool {
	return c <= ' ' && (c == ' ' || c == '\r' || c == '\n' || c == '\t')
}

func stateBeginBlock(s *Decoder, tok []byte) ([]byte, error) {
	switch tok[0] {
	case '{':
		s.step = stateInsideBlock
		s.pushStack()
	default:
		return nil, fmt.Errorf("stateBeginBlock: expected block open, got %s", tok)
	}
	return tok, nil
}

func stateInsideBlock(s *Decoder, tok []byte) ([]byte, error) {
	switch tok[0] {
	case '1', '2', '3', '4', '5':
		// 1: basic header block
		// 2: application header block
		// 3: user header block
		// 4: text block (body)
		// 5: trailer block
		s.step = stateBlockDelim
	case '}':
		s.popStack()
		s.step = stateBeginBlock
	default:
		return nil, fmt.Errorf("stateInsideBlock: unknown block identifier %s", tok)
	}
	return tok, nil
}

func stateBlockDelim(s *Decoder, tok []byte) ([]byte, error) {
	switch tok[0] {
	case ':':
		if s.isField {
			s.isField = false
			s.step = stateBlockFieldValue
		} else {
			s.step = stateBlockValue
		}
		return s.NextToken()
	default:
		return nil, fmt.Errorf("stateBlockDelim: expected delimeter, got %s", tok)
	}
}

func stateBlockValue(s *Decoder, tok []byte) ([]byte, error) {
	switch tok[0] {
	case '{':
		s.step = stateBlockTag
		s.pushStack()
	case ':':
		s.step = stateBlockTag
		s.isField = true
	case '}':
		if s.popStack() > 0 {
			s.step = stateBlockValue
		} else {
			s.step = stateBeginBlock
		}
	default:
		s.step = stateBlockValue
	}
	return tok, nil
}

func stateBlockTag(s *Decoder, tok []byte) ([]byte, error) {
	switch tok[0] {
	case '}':
		if s.popStack() > 0 {
			return nil, fmt.Errorf("stateBlockTag: malformed state")
		}
		s.step = stateBlockValue
	default:
		s.step = stateBlockDelim
	}
	return tok, nil
}

func stateBlockFieldValue(s *Decoder, tok []byte) ([]byte, error) {
	if bytes.HasSuffix(tok, []byte{'\r', '\n'}) {
		tok = tok[:len(tok)-2]
	} else {
		tok = bytes.TrimSuffix(tok, []byte{'\n'})
	}

	s.step = stateBlockValue

	return tok, nil
}
