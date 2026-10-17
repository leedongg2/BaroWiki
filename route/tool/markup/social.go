package markup

import (
	"net/url"
	"regexp"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  바깥 서비스 문법
//
//  1) 글 넣기 (embed)
//       [youtube(코드)]              영상
//       [instagram(글코드)]          글
//       [naverblog(블로그아이디, 글번호)]
//       [naverblog(https://blog.naver.com/아이디/글번호)]
//       [tistory(블로그이름, 글번호)]
//       [tistory(https://블로그이름.tistory.com/글번호)]
//
//     네이버 블로그와 티스토리는 바깥에서 iframe 을 막는 경우가 많아
//     기본은 「바로 가기 상자」이고, `embed=1` 을 적으면 iframe 으로 넣는다.
//
//  2) 계정 달기 (account)
//       [계정(유튜브, @handle)]  [account(youtube, @handle)]
//       [계정(인스타그램, 아이디)]
//       [계정(네이버블로그, 아이디)]
//       [계정(티스토리, 블로그이름)]
//
//     사용자 문서·틀에서 「이 사람의 계정」 을 한 줄로 적을 때 쓴다.
// ============================================================

var markup_social_id_regex = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)
var markup_social_number_regex = regexp.MustCompile(`^[0-9]{1,20}$`)

// markup_social_account_kind : 계정 종류 (열쇠말 → 표시 이름 · 주소 틀 · 아이디 앞머리)
type markup_social_account_kind struct {
	key    string
	name   string
	url    string
	prefix string
	icon   string
}

// markup_social_account_table : 계정 종류 표
//
//	`url` 안의 {} 자리에 아이디가 들어간다.
func markup_social_account_table() map[string]markup_social_account_kind {
	youtube := markup_social_account_kind{"youtube", "유튜브", "https://www.youtube.com/@{}", "@", "▶"}
	instagram := markup_social_account_kind{"instagram", "인스타그램", "https://www.instagram.com/{}/", "@", "◉"}
	naverblog := markup_social_account_kind{"naverblog", "네이버 블로그", "https://blog.naver.com/{}", "", "N"}
	tistory := markup_social_account_kind{"tistory", "티스토리", "https://{}.tistory.com/", "", "T"}
	twitter := markup_social_account_kind{"twitter", "X", "https://x.com/{}", "@", "X"}
	facebook := markup_social_account_kind{"facebook", "페이스북", "https://www.facebook.com/{}", "", "f"}
	github := markup_social_account_kind{"github", "GitHub", "https://github.com/{}", "@", "G"}
	chzzk := markup_social_account_kind{"chzzk", "치지직", "https://chzzk.naver.com/{}", "", "C"}
	soop := markup_social_account_kind{"soop", "SOOP", "https://ch.sooplive.co.kr/{}", "", "S"}
	tiktok := markup_social_account_kind{"tiktok", "틱톡", "https://www.tiktok.com/@{}", "@", "♪"}

	table := map[string]markup_social_account_kind{
		"youtube": youtube, "유튜브": youtube, "yt": youtube,
		"instagram": instagram, "인스타그램": instagram, "인스타": instagram, "ig": instagram,
		"naverblog": naverblog, "네이버블로그": naverblog, "네이버 블로그": naverblog, "blog": naverblog,
		"tistory": tistory, "티스토리": tistory,
		"twitter": twitter, "트위터": twitter, "x": twitter,
		"facebook": facebook, "페이스북": facebook, "fb": facebook,
		"github": github, "깃허브": github,
		"chzzk": chzzk, "치지직": chzzk,
		"soop": soop, "숲": soop, "아프리카": soop,
		"tiktok": tiktok, "틱톡": tiktok,
	}

	return table
}

