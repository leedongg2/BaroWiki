package route

import (
	"net/http"
	"path/filepath"
	"strings"

	"opennamu/route/tool"

	"github.com/gin-gonic/gin"
)

// ============================================================
//  2-4 프로필 사진  /baro/avatar
//
//  올린 사진은 오른쪽 위 그라바타 자리와 사용자 정보 틀의 얼굴 칸을 대신한다.
//  지우면 그라바타로 되돌아간다. 파일은 본인만 올리고 본인만 지운다.
// ============================================================

// baro_account_avatar_menu : 화면 아래 차림표
func baro_account_avatar_menu() [][]any {
	return [][]any{{"/baro", "바로위키"}, {"/baro/avatar", tool.Baro_t("프로필 사진")}}
}

// View_baro_account_avatar : 프로필 사진 화면 (/baro/avatar)
//
//	raw 가 nil 이 아니면 올리기, do 가 "delete" 면 지우기다.
func View_baro_account_avatar(config tool.Config, do string, raw []byte, file_name string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, stop := tool.Baro_login_wall(db, config, "/baro/avatar"); stop {
		return wall
	}

	me := config.IP
	alert := ""

	switch do {
	case "upload":
		switch {
		case tool.Baro_read_only(db):
			alert = tool.Baro_alert("warn", tool.Baro_t("지금은 읽기 전용입니다. 잠시 뒤에 다시 시도해 주세요."))
		case !tool.Baro_rate_ok(db, "avatar_upload", me, 20, 3600):
			alert = tool.Baro_alert("warn", tool.Baro_t("잠시 뒤에 다시 시도해 주세요."))
		case len(raw) == 0:
			alert = tool.Baro_alert("warn", tool.Baro_t("올릴 사진을 골라 주세요."))
		default:
			if err := tool.Baro_avatar_save(db, me, raw); err != nil {
				alert = tool.Baro_alert("danger", err.Error())
				break
			}

			tool.Baro_audit(db, config, "baro_avatar_upload", "user:"+me, "", filepath.Base(file_name), "프로필 사진 올리기")
			alert = tool.Baro_alert("ok", tool.Baro_t("프로필 사진을 올렸습니다."))
		}

	case "delete":
		if tool.Baro_avatar_has(db, me) {
			tool.Baro_avatar_delete(db, me)
			tool.Baro_audit(db, config, "baro_avatar_delete", "user:"+me, "on", "", "프로필 사진 지우기")
			alert = tool.Baro_alert("ok", tool.Baro_t("프로필 사진을 지웠습니다. 다시 그라바타가 보입니다."))
		} else {
			alert = tool.Baro_alert("warn", tool.Baro_t("지울 프로필 사진이 없습니다."))
		}
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"", tool.Baro_t("프로필 사진")}})
	body += tool.Baro_hero(
		tool.Baro_t("프로필 사진"),
		tool.Baro_t("오른쪽 위와 사용자 정보 틀에 보이는 얼굴 그림입니다."),
		"",
	)
	body += alert

	// --- 지금 쓰고 있는 그림 ---
	now_html := ""
	if url := tool.Baro_avatar_url(db, me); url != "" {
		now_html = `<p><img class="namu_gravatar bw_avatar_img" width="96" height="96" alt="" src="` +
			tool.HTML_escape(url) + `"></p>`
		now_html += tool.Baro_note(tool.Baro_t("올린 사진이 그라바타 자리를 대신하고 있습니다."))
		now_html += tool.Baro_form("/baro/avatar/delete",
			tool.Baro_submit(tool.Baro_t("프로필 사진 지우기"), "danger"))
	} else {
		gravatar := tool.Get_user_gravatar_ui(db, me, 96)
		if gravatar != "" {
			now_html = `<p>` + gravatar + `</p>`
		}
		now_html += tool.Baro_note(tool.Baro_t("아직 올린 사진이 없어 그라바타가 보입니다."))
	}

	body += tool.Baro_card(tool.Baro_t("지금 쓰고 있는 그림"), now_html)

	// --- 올리기 ---
	upload := tool.Baro_field_row_html(
		tool.Baro_t("사진 고르기"),
		`<input class="bw_in" type="file" name="avatar" accept="image/png,image/jpeg,image/gif,image/webp">`,
		tool.Baro_t("png · jpg · gif · webp 만 올릴 수 있습니다. 2MB 이하."),
	)
	upload += tool.Baro_submit(tool.Baro_t("올리기"), "primary")

	body += tool.Baro_card(tool.Baro_t("사진 올리기"), tool.Baro_form_file("/baro/avatar", upload))

	body += tool.Baro_card(tool.Baro_t("알아 두기"), tool.Baro_list([]string{
		tool.Baro_t_esc("사진은 위키 문서로 올라가지 않습니다. 문서 역사에도 남지 않고, 지우면 파일까지 함께 사라집니다."),
		tool.Baro_t_esc("다른 사람이 올린 사진이나 얼굴을 함부로 쓰지 말아 주세요."),
		tool.Baro_t_esc("새 사진을 올리면 앞서 올린 사진은 지워지고 새것으로 바뀝니다."),
		tool.Baro_t_esc("사용자 정보 틀에서는 사진 아래 「그라바타 펼치기」 로 본래 그라바타도 볼 수 있습니다."),
	}))

	return tool.Baro_page(db, config, tool.Baro_t("프로필 사진"), body, baro_account_avatar_menu())
}

// View_baro_account_avatar_file : 올린 프로필 사진 내려주기 (/baro/avatar/<사용자>)
//
//	위키를 볼 수 있는 사람에게만 내려준다.
func View_baro_account_avatar_file(c *gin.Context) {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	c.Header("X-Content-Type-Options", "nosniff")

	config := tool.Config{
		IP:      tool.Get_IP(c),
		Cookies: tool.Get_Cookies(c),
		Session: tool.Get_session(c),
	}

	if !tool.Check_acl(db, "", "", "render", config.IP) {
		c.String(http.StatusForbidden, "")
		return
	}

	user := strings.TrimPrefix(c.Param("user"), "/")
	if user == "" || !tool.Baro_is_user(user) {
		c.String(http.StatusNotFound, "")
		return
	}

	path := tool.Baro_avatar_path(db, user)
	if path == "" {
		c.String(http.StatusNotFound, "")
		return
	}

	mime_type := "application/octet-stream"
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(path), ".")) {
	case "png":
		mime_type = "image/png"
	case "jpg", "jpeg":
		mime_type = "image/jpeg"
	case "gif":
		mime_type = "image/gif"
	case "webp":
		mime_type = "image/webp"
	}

	c.Header("Content-Type", mime_type)
	c.Header("Cache-Control", "private, max-age=600")
	c.File(path)
}
