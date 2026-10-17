package tool

import "testing"

// TestSameDocumentData : 「내용이 그대로인가」 잣대
//
//	줄 끝 문자만 다른 두 글은 같은 글로 봐야 한다.
//	그러지 않으면 옛날에 들여온 문서에서 빈 편집이 계속 저장된다.
func TestSameDocumentData(t *testing.T) {
	same := [][2]string{
		{"가나다", "가나다"},
		{"가\r\n나", "가\n나"},
		{"가\n나", "가\r\n나"},
		{"가\r\n나\r\n다", "가\n나\n다"},
		{"", ""},
		{"\r\n", "\n"},
	}
	for _, one := range same {
		if !Same_document_data(one[0], one[1]) {
			t.Fatalf("%q 와 %q 는 같은 글로 봐야 한다", one[0], one[1])
		}
	}

	different := [][2]string{
		{"가나다", "가나라"},
		{"가나다", "가나다 "},
		{"가\n나", "가\n\n나"},
		{"", " "},
		{"가나다", ""},
	}
	for _, one := range different {
		if Same_document_data(one[0], one[1]) {
			t.Fatalf("%q 와 %q 는 다른 글로 봐야 한다", one[0], one[1])
		}
	}
}