// markup_social_account_id : 아이디에서 주소·앞머리를 떼어 낸다
func markup_social_account_id(raw string, kind markup_social_account_kind) string {
	value := strings.TrimSpace(tool.HTML_unescape(raw))
	value = strings.TrimPrefix(value, "@")

	// 주소를 그대로 붙여 넣은 경우 마지막 칸만 쓴다
	if strings.Contains(value, "/") {
		if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
			value = strings.Trim(parsed.Path, "/")
			if strings.Contains(value, "/") {
				value = value[strings.LastIndex(value, "/")+1:]
			}
			if value == "" {
				value = strings.SplitN(parsed.Host, ".", 2)[0]
			}
		} else {
			value = value[strings.LastIndex(value, "/")+1:]
		}
		value = strings.TrimPrefix(value, "@")
	}

	if !markup_social_id_regex.MatchString(value) {
		return ""
	}

	return value
}

// compat_social_account : 계정 상자 하나
func (class *namumark_compat_renderer) compat_social_account(data string) string {
	args := compat_split_macro_args(data)
	if len(args) < 2 {
		return ""
	}

	key := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tool.HTML_unescape(args[0])), " ", ""))
	kind, ok := markup_social_account_table()[key]
	if !ok {
		return ""
	}

	account_id := markup_social_account_id(args[1], kind)
	if account_id == "" {
		return ""
	}

	label := kind.prefix + account_id
	for _, item := range args[2:] {
		name, value, has := strings.Cut(item, "=")
		if !has {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(name), "label") || strings.TrimSpace(name) == "이름" {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				label = trimmed
			}
		}
	}

	if class.collect_only {
		return label
	}

	// 유튜브는 채널 아이디(UC…)일 때 주소 모양이 다르다
	kind_url := kind.url
	if kind.key == "youtube" && len(account_id) == 24 && strings.HasPrefix(account_id, "UC") {
		kind_url = "https://www.youtube.com/channel/{}"
	}
	link := strings.ReplaceAll(kind_url, "{}", url.PathEscape(account_id))

	return class.reserve(
		`<a class="opennamu_account opennamu_account_` + compat_html_escape(kind.key) + `" target="_blank" rel="nofollow noopener"` +
			` title="` + compat_escape_value(kind.name) + `" href="` + compat_html_escape(link) + `">` +
			`<span class="opennamu_account_icon" aria-hidden="true">` + compat_escape_value(kind.icon) + `</span>` +
			`<span class="opennamu_account_name">` + compat_escape_value(kind.name) + `</span>` +
			`<span class="opennamu_account_id">` + compat_escape_value(label) + `</span>` +
			`</a>`,
	)
}

// compat_social_link_card : 바로 가기 상자 (iframe 을 막는 서비스용)
func (class *namumark_compat_renderer) compat_social_link_card(service string, title string, link string) string {
	return class.reserve(
		`<a class="opennamu_link_card opennamu_link_card_` + compat_html_escape(service) + `" target="_blank" rel="nofollow noopener" href="` + compat_html_escape(link) + `">` +
			`<span class="opennamu_link_card_service">` + compat_escape_value(title) + `</span>` +
			`<span class="opennamu_link_card_url">` + compat_escape_value(link) + `</span>` +
			`</a>`,
	)
}

