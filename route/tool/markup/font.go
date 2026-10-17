package markup

import (
	"regexp"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  글꼴 문법 도우미
//
//  `{{{#!font 글꼴이름 …}}}` 과 `[font(글꼴이름, 글)]` 이 같은 규칙을 쓴다.
//
//    {{{#!font 나눔고딕
//    본문
//    }}}
//
//    {{{#!font "Times New Roman", serif 본문}}}
//    {{{#!font 나눔고딕, 크기=120%, 색=#c00 본문}}}
//
//  쉼표로 나눈 조각 가운데 `=` 가 들어 있으면 옵션이고,
//  나머지는 글꼴 이름 목록(font-family)으로 이어 붙인다.
// ============================================================

var markup_font_size_regex = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?(?:px|%|em|rem|pt)?$`)
var markup_font_family_regex = regexp.MustCompile(`^[\p{L}\p{N} ._-]+$`)
var markup_font_weight_set = map[string]bool{
	"normal": true, "bold": true, "bolder": true, "lighter": true,
	"100": true, "200": true, "300": true, "400": true, "500": true,
	"600": true, "700": true, "800": true, "900": true,
}
var markup_font_style_set = map[string]bool{"normal": true, "italic": true, "oblique": true}

// markup_font_generic_family : 따옴표를 씌우면 안 되는 일반 글꼴 이름
var markup_font_generic_family = map[string]bool{
	"serif": true, "sans-serif": true, "monospace": true,
	"cursive": true, "fantasy": true, "system-ui": true,
	"ui-serif": true, "ui-sans-serif": true, "ui-monospace": true,
	"inherit": true, "initial": true, "unset": true,
}

// compat_split_font_spec : 글꼴 지정 부분과 본문을 나눈다
//
//	줄바꿈이 있으면 첫 줄이 글꼴 지정이다.
//	한 줄이면 따옴표로 묶인 덩어리 또는 첫 빈칸까지가 글꼴 지정이다.
func compat_split_font_spec(body string) (string, string) {
	body = strings.TrimLeft(body, " \t")
	if body == "" {
		return "", ""
	}

	if newline_index := strings.IndexByte(body, '\n'); newline_index >= 0 {
		return strings.TrimSpace(body[:newline_index]), body[newline_index+1:]
	}

	// 따옴표로 묶은 글꼴 이름은 빈칸이 들어가도 한 덩어리로 본다
	for _, quote := range []string{`"`, `'`, "&quot;", "&#x27;"} {
		if !strings.HasPrefix(body, quote) {
			continue
		}

		rest := body[len(quote):]
		end := strings.Index(rest, quote)
		if end < 0 {
			continue
		}

		spec := rest[:end]
		tail := rest[end+len(quote):]

		// 뒤에 `, serif` 처럼 글꼴이 더 붙어 있으면 같이 가져간다
		tail = strings.TrimLeft(tail, " \t")
		for strings.HasPrefix(tail, ",") {
			tail = strings.TrimLeft(tail[1:], " \t")
			cut := strings.IndexAny(tail, " \t")
			if cut < 0 {
				spec += ", " + tail

				return strings.TrimSpace(spec), ""
			}

			spec += ", " + tail[:cut]
			tail = strings.TrimLeft(tail[cut:], " \t")
		}

		return strings.TrimSpace(spec), tail
	}

	cut := strings.IndexAny(body, " \t")
	if cut < 0 {
		return strings.TrimSpace(body), ""
	}

	return strings.TrimSpace(body[:cut]), body[cut+1:]
}

// compat_font_family_value : 글꼴 이름 하나를 CSS 값으로
func compat_font_family_value(name string) string {
	name = strings.TrimSpace(tool.HTML_unescape(name))
	name = strings.Trim(name, `"'`)
	if name == "" {
		return ""
	}
	if markup_font_generic_family[strings.ToLower(name)] {
		return strings.ToLower(name)
	}
	if !markup_font_family_regex.MatchString(name) {
		return ""
	}
	if strings.ContainsAny(name, " ") {
		return "'" + name + "'"
	}

	return name
}

// compat_font_style : 글꼴 지정 부분을 CSS 선언으로 (못 알아들으면 빈 글)
func compat_font_style(spec string) string {
	spec = strings.TrimSpace(tool.HTML_unescape(spec))
	if spec == "" {
		return ""
	}

	family := []string{}
	extra := ""

	for _, piece := range strings.Split(spec, ",") {
		piece = strings.TrimSpace(piece)
		if piece == "" {
			continue
		}

		key, value, is_option := strings.Cut(piece, "=")
		if !is_option {
			if one := compat_font_family_value(piece); one != "" {
				family = append(family, one)
			}

			continue
		}

		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(strings.Trim(strings.TrimSpace(value), `"'`))
		if value == "" {
			continue
		}

		switch key {
		case "size", "크기", "글자크기":
			if markup_font_size_regex.MatchString(value) {
				if !strings.ContainsAny(value, "%pxemrt") {
					value += "%"
				}
				extra += "font-size:" + value + ";"
			}
		case "color", "색", "색상", "글자색":
			if color := compat_color_css_value(value); color != "" {
				extra += "color:" + color + ";"
			}
		case "bg", "background", "배경", "배경색":
			if color := compat_color_css_value(value); color != "" {
				extra += "background-color:" + color + ";"
			}
		case "weight", "굵기":
			if markup_font_weight_set[strings.ToLower(value)] {
				extra += "font-weight:" + strings.ToLower(value) + ";"
			}
		case "style", "모양", "기울임":
			if markup_font_style_set[strings.ToLower(value)] {
				extra += "font-style:" + strings.ToLower(value) + ";"
			}
		case "spacing", "자간":
			if markup_font_size_regex.MatchString(value) {
				if !strings.ContainsAny(value, "%pxemrt") {
					value += "px"
				}
				extra += "letter-spacing:" + value + ";"
			}
		case "family", "글꼴":
			if one := compat_font_family_value(value); one != "" {
				family = append(family, one)
			}
		}
	}

	style := ""
	if len(family) > 0 {
		style += "font-family:" + strings.Join(family, ", ") + ";"
	}

	return style + extra
}

