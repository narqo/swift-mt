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

func (s *Decoder) NextToken() ([]byte, error) {
	s.move(s.pos)
	if s.eof() {
		if s.stack != 0 {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, io.EOF
	}
	tok := s.readToken()
	if len(tok) < 1 {
		return nil, io.ErrUnexpectedEOF
	}
	return s.step(s, tok)
}

func (s *Decoder) remaining() []byte {
	if s.offset >= len(s.data) {
		return nil
	}
	return s.data[s.offset:]
}

func (s *Decoder) move(n int) {
	s.offset += n
}

func (s *Decoder) eof() bool {
	return len(s.data[s.offset:]) == 0
}

func (s *Decoder) readToken() []byte {
	data := s.remaining()
	for pos, c := range data {
		// ignore past whitespaces
		if isWhitespace(c) {
			continue
		}

		switch c {
		case '{', '}', ':':
			s.pos = pos + 1
			return data[pos:s.pos]
		}

		s.pos = s.scanIdent()
		return data[:s.pos]
	}
	return nil
}

func (s *Decoder) scanIdent() int {
	pos := 0
	for _, c := range s.remaining() {
		if !isIdent(c) {
			break
		}
		pos++
	}
	return pos
}

func (s *Decoder) pushStack() {
	s.stack++
}

func (s *Decoder) popStack() int {
	s.stack--
	return s.stack
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
		s.step = stateBlockTagKey
		s.pushStack()
	case ':':
		s.step = stateBlockFieldKey
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

func stateBlockTagKey(s *Decoder, tok []byte) ([]byte, error) {
	switch tok[0] {
	case '}':
		if s.popStack() > 0 {
			return nil, fmt.Errorf("stateBlockTagKey: malformed state")
		}
		s.step = stateBlockValue
	default:
		s.step = stateBlockDelim
	}
	return tok, nil
}

func stateBlockFieldKey(s *Decoder, tok []byte) ([]byte, error) {
	s.step = stateBlockDelim
	s.isField = true
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