// compat_naverblog_macro : 네이버 블로그 글
func (class *namumark_compat_renderer) compat_naverblog_macro(data string) string {
	blog_id := ""
	log_no := ""
	width := "100%"
	height := "600px"
	embed := false

	for _, item := range compat_split_macro_args(data) {
		item = strings.TrimSpace(tool.HTML_unescape(item))
		if item == "" {
			continue
		}

		name, value, has := strings.Cut(item, "=")
		if has {
			switch strings.ToLower(strings.TrimSpace(name)) {
			case "width":
				width = compat_middle_style(compat_file_px(value))

				continue
			case "height":
				height = compat_middle_style(compat_file_px(value))

				continue
			case "embed":
				embed = strings.TrimSpace(value) == "1"

				continue
			case "blogid", "id":
				blog_id = strings.TrimSpace(value)

				continue
			case "logno", "no":
				log_no = strings.TrimSpace(value)

				continue
			}
		}

		if strings.Contains(item, "blog.naver.com") || strings.Contains(item, "m.blog.naver.com") {
			parsed_id, parsed_no := compat_parse_naverblog_url(item)
			if parsed_id != "" {
				blog_id = parsed_id
			}
			if parsed_no != "" {
				log_no = parsed_no
			}

			continue
		}

		if blog_id == "" {
			blog_id = item
		} else if log_no == "" {
			log_no = item
		}
	}

	if !markup_social_id_regex.MatchString(blog_id) {
		return ""
	}
	if log_no != "" && !markup_social_number_regex.MatchString(log_no) {
		return ""
	}

	link := "https://blog.naver.com/" + url.PathEscape(blog_id)
	if log_no != "" {
		link += "/" + log_no
	}

	if class.collect_only {
		return ""
	}

	if !embed || log_no == "" {
		return class.compat_social_link_card("naverblog", "네이버 블로그 - "+blog_id, link)
	}

	src := "https://blog.naver.com/PostView.naver?blogId=" + url.QueryEscape(blog_id) +
		"&logNo=" + url.QueryEscape(log_no) + "&redirect=Dlog&widgetTypeCall=true&directAccess=false"

	return class.reserve(compat_media_iframe(src, width, height, "네이버 블로그"))
}

// compat_parse_naverblog_url : 네이버 블로그 주소에서 아이디와 글번호를 뽑는다
func compat_parse_naverblog_url(raw string) (string, string) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", ""
	}

	query := parsed.Query()
	blog_id := query.Get("blogId")
	log_no := query.Get("logNo")

	path_parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	clean := []string{}
	for _, one := range path_parts {
		if one != "" && !strings.HasSuffix(strings.ToLower(one), ".naver") && !strings.HasSuffix(strings.ToLower(one), ".nhn") {
			clean = append(clean, one)
		}
	}

	if blog_id == "" && len(clean) > 0 {
		blog_id = clean[0]
	}
	if log_no == "" && len(clean) > 1 {
		log_no = clean[1]
	}

	return blog_id, log_no
}

// compat_tistory_macro : 티스토리 글
func (class *namumark_compat_renderer) compat_tistory_macro(data string) string {
	blog_name := ""
	post_no := ""
	width := "100%"
	height := "600px"
	embed := false

	for _, item := range compat_split_macro_args(data) {
		item = strings.TrimSpace(tool.HTML_unescape(item))
		if item == "" {
			continue
		}

		name, value, has := strings.Cut(item, "=")
		if has {
			switch strings.ToLower(strings.TrimSpace(name)) {
			case "width":
				width = compat_middle_style(compat_file_px(value))

				continue
			case "height":
				height = compat_middle_style(compat_file_px(value))

				continue
			case "embed":
				embed = strings.TrimSpace(value) == "1"

				continue
			case "blog", "name":
				blog_name = strings.TrimSpace(value)

				continue
			case "no", "post":
				post_no = strings.TrimSpace(value)

				continue
			}
		}

		if strings.Contains(item, "tistory.com") {
			parsed_name, parsed_no := compat_parse_tistory_url(item)
			if parsed_name != "" {
				blog_name = parsed_name
			}
			if parsed_no != "" {
				post_no = parsed_no
			}

			continue
		}

		if blog_name == "" {
			blog_name = item
		} else if post_no == "" {
			post_no = item
		}
	}

	if !markup_social_id_regex.MatchString(blog_name) {
		return ""
	}
	if post_no != "" && !markup_social_number_regex.MatchString(post_no) {
		return ""
	}

	link := "https://" + blog_name + ".tistory.com/"
	if post_no != "" {
		link += post_no
	}

	if class.collect_only {
		return ""
	}

	if !embed {
		title := "티스토리 - " + blog_name
		if post_no != "" {
			title += " #" + post_no
		}

		return class.compat_social_link_card("tistory", title, link)
	}

	return class.reserve(compat_media_iframe(link, width, height, "티스토리"))
}