// ============================================================
//  색 이름 검사
//
//  표의 `||<빨강>` 같은 자리에서 아무 낱말이나 색으로 받아들이면
//  `<tablesortable>` 같은 옵션이 배경색으로 새어 들어간다.
//  그래서 16진수이거나 CSS 가 아는 색 이름일 때만 색으로 본다.
// ============================================================

var markup_css_color_names = map[string]bool{}

func init() {
	names := []string{
		"aliceblue", "antiquewhite", "aqua", "aquamarine", "azure", "beige", "bisque", "black",
		"blanchedalmond", "blue", "blueviolet", "brown", "burlywood", "cadetblue", "chartreuse",
		"chocolate", "coral", "cornflowerblue", "cornsilk", "crimson", "cyan", "darkblue",
		"darkcyan", "darkgoldenrod", "darkgray", "darkgrey", "darkgreen", "darkkhaki",
		"darkmagenta", "darkolivegreen", "darkorange", "darkorchid", "darkred", "darksalmon",
		"darkseagreen", "darkslateblue", "darkslategray", "darkslategrey", "darkturquoise",
		"darkviolet", "deeppink", "deepskyblue", "dimgray", "dimgrey", "dodgerblue", "firebrick",
		"floralwhite", "forestgreen", "fuchsia", "gainsboro", "ghostwhite", "gold", "goldenrod",
		"gray", "grey", "green", "greenyellow", "honeydew", "hotpink", "indianred", "indigo",
		"ivory", "khaki", "lavender", "lavenderblush", "lawngreen", "lemonchiffon", "lightblue",
		"lightcoral", "lightcyan", "lightgoldenrodyellow", "lightgray", "lightgrey", "lightgreen",
		"lightpink", "lightsalmon", "lightseagreen", "lightskyblue", "lightslategray",
		"lightslategrey", "lightsteelblue", "lightyellow", "lime", "limegreen", "linen", "magenta",
		"maroon", "mediumaquamarine", "mediumblue", "mediumorchid", "mediumpurple",
		"mediumseagreen", "mediumslateblue", "mediumspringgreen", "mediumturquoise",
		"mediumvioletred", "midnightblue", "mintcream", "mistyrose", "moccasin", "navajowhite",
		"navy", "oldlace", "olive", "olivedrab", "orange", "orangered", "orchid", "palegoldenrod",
		"palegreen", "paleturquoise", "palevioletred", "papayawhip", "peachpuff", "peru", "pink",
		"plum", "powderblue", "purple", "rebeccapurple", "red", "rosybrown", "royalblue",
		"saddlebrown", "salmon", "sandybrown", "seagreen", "seashell", "sienna", "silver",
		"skyblue", "slateblue", "slategray", "slategrey", "snow", "springgreen", "steelblue",
		"tan", "teal", "thistle", "tomato", "turquoise", "violet", "wheat", "white", "whitesmoke",
		"yellow", "yellowgreen", "transparent", "currentcolor", "inherit",
	}
	for _, one := range names {
		markup_css_color_names[one] = true
	}
}

// compat_is_color_word : 색으로 볼 수 있는 낱말인지
func compat_is_color_word(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return false
	}
	if markup_css_color_names[value] {
		return true
	}

	value = strings.TrimPrefix(value, "#")
	switch len(value) {
	case 3, 4, 6, 8:
		return namumark_compat_hex_regex.MatchString(value)
	}

	return false
}

// compat_color_css_value : 색 값을 CSS 로 (색이 아니면 빈 글)
func compat_color_css_value(value string) string {
	value = strings.TrimSpace(tool.HTML_unescape(value))
	if !compat_is_color_word(value) {
		return ""
	}
	if markup_css_color_names[strings.ToLower(value)] {
		return strings.ToLower(value)
	}
	if strings.HasPrefix(value, "#") {
		return value
	}

	return "#" + value
}

// compat_is_color_list : `#fff` `#fff,#000` `red` 처럼 색만 적혀 있는지
//
//	표의 `||<#fff,#000>` 처럼 밝은 화면 · 어두운 화면 색을 함께 적을 수 있다.
func compat_is_color_list(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}

	parts := strings.Split(value, ",")
	if len(parts) > 2 {
		return false
	}

	for _, one := range parts {
		if !compat_is_color_word(one) {
			return false
		}
	}

	return true
}
