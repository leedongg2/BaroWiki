package route

import (
	"testing"

	"opennamu/route/tool"
)

// TestBaroProjectDocName : 프로젝트 문서 이름이 왕복하는가
//
//	저장하는 이름과 화면에 보이는 이름이 다르므로,
//	보이는 이름으로 들어온 주소가 다시 저장한 이름으로 돌아와야 한다.
//	그러지 않으면 링크를 눌렀을 때 다른(없는) 문서로 간다.
func TestBaroProjectDocName(t *testing.T) {
	cases := []string{
		"칶보디아 정리",
		"한국사",
		"Barowiki",
		"띄어 쓴 이름",
	}

	for _, name := range cases {
		stored := baro_project_doc_name(name)

		if stored != "baro:프로젝트:"+name {
			t.Fatalf("%q : 저장 이름이 다르다 : %q", name, stored)
		}

		// 화면에는 바로위키:프로젝트:<이름> 으로 보인다
		shown := tool.Namespace_display(stored)
		if shown != "바로위키:프로젝트:"+name {
			t.Fatalf("%q : 보이는 이름이 다르다 : %q", name, shown)
		}

		// 보이는 이름으로 들어와도 저장한 이름으로 돌아온다
		if back := tool.Namespace_normalize(shown); back != stored {
			t.Fatalf("%q : 왕복이 깨졌다 : %q → %q (바란 값 %q)", name, shown, back, stored)
		}

		// 주소에도 한국어로 나간다
		if url := tool.Doc_url(stored); url == "" {
			t.Fatalf("%q : 주소가 비었다", name)
		}
	}

	// 이름공간은 바로위키다 (프로젝트 이름공간이 아니다)
	namespace := tool.Namespace_of(baro_project_doc_name("아무개"))
	if namespace.Prefix != "baro:" {
		t.Fatalf("바로위키 이름공간이 아니다 : %q", namespace.Prefix)
	}
}