// compat_parse_tistory_url : 티스토리 주소에서 블로그 이름과 글번호를 뽑는다
func compat_parse_tistory_url(raw string) (string, string) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", ""
	}

	host := strings.ToLower(parsed.Hostname())
	blog_name := ""
	if strings.HasSuffix(host, ".tistory.com") {
		blog_name = strings.TrimSuffix(host, ".tistory.com")
	}

	post_no := ""
	for _, one := range strings.Split(strings.Trim(parsed.Path, "/"), "/") {
		if markup_social_number_regex.MatchString(one) {
			post_no = one
		}
	}

	return blog_name, post_no
}

// compat_chzzk_macro : 치지직 영상
func (class *namumark_compat_renderer) compat_chzzk_macro(data string) string {
	code := ""
	width := "640px"
	height := "360px"

	for _, item := range compat_split_macro_args(data) {
		item = strings.TrimSpace(tool.HTML_unescape(item))
		name, value, has := strings.Cut(item, "=")
		if has {
			switch strings.ToLower(strings.TrimSpace(name)) {
			case "width":
				width = compat_middle_style(compat_file_px(value))
			case "height":
				height = compat_middle_style(compat_file_px(value))
			}

			continue
		}
		if code == "" {
			code = item
		}
	}

	if strings.Contains(code, "chzzk.naver.com") {
		code = code[strings.LastIndex(code, "/")+1:]
	}
	if !namumark_compat_media_code_regex.MatchString(code) || width == "" || height == "" {
		return ""
	}
	if class.collect_only {
		return ""
	}

	return class.reserve(compat_media_iframe("https://chzzk.naver.com/embed/video/"+code, width, height, "치지직"))
}

// ============================================================
//  바깥 쪽에서 쓰는 계정 도우미
//
//  사용자 설정에 저장해 둔 계정을 본문 문법과 똑같은 모양으로 그린다.
//  본문 `[계정(…)]` 과 화면이 갈라지지 않게 같은 함수를 쓴다.
// ============================================================

// Markup_account_kind_list : 쓸 수 있는 계정 종류 열쇠말
func Markup_account_kind_list() []string {
	seen := map[string]bool{}
	out := []string{}
	for _, kind := range markup_social_account_table() {
		if seen[kind.key] {
			continue
		}

		seen[kind.key] = true
		out = append(out, kind.key)
	}

	return out
}

// Markup_account_id : 적어 넣은 값에서 아이디만 뽑는다 (아이디가 아니면 빈 글)
func Markup_account_id(kind_key string, raw string) string {
	kind, ok := markup_social_account_table()[strings.ToLower(strings.TrimSpace(kind_key))]
	if !ok {
		return ""
	}

	return markup_social_account_id(raw, kind)
}

// Markup_account_html : 계정 상자 하나 (본문 문법과 같은 모양)
func Markup_account_html(kind_key string, raw string) string {
	kind, ok := markup_social_account_table()[strings.ToLower(strings.TrimSpace(kind_key))]
	if !ok {
		return ""
	}

	account_id := markup_social_account_id(raw, kind)
	if account_id == "" {
		return ""
	}

	kind_url := kind.url
	if kind.key == "youtube" && len(account_id) == 24 && strings.HasPrefix(account_id, "UC") {
		kind_url = "https://www.youtube.com/channel/{}"
	}
	link := strings.ReplaceAll(kind_url, "{}", url.PathEscape(account_id))

	return `<a class="opennamu_account opennamu_account_` + compat_html_escape(kind.key) + `" target="_blank" rel="nofollow noopener"` +
		` title="` + compat_escape_value(kind.name) + `" href="` + compat_html_escape(link) + `">` +
		`<span class="opennamu_account_icon" aria-hidden="true">` + compat_escape_value(kind.icon) + `</span>` +
		`<span class="opennamu_account_name">` + compat_escape_value(kind.name) + `</span>` +
		`<span class="opennamu_account_id">` + compat_escape_value(kind.prefix+account_id) + `</span>` +
		`</a>`
}
