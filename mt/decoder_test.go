package mt_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/narqo/swift-mt/mt"
)

const testData = `{1:F01YOURCODEZABC1234123456}{2:I103SOGEFRPPZXXXU3003}{3:{103:TGT}{108:OPTUSERREF16CHAR}}{4:
:16R:USECU
:35B:ISIN CH0101010101
/XS/232323232
FINANCIAL INSTRUMENT ACME
-}{5:{AA:11}}`

//const testData = `{1:F01YOURCODEZABC1234123456}{2:I103SOGEFRPPZXXXU3003}{3:{103:TGT}{108:OPTUSERREF16CHAR}}{5:{AA:11}}`

var mtData = bytes.ReplaceAll([]byte(testData), []byte("\n"), []byte("\r\n"))

func TestDecoder_NextToken(t *testing.T) {
	dec := mt.NewDecoder(mtData)
	for {
		token, err := dec.NextToken()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("token %s\n", token)
	}
}

//func TestDecoder_Decode(t *testing.T) {
//	m := make(map[string]interface{})
//	err := mt.NewDecoder(mtData).Decode(&m)
//	if err != nil {
//		t.Fatal(err)
//	}
//	t.Logf("decode %v\n", m)
//}
