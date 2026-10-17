package tool

import "testing"

func TestBaroFilterExpr(t *testing.T) {
	env := Baro_filter_env{
		Content:   "여기에 카지노 광고가 있고 https://a.example https://b.example 링크도 있다",
		Old:       "짧은 예전 본문",
		Title:     "테스트 문서",
		Summary:   "광고 추가",
		User:      "1.2.3.4",
		Namespace: "문서",
		Recent:    3,
	}

	cases := []struct {
		source string
		want   bool
	}{
		{`content contains "카지노"`, true},
		{`content contains "없는말"`, false},
		{`content contains "카지노" and namespace == "문서"`, true},
		{`content contains "카지노" and namespace == "틀"`, false},
		{`content contains "카지노" or namespace == "틀"`, true},
		{`not content contains "없는말"`, true},
		{`title matches "^테스트"`, true},
		{`title !matches "^없음"`, true},
		{`user_is_ip`, true},
		{`user_is_member`, false},
		{`not user_is_member`, true},
		{`links >= 2`, true},
		{`links > 5`, false},
		{`added > 5`, true},
		{`recent >= 3 and user_is_ip`, true},
		{`namespace in "문서, 틀"`, true},
		{`namespace !in "틀, 분류"`, true},
		{`(content contains "카지노" or content contains "토토") and user_is_ip`, true},
		{`is_new`, false},
		{`is_blank`, false},
		{`summary contains "광고" and added > 5 and not user_is_member`, true},
	}

	for _, one := range cases {
		got, detail, fail := Baro_filter_expr_run(one.source, env)
		if fail != "" {
			t.Fatalf("%q : 문법 오류 %s", one.source, fail)
		}
		if got != one.want {
			t.Fatalf("%q : %v 를 바랐는데 %v (까닭 %s)", one.source, one.want, got, detail)
		}
	}

	broken := []string{
		`content contains`,
		`(content contains "a"`,
		`nosuchfield == "1"`,
		`content contains "a" and`,
	}
	for _, one := range broken {
		if _, _, fail := Baro_filter_expr_run(one, env); fail == "" {
			t.Fatalf("%q : 문법 오류를 잡지 못했다", one)
		}
	}
}
