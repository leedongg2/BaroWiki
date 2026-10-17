package tool

import (
	"database/sql"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ============================================================
//  GeoIP — 아이피로 나라 알아내기
//
//  나라를 알아내는 길은 두 가지다.
//
//   1. 앞단(CDN·리버스 프록시)이 붙여 주는 머리글
//      Cloudflare 의 `CF-IPCountry` 처럼 앞단이 이미 판정해 붙여 주는 값이다.
//      가장 정확하고 빠르지만, 위키가 앞단 없이 바로 열려 있으면
//      이용자가 머리글을 마음대로 지어 보낼 수 있다.
//      그래서 **설정에서 머리글 이름을 적어 둘 때만** 쓴다. (기본은 꺼짐)
//
//   2. 위키에 넣어 둔 대역표 (baro_geoip)
//      운영자가 「이 대역은 어느 나라」 를 직접 넣어 둔다.
//      앞단이 없을 때 쓰는 길이며, 넣어 둔 만큼만 맞는다.
//
//  둘 다 없으면 나라를 모르는 것으로 보고, geoip 규칙은 걸리지 않는다.
//  (모른다고 해서 허용도 거부도 하지 않는다)
// ============================================================

// Acl_geoip_header_setting : 앞단이 붙여 주는 나라 머리글 이름을 담는 설정 열쇠
const Acl_geoip_header_setting = "geoip_header"

// acl_geoip_cache_max : 들고 있을 아이피 수
const acl_geoip_cache_max = 4096

// acl_geoip_cache_ttl : 들고 있는 시간
const acl_geoip_cache_ttl = 10 * time.Minute

type acl_geoip_entry struct {
	country string
	until   time.Time
}

var (
	acl_geoip_lock  sync.Mutex
	acl_geoip_cache = map[string]acl_geoip_entry{}
)

// Acl_geoip_remember : 앞단이 알려 준 나라를 이 아이피에 적어 둔다
//
//	요청을 받을 때 한 번 불러 두면, 그 요청 안에서 ACL 을 여러 번 따져도 다시 읽지 않는다.
func Acl_geoip_remember(ip string, country string) {
	country = strings.ToUpper(strings.TrimSpace(country))
	if ip == "" || len(country) != 2 {
		return
	}

	acl_geoip_lock.Lock()
	defer acl_geoip_lock.Unlock()

	// 너무 불어나면 통째로 비운다 (다시 채워진다)
	if len(acl_geoip_cache) >= acl_geoip_cache_max {
		acl_geoip_cache = map[string]acl_geoip_entry{}
	}

	acl_geoip_cache[ip] = acl_geoip_entry{country: country, until: time.Now().Add(acl_geoip_cache_ttl)}
}

// acl_geoip_recall : 적어 둔 나라 꺼내기
func acl_geoip_recall(ip string) (string, bool) {
	acl_geoip_lock.Lock()
	defer acl_geoip_lock.Unlock()

	entry, ok := acl_geoip_cache[ip]
	if !ok {
		return "", false
	}
	if time.Now().After(entry.until) {
		delete(acl_geoip_cache, ip)

		return "", false
	}

	return entry.country, true
}

// Acl_geoip_country : 아이피의 나라 (모르면 "", false)
func Acl_geoip_country(db *sql.DB, ip string) (string, bool) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return "", false
	}

	if country, ok := acl_geoip_recall(ip); ok {
		return country, true
	}

	// 위키에 넣어 둔 대역표에서 찾는다
	number, ok := Acl_geoip_ipv4_number(ip)
	if !ok {
		return "", false
	}

	country := ""
	found := QueryRow_DB(
		db,
		"select country from baro_geoip where start_num + 0 <= ? and end_num + 0 >= ? order by start_num + 0 desc limit 1",
		[]any{&country},
		number, number,
	)
	if !found || strings.TrimSpace(country) == "" {
		return "", false
	}

	country = strings.ToUpper(strings.TrimSpace(country))
	Acl_geoip_remember(ip, country)

	return country, true
}

// Acl_geoip_ipv4_number : IPv4 를 숫자로 (IPv6 는 다루지 않는다)
func Acl_geoip_ipv4_number(ip string) (uint32, bool) {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return 0, false
	}

	four := parsed.To4()
	if four == nil {
		return 0, false
	}

	return uint32(four[0])<<24 | uint32(four[1])<<16 | uint32(four[2])<<8 | uint32(four[3]), true
}

// Acl_geoip_range_add : 대역표에 한 줄 넣기
//
//	`1.2.3.0/24` 또는 `1.2.3.0-1.2.3.255` 모양을 받는다.
func Acl_geoip_range_add(db *sql.DB, area string, country string, note string, actor string) bool {
	start, end, ok := Acl_geoip_range_parse(area)
	if !ok {
		return false
	}

	country = strings.ToUpper(strings.TrimSpace(country))
	if len(country) != 2 {
		return false
	}

	Exec_DB(
		db,
		"insert into baro_geoip (id, start_num, end_num, country, area, note, actor, date) values (?, ?, ?, ?, ?, ?, ?, ?)",
		Baro_next_id(db, "baro_geoip"),
		strconv.FormatUint(uint64(start), 10),
		strconv.FormatUint(uint64(end), 10),
		country, strings.TrimSpace(area), Baro_trunc(note, 300), actor, Get_time(),
	)

	// 대역이 바뀌었으니 들고 있던 것은 버린다
	Acl_geoip_cache_clear()

	return true
}

// Acl_geoip_range_parse : 적어 낸 대역을 숫자 두 개로
func Acl_geoip_range_parse(area string) (uint32, uint32, bool) {
	area = strings.TrimSpace(area)
	if area == "" {
		return 0, 0, false
	}

	if strings.Contains(area, "/") {
		_, network, err := net.ParseCIDR(area)
		if err != nil || network.IP.To4() == nil {
			return 0, 0, false
		}

		start, ok := Acl_geoip_ipv4_number(network.IP.String())
		if !ok {
			return 0, 0, false
		}

		ones, bits := network.Mask.Size()
		if bits != 32 {
			return 0, 0, false
		}

		size := uint32(1)<<uint(32-ones) - 1

		return start, start + size, true
	}

	if first, second, ok := strings.Cut(area, "-"); ok {
		start, ok1 := Acl_geoip_ipv4_number(first)
		end, ok2 := Acl_geoip_ipv4_number(second)
		if !ok1 || !ok2 || start > end {
			return 0, 0, false
		}

		return start, end, true
	}

	// 아이피 하나만 적은 경우
	one, ok := Acl_geoip_ipv4_number(area)
	if !ok {
		return 0, 0, false
	}

	return one, one, true
}

// Acl_geoip_range_delete : 대역표에서 한 줄 빼기
func Acl_geoip_range_delete(db *sql.DB, id string) {
	Exec_DB(db, "delete from baro_geoip where id = ?", id)
	Acl_geoip_cache_clear()
}

// Acl_geoip_cache_clear : 들고 있던 나라 정보를 모두 버린다
func Acl_geoip_cache_clear() {
	acl_geoip_lock.Lock()
	defer acl_geoip_lock.Unlock()

	acl_geoip_cache = map[string]acl_geoip_entry{}
}
