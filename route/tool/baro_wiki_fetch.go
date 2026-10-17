package tool

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ============================================================
//  다른 위키에서 글 받아 오기
//
//  주소를 관리자만 등록할 수 있게 해도, 그 주소가 사내망이나
//  클라우드 메타데이터 주소를 가리키면 위험하다.
//  그래서 받아 오기 직전에 실제로 이어지는 아이피를 확인해
//  사설·특수 대역이면 끊는다.
// ============================================================

// Baro_fetch_max_size : 받아 올 수 있는 최대 크기
const Baro_fetch_max_size = 2 << 20 // 2MB

// Baro_fetch_timeout : 받아 오기 제한 시간
const Baro_fetch_timeout = 12 * time.Second

// baro_fetch_blocked_ip : 받아 오면 안 되는 대역인지
func baro_fetch_blocked_ip(address net.IP) bool {
	if address == nil {
		return true
	}
	if address.IsLoopback() || address.IsPrivate() || address.IsUnspecified() ||
		address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsMulticast() {
		return true
	}

	// 클라우드 메타데이터 주소
	if address.String() == "169.254.169.254" {
		return true
	}

	// 공유기 · 통신사 내부 대역 (100.64.0.0/10)
	if four := address.To4(); four != nil && four[0] == 100 && four[1] >= 64 && four[1] <= 127 {
		return true
	}

	return false
}

// baro_fetch_dial : 이어지는 아이피를 확인하고 막힌 대역이면 끊는다
func baro_fetch_dial(ctx context.Context, network string, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	ip_list, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}

	for _, one := range ip_list {
		if baro_fetch_blocked_ip(one) {
			return nil, errors.New("내부망 주소로는 이어지지 않습니다")
		}
	}

	dialer := &net.Dialer{Timeout: Baro_fetch_timeout}

	return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
}

// baro_fetch_client : 받아 오기 전용 http 손님
var baro_fetch_client = &http.Client{
	Timeout: Baro_fetch_timeout,
	Transport: &http.Transport{
		DialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			return baro_fetch_dial(ctx, network, address)
		},
	},
	CheckRedirect: func(request *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errors.New("주소가 너무 여러 번 넘어갑니다")
		}

		return nil
	},
}

// Baro_fetch_url_check : 받아 올 수 있는 주소인지 (문제가 있으면 까닭을 돌려준다)
func Baro_fetch_url_check(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "주소가 비어 있습니다."
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "주소를 읽을 수 없습니다."
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", "http 또는 https 주소만 쓸 수 있습니다."
	}
	if parsed.Hostname() == "" {
		return "", "주소에 사이트 이름이 없습니다."
	}
	if address := net.ParseIP(parsed.Hostname()); address != nil && baro_fetch_blocked_ip(address) {
		return "", "내부망 주소는 쓸 수 없습니다."
	}

	return parsed.String(), ""
}

// Baro_fetch_text : 주소에서 글을 받아 온다 (실패하면 까닭을 돌려준다)
func Baro_fetch_text(raw string) (string, string) {
	target, check_error := Baro_fetch_url_check(raw)
	if check_error != "" {
		return "", check_error
	}

	request, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return "", "요청을 만들지 못했습니다."
	}
	request.Header.Set("User-Agent", "openNAMU-baro-wiki-link/1.0")
	request.Header.Set("Accept", "text/plain, text/html;q=0.8, */*;q=0.5")

	response, err := baro_fetch_client.Do(request)
	if err != nil {
		return "", "받아 오지 못했습니다 : " + err.Error()
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", "상대 위키가 " + response.Status + " 를 돌려줬습니다."
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, Baro_fetch_max_size))
	if err != nil {
		return "", "내용을 읽지 못했습니다."
	}

	return string(body), ""
}
