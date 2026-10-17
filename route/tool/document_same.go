package tool

import "strings"

// ============================================================
//  「내용이 그대로인가」 판정
//
//  편집을 저장할 때, 되돌릴 때, 요청을 반영할 때 모두 이 잣대를 쓴다.
//  세 곳이 다른 잣대를 쓰면 한 곳에서만 빈 편집이 새어 들어간다.
//
//  줄 끝 문자(\r)는 어디서 들어왔느냐에 따라 달라서 글의 내용이 아니다.
//  편집창은 \r\n 을 보내고, 서버는 저장할 때 \r 을 지운다.
//  하지만 옛날에 들여온 문서에는 \r 이 남아 있을 수 있어 견줄 때 맞춰 준다.
// ============================================================

// Normalize_document_data : 견주기 좋게 줄 끝 문자를 맞춘다
func Normalize_document_data(data string) string {
	return strings.ReplaceAll(data, "\r", "")
}

// Same_document_data : 두 본문이 사실상 같은가
func Same_document_data(left string, right string) bool {
	return Normalize_document_data(left) == Normalize_document_data(right)
}
